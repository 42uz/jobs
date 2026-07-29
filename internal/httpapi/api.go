package httpapi

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// API exposes the JSON endpoints backed by an Index.
type API struct {
	idx *Index
}

// NewAPI builds an API handler set.
func NewAPI(idx *Index) *API { return &API{idx: idx} }

// Register wires the API routes onto a mux.
func (a *API) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/jobs", a.handleJobs)
	mux.HandleFunc("GET /api/jobs/{id}", a.handleJobDetail)
	mux.HandleFunc("GET /api/companies", a.handleCompanies)
	mux.HandleFunc("GET /api/stats", a.handleStats)
	mux.HandleFunc("GET /api/filters", a.handleFilters)
	mux.HandleFunc("GET /api/healthz", a.handleHealth)
	mux.HandleFunc("GET /healthz", a.handleHealth)
	// Catch-all for unmatched /api/* paths: return a JSON 404 instead of falling
	// through to the SPA shell. More specific routes above take precedence.
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "unknown API endpoint")
	})
}

func (a *API) handleJobs(w http.ResponseWriter, r *http.Request) {
	q := parseQuery(r)
	snap := a.idx.Snapshot()
	res := snap.Run(q)
	writeJSON(w, http.StatusOK, res, 30*time.Second)
}

func (a *API) handleJobDetail(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	job, ok := a.idx.JobByID(id)
	if !ok {
		writeError(w, http.StatusNotFound, "job not found")
		return
	}
	writeJSON(w, http.StatusOK, job, 5*time.Minute)
}

func (a *API) handleCompanies(w http.ResponseWriter, r *http.Request) {
	snap := a.idx.Snapshot()
	writeJSON(w, http.StatusOK, map[string]any{
		"companies": snap.companies,
		"total":     len(snap.companies),
	}, time.Minute)
}

func (a *API) handleStats(w http.ResponseWriter, r *http.Request) {
	snap := a.idx.Snapshot()
	var lastUpdated time.Time
	var status any
	if snap.status != nil {
		lastUpdated = snap.status.FinishedAt
		status = snap.status
	}
	if lastUpdated.IsZero() {
		lastUpdated = snap.builtAt
	}
	companiesWithJobs := 0
	for _, c := range snap.companies {
		if c.Count > 0 {
			companiesWithJobs++
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"totalJobs":   len(snap.jobs),
		"companies":   companiesWithJobs,
		"sources":     snap.sources,
		"categories":  snap.categories,
		"lastUpdated": lastUpdated,
		"builtAt":     snap.builtAt,
		"run":         status,
	}, time.Minute)
}

func (a *API) handleFilters(w http.ResponseWriter, r *http.Request) {
	snap := a.idx.Snapshot()
	companyFacets := make([]Facet, 0, len(snap.companies))
	for _, c := range snap.companies {
		companyFacets = append(companyFacets, Facet{Value: c.ID, Label: c.Name, Count: c.Count})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"companies":  companyFacets,
		"categories": snap.categories,
		"sources":    snap.sources,
		"locations":  snap.locations,
		"countries":  snap.countryFacets,
		"regions":    snap.regionFacets,
		"states":     snap.stateFacets,
	}, time.Minute)
}

func (a *API) handleHealth(w http.ResponseWriter, r *http.Request) {
	snap := a.idx.Snapshot()
	writeJSON(w, http.StatusOK, map[string]any{
		"status":     "ok",
		"jobs":       len(snap.jobs),
		"companies":  len(snap.companies),
		"generation": snap.generation,
	}, 0)
}

// --- request parsing ---

func parseQuery(r *http.Request) Query {
	v := r.URL.Query()
	q := Query{
		Text:       strings.TrimSpace(v.Get("q")),
		Companies:  multiSet(v, "company"),
		Sources:    multiSet(v, "source"),
		Categories: multiSet(v, "category"),
		Location:   strings.TrimSpace(v.Get("location")),
		Country:    strings.TrimSpace(v.Get("country")),
		Region:     strings.TrimSpace(v.Get("region")),
		State:      strings.TrimSpace(v.Get("state")),
		Sort:       v.Get("sort"),
		Page:       atoiDefault(v.Get("page"), 1),
		PageSize:   atoiDefault(v.Get("pageSize"), 25),
		SinceDays:  atoiDefault(v.Get("since"), 0),
	}
	switch strings.ToLower(v.Get("remote")) {
	case "true", "1", "yes":
		t := true
		q.Remote = &t
	case "false", "0", "no":
		f := false
		q.Remote = &f
	}
	if strings.EqualFold(v.Get("relocation"), "true") || v.Get("relocation") == "1" {
		t := true
		q.Relocation = &t
	}
	return q
}

// multiSet collects a filter that may appear as repeated params and/or
// comma-separated values into a set.
func multiSet(v map[string][]string, key string) map[string]bool {
	vals := v[key]
	if len(vals) == 0 {
		return nil
	}
	out := map[string]bool{}
	for _, raw := range vals {
		for _, part := range strings.Split(raw, ",") {
			if p := strings.TrimSpace(part); p != "" {
				out[p] = true
			}
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func atoiDefault(s string, def int) int {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return n
}

// --- responses ---

func writeJSON(w http.ResponseWriter, status int, payload any, cache time.Duration) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if cache > 0 {
		w.Header().Set("Cache-Control", "public, max-age="+strconv.Itoa(int(cache.Seconds())))
	} else {
		w.Header().Set("Cache-Control", "no-store")
	}
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	_ = enc.Encode(payload)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg}, 0)
}
