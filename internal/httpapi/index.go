package httpapi

import (
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"faangjobs/internal/model"
	"faangjobs/internal/store"
)

// Index is a hot-reloadable, in-memory, searchable view of the crawled data.
// Reads are lock-free via an atomic snapshot pointer; a background reloader
// rebuilds the snapshot when the data folder changes.
type Index struct {
	store *store.Store
	log   func(format string, args ...any)

	snap atomic.Pointer[snapshot]

	// lastMTime guards rebuilds so we only reload when data actually changes.
	reloadMu  sync.Mutex
	lastMTime time.Time

	// descCache caches recently-read full company files for the detail endpoint.
	descMu    sync.Mutex
	descCache map[string]cachedCompany
	descGen   int64
}

type cachedCompany struct {
	gen  int64
	res  *store.CompanyResult
	used int64
}

type snapshot struct {
	jobs      []model.Job // slim (no descriptions), sorted most-recent first
	haystacks []string    // lowercased search text, aligned with jobs
	countries []string    // canonical country per job, aligned with jobs
	regions   []string    // canonical region per job, aligned with jobs
	states    []string    // US state / Canadian province per job ("" elsewhere)
	// facetParents nests the location facets: country -> region, state ->
	// country. The Remote / Other buckets are region-less and stay unlisted.
	facetParents  map[string]string
	byID          map[string]int
	companies     []CompanyMeta
	nameByID      map[string]string
	locations     []Facet
	countryFacets []Facet
	regionFacets  []Facet
	stateFacets   []Facet
	categories    []Facet
	sources       []Facet
	status        *store.RunStatus
	builtAt       time.Time
	generation    int64
}

// CompanyMeta describes a company for the /api/companies endpoint.
type CompanyMeta struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	ATS       string    `json:"ats"`
	Count     int       `json:"count"`
	OK        bool      `json:"ok"`
	FetchedAt time.Time `json:"fetchedAt"`
	Error     string    `json:"error,omitempty"`
}

// Facet is a value/count pair used for filter UIs. Location facets also name
// the facet they nest under — a country's parent is its region, a state's is its
// country — so the UI can narrow one list by the selection in another without
// another request.
type Facet struct {
	Value  string `json:"value"`
	Label  string `json:"label,omitempty"`
	Count  int    `json:"count"`
	Parent string `json:"parent,omitempty"`
}

// NewIndex builds an Index and performs the initial load.
func NewIndex(st *store.Store, log func(string, ...any)) *Index {
	if log == nil {
		log = func(string, ...any) {}
	}
	idx := &Index{store: st, log: log, descCache: map[string]cachedCompany{}}
	idx.snap.Store(emptySnapshot())
	idx.Reload()
	return idx
}

func emptySnapshot() *snapshot {
	return &snapshot{byID: map[string]int{}, nameByID: map[string]string{}, builtAt: time.Now()}
}

// JobByID returns the full job (including description) for an id, reading the
// owning company's file on demand and caching it briefly. Returns false if the
// job is not found.
func (i *Index) JobByID(id string) (model.Job, bool) {
	companyID, _, ok := strings.Cut(id, "~")
	if !ok || companyID == "" {
		// Fall back to the slim snapshot if the id is not in our canonical form.
		snap := i.snap.Load()
		if idx, ok := snap.byID[id]; ok {
			return snap.jobs[idx], true
		}
		return model.Job{}, false
	}
	res := i.readCompanyCached(companyID)
	if res == nil {
		// Company file missing; try the slim snapshot as a fallback.
		snap := i.snap.Load()
		if idx, ok := snap.byID[id]; ok {
			return snap.jobs[idx], true
		}
		return model.Job{}, false
	}
	for _, j := range res.Jobs {
		if j.ID == id {
			return j, true
		}
	}
	return model.Job{}, false
}

// readCompanyCached reads a company file, memoizing the parsed result until the
// next index reload (tracked by descGen).
func (i *Index) readCompanyCached(companyID string) *store.CompanyResult {
	i.descMu.Lock()
	gen := i.descGen
	if c, ok := i.descCache[companyID]; ok && c.gen == gen {
		i.descMu.Unlock()
		return c.res
	}
	i.descMu.Unlock()

	res, err := i.store.ReadCompany(companyID)
	if err != nil {
		return nil
	}

	i.descMu.Lock()
	// Bound the cache: evict an arbitrary entry when it grows too large.
	if len(i.descCache) >= 32 {
		for k := range i.descCache {
			delete(i.descCache, k)
			break
		}
	}
	i.descCache[companyID] = cachedCompany{gen: gen, res: res}
	i.descMu.Unlock()
	return res
}

// Snapshot returns the current immutable snapshot.
func (i *Index) Snapshot() *snapshot { return i.snap.Load() }

// Reload rebuilds the snapshot from disk if the data has changed. It is safe to
// call frequently.
func (i *Index) Reload() {
	i.reloadMu.Lock()
	defer i.reloadMu.Unlock()

	mtime := i.store.LatestMTime()
	if !mtime.IsZero() && mtime.Equal(i.lastMTime) && i.snap.Load().generation > 0 {
		return // nothing changed
	}

	results, errs := i.store.LoadAll()
	for _, e := range errs {
		i.log("index load warning: %v", e)
	}
	status, _ := i.store.ReadStatus()

	snap := buildSnapshot(results, status)
	snap.generation = i.snap.Load().generation + 1
	i.snap.Store(snap)
	i.lastMTime = mtime

	// Invalidate the description cache.
	i.descMu.Lock()
	i.descGen++
	i.descCache = map[string]cachedCompany{}
	i.descMu.Unlock()

	i.log("index rebuilt: %d jobs from %d companies", len(snap.jobs), len(snap.companies))
}

