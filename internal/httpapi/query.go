package httpapi

import (
	"sort"
	"strings"
	"time"

	"faangjobs/internal/model"
)

// Query is a normalized job search request.
type Query struct {
	Text       string
	Companies  map[string]bool
	Sources    map[string]bool
	Categories map[string]bool
	Location   string
	Country    string // canonical country facet value (see model.CountryOf)
	Region     string // canonical region facet value (see model.RegionOf)
	State      string // US state / Canadian province facet value (see model.StateOf)
	Remote     *bool
	Relocation *bool
	SinceDays  int
	Sort       string // "recent" (default), "oldest", "title", "company"
	Page       int
	PageSize   int
}

// QueryResult is the response payload for a job search.
type QueryResult struct {
	Total    int         `json:"total"`
	Page     int         `json:"page"`
	PageSize int         `json:"pageSize"`
	Jobs     []model.Job `json:"jobs"`
	Facets   Facets      `json:"facets"`
}

// Facets holds dynamic filter counts computed over the matched set.
type Facets struct {
	Companies  []Facet `json:"companies"`
	Categories []Facet `json:"categories"`
	Sources    []Facet `json:"sources"`
	Locations  []Facet `json:"locations"`
	Countries  []Facet `json:"countries"`
	Regions    []Facet `json:"regions"`
	States     []Facet `json:"states"`
}

// Run executes the query against the given snapshot.
func (s *snapshot) Run(q Query) QueryResult {
	tokens := tokenize(q.Text)
	locNeedle := strings.ToLower(strings.TrimSpace(q.Location))

	var cutoff time.Time
	if q.SinceDays > 0 {
		cutoff = time.Now().AddDate(0, 0, -q.SinceDays)
	}

	matched := make([]int, 0, 256)
	compCount := map[string]int{}
	catCount := map[string]int{}
	srcCount := map[string]int{}
	locCount := map[string]int{}
	countryCount := map[string]int{}
	regionCount := map[string]int{}
	stateCount := map[string]int{}

	for idx := range s.jobs {
		j := &s.jobs[idx]

		if len(tokens) > 0 && !matchAllTokens(s.haystacks[idx], tokens) {
			continue
		}
		if len(q.Companies) > 0 && !q.Companies[j.CompanyID] {
			continue
		}
		if len(q.Sources) > 0 && !q.Sources[j.Source] {
			continue
		}
		if len(q.Categories) > 0 && !anyCategory(j.Categories, q.Categories) {
			continue
		}
		if locNeedle != "" && !strings.Contains(strings.ToLower(j.Location), locNeedle) {
			continue
		}
		if q.Country != "" && s.countries[idx] != q.Country {
			continue
		}
		if q.Region != "" && s.regions[idx] != q.Region {
			continue
		}
		if q.State != "" && s.states[idx] != q.State {
			continue
		}
		if q.Remote != nil && j.Remote != *q.Remote {
			continue
		}
		if q.Relocation != nil && j.Relocation != *q.Relocation {
			continue
		}
		if q.SinceDays > 0 && (j.PostedAt.IsZero() || j.PostedAt.Before(cutoff)) {
			continue
		}

		matched = append(matched, idx)
		compCount[j.CompanyID]++
		srcCount[j.Source]++
		for _, c := range j.Categories {
			catCount[c]++
		}
		if loc := primaryLocation(j.Location); loc != "" {
			locCount[loc]++
		}
		countryCount[s.countries[idx]]++
		regionCount[s.regions[idx]]++
		// Jobs outside the US/Canada have no state; they are simply absent from
		// the state facet rather than lumped into a bucket.
		if st := s.states[idx]; st != "" {
			stateCount[st]++
		}
	}

	total := len(matched)

	// Ordering. The base slice is already most-recent-first.
	switch q.Sort {
	case "oldest":
		// Ascending by posted date, keeping unknown-date jobs last (mirrors the
		// "recent" invariant rather than a naive reverse, which would surface
		// undated jobs first).
		sort.SliceStable(matched, func(a, b int) bool {
			ta, tb := s.jobs[matched[a]].PostedAt, s.jobs[matched[b]].PostedAt
			if ta.IsZero() != tb.IsZero() {
				return !ta.IsZero()
			}
			return ta.Before(tb)
		})
	case "title":
		sort.SliceStable(matched, func(a, b int) bool {
			return strings.ToLower(s.jobs[matched[a]].Title) < strings.ToLower(s.jobs[matched[b]].Title)
		})
	case "company":
		sort.SliceStable(matched, func(a, b int) bool {
			ca, cb := s.jobs[matched[a]].Company, s.jobs[matched[b]].Company
			if !strings.EqualFold(ca, cb) {
				return strings.ToLower(ca) < strings.ToLower(cb)
			}
			return s.jobs[matched[a]].PostedAt.After(s.jobs[matched[b]].PostedAt)
		})
	default: // "recent" — already ordered
	}

	page := q.Page
	if page < 1 {
		page = 1
	}
	size := q.PageSize
	if size <= 0 {
		size = 25
	}
	// Generous cap: the frontend renders the full result set (no pagination UI)
	// and requests one large page.
	if size > 20000 {
		size = 20000
	}

	// Clamp page so (page-1)*size cannot overflow int for absurd page values
	// (e.g. ?page=100000000000000001). Beyond the last page → empty result.
	if maxPage := total/size + 1; page > maxPage {
		page = maxPage + 1
	}
	start := (page - 1) * size
	if start < 0 || start > total {
		start = total
	}
	end := start + size
	if end > total {
		end = total
	}

	jobs := make([]model.Job, 0, end-start)
	for _, mi := range matched[start:end] {
		jobs = append(jobs, s.jobs[mi])
	}

	return QueryResult{
		Total:    total,
		Page:     page,
		PageSize: size,
		Jobs:     jobs,
		Facets: Facets{
			Companies:  s.companyFacets(compCount),
			Categories: sortedFacets(catCount),
			Sources:    sortedFacets(srcCount),
			Locations:  topFacets(locCount, 40),
			Countries:  withParents(sortedFacets(countryCount), s.facetParents),
			Regions:    sortedFacets(regionCount),
			States:     withParents(sortedFacets(stateCount), s.facetParents),
		},
	}
}

func (s *snapshot) companyFacets(counts map[string]int) []Facet {
	out := make([]Facet, 0, len(counts))
	for id, n := range counts {
		out = append(out, Facet{Value: id, Label: s.nameByID[id], Count: n})
	}
	sort.Slice(out, func(a, b int) bool {
		if out[a].Count != out[b].Count {
			return out[a].Count > out[b].Count
		}
		return out[a].Label < out[b].Label
	})
	if len(out) > 80 {
		out = out[:80]
	}
	return out
}

func tokenize(s string) []string {
	fields := strings.Fields(strings.ToLower(s))
	out := fields[:0]
	for _, f := range fields {
		if len(f) > 0 {
			out = append(out, f)
		}
	}
	return out
}

func matchAllTokens(haystack string, tokens []string) bool {
	for _, t := range tokens {
		if !strings.Contains(haystack, t) {
			return false
		}
	}
	return true
}

func anyCategory(cats []string, want map[string]bool) bool {
	for _, c := range cats {
		if want[c] {
			return true
		}
	}
	return false
}
