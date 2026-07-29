package source

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"faangjobs/internal/model"
	"faangjobs/internal/registry"
)

// applecareers adapts Apple's official careers site (jobs.apple.com). The
// search page embeds job records as JSON that has been string-escaped into a
// script tag. This adapter unescapes one level, then extracts each job object
// by brace-matching around its "positionId" key and decoding it as JSON.
//
// A site change makes the adapter return errEmpty (preserving last-good data)
// rather than emitting garbage.
//
// Config keys: query (extra query string, default "sort=newest"),
// maxJobs (default 8000; the site serves 20 jobs per page).
type applecareers struct{}

func init() { Register(applecareers{}) }

func (applecareers) Kind() string { return "applecareers" }

type appleJob struct {
	PositionID    string `json:"positionId"`
	PostingTitle  string `json:"postingTitle"`
	Transformed   string `json:"transformedPostingTitle"`
	PostDateInGMT string `json:"postDateInGMT"`
	JobSummary    string `json:"jobSummary"`
	HomeOffice    bool   `json:"homeOffice"`
	Team          struct {
		TeamName string `json:"teamName"`
	} `json:"team"`
	Locations []struct {
		Name        string `json:"name"`
		City        string `json:"city"`
		CountryName string `json:"countryName"`
	} `json:"locations"`
}

var appleBrowserHeaders = map[string]string{
	"User-Agent": "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36",
	"Accept":     "text/html,application/xhtml+xml",
}

func (applecareers) Fetch(ctx context.Context, f *Fetcher, c registry.Company) ([]model.Job, error) {
	query := configStr(c, "query", "sort=newest")
	maxJobs := configInt(c, "maxJobs", 8000)
	const pageSize = 20 // fixed by the site
	// Pages routinely repeat a few postings and occasionally carry an
	// unparseable record, so a short page is not the end of the listing: only a
	// run of pages without a single new job is. The site's own totalRecords
	// bounds the sweep.
	const emptyStreakLimit = 3

	var jobs []model.Job
	seen := map[string]bool{}
	total, maxPages := 0, maxJobs/pageSize+emptyStreakLimit+1
	emptyStreak := 0

	for page := 1; page <= maxPages && len(jobs) < maxJobs; page++ {
		pageURL := fmt.Sprintf("https://jobs.apple.com/en-us/search?%s&page=%d", query, page)
		html, err := f.Do(ctx, "GET", pageURL, nil, cloneHeaders(appleBrowserHeaders))
		if err != nil {
			if len(jobs) > 0 {
				break
			}
			return nil, err
		}
		text := unescapeOneLevel(string(html))
		if total == 0 {
			if n := parseAppleTotal(text); n > 0 {
				total = n
				if pages := n/pageSize + 2; pages < maxPages {
					maxPages = pages
				}
			}
		}
		fresh := 0
		for _, j := range parseAppleJobs(text, c) {
			if seen[j.ID] {
				continue
			}
			seen[j.ID] = true
			jobs = append(jobs, j)
			fresh++
		}
		if fresh == 0 {
			if emptyStreak++; emptyStreak >= emptyStreakLimit {
				break
			}
			continue
		}
		emptyStreak = 0
		if total > 0 && len(jobs) >= total {
			break
		}
	}
	return finalizeResult(c, jobs)
}

var appleTotalRe = regexp.MustCompile(`"totalRecords"\s*:\s*(\d+)`)

// parseAppleTotal reads the result count the search page reports.
func parseAppleTotal(text string) int {
	m := appleTotalRe.FindStringSubmatch(text)
	if m == nil {
		return 0
	}
	n, err := strconv.Atoi(m[1])
	if err != nil {
		return 0
	}
	return n
}

// parseAppleJobs extracts job objects from an unescaped search-results page.
func parseAppleJobs(text string, c registry.Company) []model.Job {
	var out []model.Job
	seen := map[string]bool{}
	for _, objText := range extractObjects(text, `"positionId":"`) {
		var j appleJob
		if err := json.Unmarshal([]byte(objText), &j); err != nil {
			continue
		}
		if j.PositionID == "" || j.PostingTitle == "" || seen[j.PositionID] {
			continue
		}
		seen[j.PositionID] = true

		var locs []string
		for _, l := range j.Locations {
			if d := firstNonEmpty(joinLoc(l.City, l.Name), l.CountryName); d != "" {
				locs = append(locs, d)
			}
		}
		primary := ""
		if len(locs) > 0 {
			primary = locs[0]
		}
		slug := firstNonEmpty(j.Transformed, "role")
		out = append(out, model.Job{
			ID:          c.ID + "~" + model.StableID(j.PositionID),
			Title:       j.PostingTitle,
			Location:    primary,
			Locations:   locs,
			Department:  j.Team.TeamName,
			Remote:      j.HomeOffice,
			URL:         fmt.Sprintf("https://jobs.apple.com/en-us/details/%s/%s", j.PositionID, slug),
			Description: sanitizeHTML(j.JobSummary),
			PostedAt:    parseTime(j.PostDateInGMT),
			UpdatedAt:   parseTime(j.PostDateInGMT),
		})
	}
	return out
}

// unescapeOneLevel undoes one level of JSON string escaping for the two
// characters that matter for structure (backslash and quote), so that a
// JSON-in-a-JSON-string blob becomes directly parseable JSON text.
func unescapeOneLevel(s string) string {
	const marker = "\x01"
	s = strings.ReplaceAll(s, `\\`, marker)
	s = strings.ReplaceAll(s, `\"`, `"`)
	return strings.ReplaceAll(s, marker, `\`)
}

// extractObjects returns every balanced JSON object in text that contains the
// given key marker. For each occurrence it walks backward to the object's
// opening brace and forward to its closing brace, counting nesting while
// skipping over string literals.
func extractObjects(text, marker string) []string {
	var out []string
	from := 0
	for {
		i := strings.Index(text[from:], marker)
		if i < 0 {
			break
		}
		i += from
		// Start the backward walk just BEFORE the marker's opening quote: that
		// position is genuinely outside any string, which keeps the in-string
		// parity of the backward scan correct.
		start := walkBack(text, i-1)
		end := walkForward(text, start)
		if start >= 0 && end > start {
			out = append(out, text[start:end+1])
			from = end + 1
		} else {
			from = i + len(marker)
		}
	}
	return out
}

// walkBack finds the opening '{' of the object enclosing position i.
func walkBack(s string, i int) int {
	depth := 0
	inStr := false
	for p := i; p >= 0; p-- {
		ch := s[p]
		if inStr {
			// A quote closes the string unless escaped (look behind for '\').
			if ch == '"' && (p == 0 || s[p-1] != '\\') {
				inStr = false
			}
			continue
		}
		switch ch {
		case '"':
			if p == 0 || s[p-1] != '\\' {
				inStr = true
			}
		case '}':
			depth++
		case '{':
			if depth == 0 {
				return p
			}
			depth--
		}
	}
	return -1
}

// walkForward finds the matching '}' for the '{' at position start.
func walkForward(s string, start int) int {
	if start < 0 || start >= len(s) || s[start] != '{' {
		return -1
	}
	depth := 0
	inStr := false
	esc := false
	for p := start; p < len(s); p++ {
		ch := s[p]
		if inStr {
			if esc {
				esc = false
			} else if ch == '\\' {
				esc = true
			} else if ch == '"' {
				inStr = false
			}
			continue
		}
		switch ch {
		case '"':
			inStr = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return p
			}
		}
	}
	return -1
}