// StartAutoReload polls for data changes every interval until stop is closed.
func (i *Index) StartAutoReload(interval time.Duration, stop <-chan struct{}) {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	go func() {
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-stop:
				return
			case <-t.C:
				i.Reload()
			}
		}
	}()
}

func buildSnapshot(results []store.CompanyResult, status *store.RunStatus) *snapshot {
	var jobs []model.Job
	companies := make([]CompanyMeta, 0, len(results))
	locCount := map[string]int{}
	catCount := map[string]int{}
	srcCount := map[string]int{}

	for _, r := range results {
		companies = append(companies, CompanyMeta{
			ID: r.CompanyID, Name: r.Company, ATS: r.ATS,
			Count: len(r.Jobs), OK: r.OK, FetchedAt: r.FetchedAt, Error: r.Error,
		})
		for _, j := range r.Jobs {
			jobs = append(jobs, j.Slim())
			if loc := primaryLocation(j.Location); loc != "" {
				locCount[loc]++
			}
			for _, c := range j.Categories {
				catCount[c]++
			}
			srcCount[j.Source]++
		}
	}

	// Sort most-recent first; jobs with unknown dates sort last.
	sort.SliceStable(jobs, func(a, b int) bool {
		ta, tb := jobs[a].PostedAt, jobs[b].PostedAt
		if ta.IsZero() != tb.IsZero() {
			return !ta.IsZero() // known dates before unknown
		}
		return ta.After(tb)
	})

	haystacks := make([]string, len(jobs))
	countries := make([]string, len(jobs))
	regions := make([]string, len(jobs))
	states := make([]string, len(jobs))
	countryCount := map[string]int{}
	regionCount := map[string]int{}
	stateCount := map[string]int{}
	facetParents := map[string]string{}
	byID := make(map[string]int, len(jobs))
	for idx, j := range jobs {
		haystacks[idx] = strings.ToLower(j.Title + " \x00 " + j.Company + " \x00 " + j.Location + " \x00 " + j.Department + " \x00 " + strings.Join(j.Categories, " "))
		byID[j.ID] = idx
		locs := append([]string{j.Location}, j.Locations...)
		country, region, state := model.ResolveLocations(locs...)
		if country != "" {
			// Only real countries get a parent: the Remote / Other buckets span
			// every region ("Remote - EMEA" has a region but no country), so they
			// stay parent-less and the UI lists them under every region.
			facetParents[country] = region
		} else {
			country = unknownBucket(j)
		}
		if region == "" {
			region = unknownBucket(j)
		}
		if state != "" {
			stateCount[state]++
			facetParents[state] = country
		}
		countries[idx] = country
		regions[idx] = region
		states[idx] = state
		countryCount[country]++
		regionCount[region]++
	}

	sort.Slice(companies, func(a, b int) bool {
		if companies[a].Count != companies[b].Count {
			return companies[a].Count > companies[b].Count
		}
		return companies[a].Name < companies[b].Name
	})

	nameByID := make(map[string]string, len(companies))
	for _, c := range companies {
		nameByID[c.ID] = c.Name
	}

	return &snapshot{
		jobs:          jobs,
		haystacks:     haystacks,
		countries:     countries,
		regions:       regions,
		states:        states,
		facetParents:  facetParents,
		byID:          byID,
		companies:     companies,
		nameByID:      nameByID,
		locations:     topFacets(locCount, 60),
		countryFacets: withParents(sortedFacets(countryCount), facetParents),
		regionFacets:  sortedFacets(regionCount),
		stateFacets:   withParents(sortedFacets(stateCount), facetParents),
		categories:    sortedFacets(catCount),
		sources:       sortedFacets(srcCount),
		status:        status,
		builtAt:       time.Now(),
	}
}

// unknownBucket labels jobs whose location names no place we recognize, so the
// facet lists stay exhaustive instead of silently dropping them.
func unknownBucket(j model.Job) string {
	if j.Remote {
		return "Remote"
	}
	return "Other"
}

// primaryLocation reduces a possibly-multi location string to a stable facet key.
func primaryLocation(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	// Take the first listed location if several are comma/semicolon separated
	// and the string is long (heuristic to avoid splitting "City, State").
	if idx := strings.IndexAny(s, ";"); idx > 0 {
		s = strings.TrimSpace(s[:idx])
	}
	if len(s) > 48 {
		s = strings.TrimSpace(s[:48])
	}
	return s
}

func sortedFacets(m map[string]int) []Facet {
	out := make([]Facet, 0, len(m))
	for k, v := range m {
		out = append(out, Facet{Value: k, Count: v})
	}
	sort.Slice(out, func(a, b int) bool {
		if out[a].Count != out[b].Count {
			return out[a].Count > out[b].Count
		}
		return out[a].Value < out[b].Value
	})
	return out
}

// withParents tags location facets with the facet they nest under.
func withParents(facets []Facet, parents map[string]string) []Facet {
	for i := range facets {
		facets[i].Parent = parents[facets[i].Value]
	}
	return facets
}

func topFacets(m map[string]int, n int) []Facet {
	out := sortedFacets(m)
	if len(out) > n {
		out = out[:n]
	}
	return out
}
