package source

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strings"

	"faangjobs/internal/model"
	"faangjobs/internal/registry"
)

// amazon adapts the public amazon.jobs search.json endpoint.
//
// The endpoint refuses offsets at or beyond 10 000 ("Cannot return more than
// 10000 results at once") and reports a capped `hits` of exactly 10 000 for any
// query that large — so a plain sweep silently truncates Amazon's global
// listing. When the unfiltered query is capped, the adapter shards the crawl by
// job category instead: every category is comfortably below the cap, and the
// union is Amazon's full board.
type amazon struct{}

func init() { Register(amazon{}) }

func (amazon) Kind() string { return "amazon" }

// amazonResultCap is the deepest offset the search endpoint will serve.
const amazonResultCap = 10000

// amazonCategories are the job_category facet values used for sharding. They
// are stable, but the adapter also unions in whatever categories it sees in the
// most recent postings, so a category Amazon adds later is still swept.
var amazonCategories = []string{
	"Software Development", "Operations, IT, & Support Engineering",
	"Fulfillment & Operations Management", "Sales, Advertising, & Account Management",
	"Fulfillment Associate", "Fulfillment / Warehouse Associate",
	"Project/Program/Product Management--Technical",
	"Project/Program/Product Management--Non-Tech",
	"Solutions Architect", "Facilities, Maintenance, & Real Estate", "Customer Service",
	"Hardware Development", "Business & Merchant Development", "Medical, Health, & Safety",
	"Finance & Accounting", "Administrative Support", "Supply Chain/Transportation Management",
	"Applied Science", "Marketing & PR", "PR", "Systems, Quality, & Security Engineering",
	"Corporate Operations", "Human Resources", "Buying, Planning, & Instock Management",
	"Audio / Video / Photography Production", "Design", "Business Intelligence",
	"Editorial, Writing, & Content Management", "Legal", "Procurement", "Data Science",
	"Machine Learning Science", "Economics", "Investigation & Loss Prevention",
	"Leadership Development & Training", "Public Policy", "Research Science",
}

type amazonResponse struct {
	Hits int         `json:"hits"`
	Jobs []amazonJob `json:"jobs"`
}

type amazonJob struct {
	IDIcims                 string   `json:"id_icims"`
	Title                   string   `json:"title"`
	JobPath                 string   `json:"job_path"`
	Location                string   `json:"location"`
	NormalizedLocation      string   `json:"normalized_location"`
	Locations               []string `json:"locations"`
	City                    string   `json:"city"`
	PostedDate              string   `json:"posted_date"`
	UpdatedTime             string   `json:"updated_time"`
	Description             string   `json:"description"`
	BasicQualifications     string   `json:"basic_qualifications"`
	PreferredQualifications string   `json:"preferred_qualifications"`
	JobCategory             string   `json:"job_category"`
	Team                    struct {
		Label string `json:"label"`
	} `json:"team"`
	BusinessCategory string `json:"business_category"`
}

func (amazon) Fetch(ctx context.Context, f *Fetcher, c registry.Company) ([]model.Job, error) {
	const pageSize = 100
	maxJobs := configInt(c, "maxJobs", 30000)

	var jobs []model.Job
	seen := map[string]bool{}
	seenCats := map[string]bool{}

	// sweep paginates one (optionally filtered) query from start up to budget
	// results, appending jobs it has not collected yet. It returns the hit count
	// the endpoint reported.
	sweep := func(filter string, start, budget int) (int, error) {
		if budget > amazonResultCap {
			budget = amazonResultCap
		}
		hits := 0
		for offset := start; offset < budget && len(jobs) < maxJobs; offset += pageSize {
			reqURL := fmt.Sprintf("https://www.amazon.jobs/en/search.json?result_limit=%d&offset=%d&sort=recent%s", pageSize, offset, filter)
			var resp amazonResponse
			if err := f.GetJSON(ctx, reqURL, nil, &resp); err != nil {
				return hits, err
			}
			hits = resp.Hits
			if len(resp.Jobs) == 0 {
				break
			}
			for _, j := range resp.Jobs {
				if j.JobCategory != "" {
					seenCats[j.JobCategory] = true
				}
				id := c.ID + "~" + model.StableID(j.IDIcims)
				if seen[id] {
					continue
				}
				seen[id] = true
				desc := j.Description
				if j.BasicQualifications != "" {
					desc += "<h3>Basic qualifications</h3>" + j.BasicQualifications
				}
				if j.PreferredQualifications != "" {
					desc += "<h3>Preferred qualifications</h3>" + j.PreferredQualifications
				}
				jobs = append(jobs, model.Job{
					ID:          id,
					Title:       j.Title,
					Location:    tidyAmazonLoc(firstNonEmpty(j.NormalizedLocation, j.Location, j.City)),
					Locations:   j.Locations,
					Department:  firstNonEmpty(j.JobCategory, j.Team.Label, j.BusinessCategory),
					URL:         "https://www.amazon.jobs" + j.JobPath,
					Description: sanitizeHTML(desc),
					PostedAt:    parseTime(j.PostedDate),
					UpdatedAt:   parseTime(firstNonEmpty(j.UpdatedTime, j.PostedDate)),
				})
			}
			if offset+len(resp.Jobs) >= hits {
				break
			}
		}
		return hits, nil
	}

	// Probe the unfiltered listing: a few pages are enough to learn the total
	// and the currently-used categories.
	const probeDepth = 3 * pageSize
	hits, err := sweep("", 0, probeDepth)
	if err != nil && len(jobs) == 0 {
		return nil, err
	}

	if hits < amazonResultCap {
		// Small enough to page through directly.
		if _, err := sweep("", probeDepth, hits); err != nil && len(jobs) == 0 {
			return nil, err
		}
		return finalizeResult(c, jobs)
	}

	// Capped: sweep category by category. Each shard is well under the cap, so
	// together they cover the whole board.
	for _, cat := range amazonShardCategories(seenCats) {
		if len(jobs) >= maxJobs {
			break
		}
		filter := "&category%5B%5D=" + url.QueryEscape(cat)
		if _, err := sweep(filter, 0, amazonResultCap); err != nil && len(jobs) == 0 {
			return nil, err
		}
	}
	return finalizeResult(c, jobs)
}

// amazonShardCategories returns the known category list plus any category seen
// in the freshly-fetched postings that is not part of it.
func amazonShardCategories(seen map[string]bool) []string {
	out := make([]string, len(amazonCategories))
	copy(out, amazonCategories)
	known := make(map[string]bool, len(amazonCategories))
	for _, c := range amazonCategories {
		known[c] = true
	}
	var extra []string
	for c := range seen {
		if !known[c] {
			extra = append(extra, c)
		}
	}
	sort.Strings(extra) // deterministic sweep order
	return append(out, extra...)
}

func tidyAmazonLoc(s string) string {
	parts := strings.Split(s, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	// reverse "US, WA, Seattle" -> "Seattle, WA, US"
	if len(parts) == 3 {
		return parts[2] + ", " + parts[1] + ", " + parts[0]
	}
	return s
}
