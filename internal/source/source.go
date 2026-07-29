// Package source contains the pluggable ATS adapters. Each adapter knows how to
// talk to one kind of job-board backend (Greenhouse, Lever, Ashby, Workday, a
// company's bespoke API, ...) and normalize the result into []model.Job.
package source

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"faangjobs/internal/model"
	"faangjobs/internal/registry"
)

// Source fetches and normalizes all currently-listed jobs for one company.
//
// Implementations must:
//   - honor ctx cancellation/deadlines,
//   - return a non-nil error on failure (so the crawler preserves prior data),
//   - be safe for concurrent use (a single instance serves many companies).
type Source interface {
	// Kind is the adapter identifier, matched against Company.ATS.
	Kind() string
	// Fetch returns the normalized jobs for the company.
	Fetch(ctx context.Context, f *Fetcher, c registry.Company) ([]model.Job, error)
}

var (
	mu         sync.RWMutex
	registered = map[string]Source{}
)

// Register makes an adapter available under its Kind(). It is meant to be
// called from adapter init() functions.
func Register(s Source) {
	mu.Lock()
	defer mu.Unlock()
	registered[s.Kind()] = s
}

// Get returns the adapter for kind, if registered.
func Get(kind string) (Source, bool) {
	mu.RLock()
	defer mu.RUnlock()
	s, ok := registered[kind]
	return s, ok
}

// Kinds lists all registered adapter kinds, sorted.
func Kinds() []string {
	mu.RLock()
	defer mu.RUnlock()
	out := make([]string, 0, len(registered))
	for k := range registered {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// finalize applies the shared post-processing every adapter needs: it fills the
// company fields, derives categories and remote status, cleans text, and drops
// obviously-empty jobs. It never returns nil.
func finalize(c registry.Company, jobs []model.Job) []model.Job {
	out := make([]model.Job, 0, len(jobs))
	seen := map[string]bool{}
	for _, j := range jobs {
		j.Title = model.CleanText(j.Title)
		if j.Title == "" || j.URL == "" {
			continue
		}
		j.CompanyID = c.ID
		if j.Company == "" {
			j.Company = c.Name
		}
		j.Source = c.ATS
		j.Location = model.CleanText(j.Location)
		for i := range j.Locations {
			j.Locations[i] = model.CleanText(j.Locations[i])
		}
		if !j.Remote {
			j.Remote = model.DetectRemote(append(j.Locations, j.Location)...)
		}
		if !j.Relocation {
			j.Relocation = model.MentionsRelocation(j.Title, j.Department, j.Description)
		}
		if len(j.Categories) == 0 {
			j.Categories = model.Categorize(j.Title, j.Department)
		}
		if j.ID == "" {
			j.ID = c.ID + "~" + model.StableID(j.URL, j.Title)
		}
		if seen[j.ID] {
			continue
		}
		seen[j.ID] = true
		out = append(out, j)
	}
	return out
}

// errEmpty is returned when a source responds successfully but lists no jobs.
// The crawler treats this as a soft failure and keeps any prior data.
var errEmpty = fmt.Errorf("source returned zero usable jobs")

// finalizeResult post-processes the raw jobs and returns errEmpty when nothing
// usable survives. Adapters must use this (not a raw len(jobs) check) so that a
// source returning N>0 malformed records — which finalize() drops — is treated
// as a soft failure that preserves the last-good snapshot, rather than a
// "successful" zero-job crawl that wipes it.
func finalizeResult(c registry.Company, jobs []model.Job) ([]model.Job, error) {
	out := finalize(c, jobs)
	if len(out) == 0 {
		return nil, errEmpty
	}
	return out, nil
}
