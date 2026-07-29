package source

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"faangjobs/internal/model"
	"faangjobs/internal/registry"
)

// workday adapts Workday "cxs" job-search endpoints. Because every tenant has
// its own host and site id, these are provided via the company Config:
//
//	config: {
//	  "host": "nvidia.wd5.myworkdayjobs.com",
//	  "site": "NVIDIAExternalCareerSite",
//	  "tenant": "nvidia",   // optional; defaults to slug
//	  "lang": "en-US"        // optional
//	}
//
// Workday's list view hides multi-office locations behind "N Locations"
// strings, which would make such jobs invisible to any geographic filter, and
// its unfiltered `total` is capped, which hides jobs from a plain pagination
// sweep. To fix both, the adapter discovers the tenant's country facet from the
// first response and sweeps every country separately, so each job gains a
// country-level location hint and no country's tail is cut off.
type workday struct{}

func init() { Register(workday{}) }

func (workday) Kind() string { return "workday" }

type wdResponse struct {
	Total       int         `json:"total"`
	JobPostings []wdPosting `json:"jobPostings"`
	Facets      []wdFacet   `json:"facets"`
}

type wdPosting struct {
	Title         string   `json:"title"`
	ExternalPath  string   `json:"externalPath"`
	LocationsText string   `json:"locationsText"`
	PostedOn      string   `json:"postedOn"`
	BulletFields  []string `json:"bulletFields"`
}

// wdFacet is recursive: location facets nest under "locationMainGroup".
type wdFacet struct {
	FacetParameter string    `json:"facetParameter"`
	Descriptor     string    `json:"descriptor"`
	Values         []wdFacet `json:"values"`
	ID             string    `json:"id"`
	Count          int       `json:"count"`
}

var nLocationsRe = regexp.MustCompile(`^\d+\s+[Ll]ocations?$`)

func (workday) Fetch(ctx context.Context, f *Fetcher, c registry.Company) ([]model.Job, error) {
	host := configStr(c, "host", "")
	site := configStr(c, "site", "")
	tenant := configStr(c, "tenant", c.Slug)
	lang := configStr(c, "lang", "en-US")
	if host == "" || site == "" {
		return nil, fmt.Errorf("workday company %q missing host/site config", c.ID)
	}
	endpoint := fmt.Sprintf("https://%s/wday/cxs/%s/%s/jobs", host, tenant, site)
	maxJobs := configInt(c, "maxJobs", 20000)
	urlBase := fmt.Sprintf("https://%s/%s/%s", host, lang, site)
	now := time.Now().UTC()

	query := func(offset, limit int, facets map[string]any) (*wdResponse, error) {
		body := map[string]any{"limit": limit, "offset": offset, "searchText": ""}
		if facets == nil {
			facets = map[string]any{}
		}
		body["appliedFacets"] = facets
		var resp wdResponse
		if err := f.PostJSON(ctx, endpoint, body, nil, &resp); err != nil {
			return nil, err
		}
		return &resp, nil
	}

	probe, err := query(0, 1, nil)
	if err != nil {
		return nil, err
	}

	seen := map[string]bool{}
	var jobs []model.Job
	collect := func(resp *wdResponse, countryHint string) {
		for _, p := range resp.JobPostings {
			if p.ExternalPath == "" || seen[p.ExternalPath] {
				continue
			}
			seen[p.ExternalPath] = true
			reqID := ""
			if len(p.BulletFields) > 0 {
				reqID = p.BulletFields[0]
			}
			loc := model.CleanText(p.LocationsText)
			var locs []string
			if loc != "" && !nLocationsRe.MatchString(loc) {
				locs = append(locs, loc)
			} else if countryHint != "" {
				// The list view hides the actual offices; surface the country
				// we filtered by instead of an opaque "5 Locations".
				loc = countryHint
			}
			if countryHint != "" {
				locs = append(locs, countryHint)
			}
			jobs = append(jobs, model.Job{
				ID:        c.ID + "~" + model.StableID(firstNonEmpty(reqID, p.ExternalPath)),
				Title:     p.Title,
				Location:  loc,
				Locations: locs,
				URL:       urlBase + p.ExternalPath,
				PostedAt:  parseWorkdayPosted(p.PostedOn, now),
				UpdatedAt: parseWorkdayPosted(p.PostedOn, now),
			})
		}
	}

	// paginate runs one filtered pagination sweep. Workday only reports `total`
	// on the first page of a sweep (later pages carry 0), so the total is
	// remembered rather than re-read — reading it per page would end every
	// sweep after two pages.
	paginate := func(facets map[string]any, countryHint string, budget int) error {
		const pageSize = 20
		total := 0
		for offset := 0; len(jobs) < maxJobs && offset < budget; offset += pageSize {
			resp, err := query(offset, pageSize, facets)
			if err != nil {
				return err
			}
			if len(resp.JobPostings) == 0 {
				return nil
			}
			collect(resp, countryHint)
			if resp.Total > total {
				total = resp.Total
			}
			if len(resp.JobPostings) < pageSize {
				return nil
			}
			if total > 0 && offset+len(resp.JobPostings) >= total {
				return nil
			}
		}
		return nil
	}

	param, values := findCountryFacet(probe.Facets)
	if param == "" {
		// No usable country facet: plain pagination over everything.
		if err := paginate(nil, "", maxJobs); err != nil && len(jobs) == 0 {
			return nil, err
		}
		return finalizeResult(c, jobs)
	}
	// Country-level facet: sweep each country separately so every job gets an
	// accurate country attribution and no country's tail is truncated.
	for _, v := range values {
		if len(jobs) >= maxJobs {
			break
		}
		err := paginate(map[string]any{param: []string{v.ID}}, v.Descriptor, v.Count+40)
		if err != nil && len(jobs) == 0 {
			return nil, err
		}
	}
	return finalizeResult(c, jobs)
}

// findCountryFacet walks the facet tree and returns the tenant's country facet
// parameter plus its non-empty values. Most tenants expose "locationCountry" or
// "locationHierarchy1"; some (Salesforce, for one) ship a custom calculated
// field instead, so any facet whose parameter mentions "country" is accepted as
// a fallback. Tenants with no country facet are crawled by plain pagination.
func findCountryFacet(facets []wdFacet) (string, []wdFacet) {
	var param, altParam string
	var values, altValues []wdFacet

	nonEmpty := func(fc wdFacet) []wdFacet {
		var out []wdFacet
		for _, v := range fc.Values {
			if v.Count > 0 {
				out = append(out, v)
			}
		}
		return out
	}

	var walk func(fs []wdFacet)
	walk = func(fs []wdFacet) {
		for _, fc := range fs {
			switch {
			case fc.FacetParameter == "locationCountry" || fc.FacetParameter == "locationHierarchy1":
				if param == "" {
					param, values = fc.FacetParameter, nonEmpty(fc)
				}
			case strings.Contains(strings.ToLower(fc.FacetParameter), "country"):
				if altParam == "" {
					altParam, altValues = fc.FacetParameter, nonEmpty(fc)
				}
			}
			// Descend into facet groups (e.g. locationMainGroup).
			if len(fc.Values) > 0 && fc.Values[0].FacetParameter != "" {
				walk(fc.Values)
			}
		}
	}
	walk(facets)

	if len(values) == 0 {
		param, values = altParam, altValues
	}
	if len(values) == 0 {
		return "", nil
	}
	return param, values
}

// parseWorkdayPosted interprets Workday's relative "postedOn" strings such as
// "Posted Today", "Posted Yesterday", "Posted 3 Days Ago", "Posted 30+ Days Ago".
func parseWorkdayPosted(s string, now time.Time) time.Time {
	l := strings.ToLower(strings.TrimSpace(s))
	l = strings.TrimPrefix(l, "posted ")
	switch {
	case l == "":
		return time.Time{}
	case strings.Contains(l, "today"):
		return now
	case strings.Contains(l, "yesterday"):
		return now.AddDate(0, 0, -1)
	}
	// "N days ago" / "N+ days ago"
	fields := strings.Fields(l)
	if len(fields) >= 2 && strings.HasPrefix(fields[1], "day") {
		numStr := strings.TrimSuffix(fields[0], "+")
		var n int
		if _, err := fmt.Sscanf(numStr, "%d", &n); err == nil && n >= 0 {
			return now.AddDate(0, 0, -n)
		}
	}
	return time.Time{}
}
