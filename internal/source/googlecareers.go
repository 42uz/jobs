package source

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"faangjobs/internal/model"
	"faangjobs/internal/registry"
)

// googlecareers adapts Google's official careers site
// (google.com/about/careers/applications). There is no public JSON API in
// 2026; the jobs-results page embeds the job data as an AF_initDataCallback
// payload, which this adapter extracts and decodes.
//
// The positional layout of a job entry (verified live):
//
//	[0]  job id                      [1]  title
//	[7]  hiring company name         [9]  locations: [[display, ...], ...]
//	[10] [_, intro html]             [4]  [_, minimum qualifications html]
//	[19] [_, preferred quals html]   [3]  [_, responsibilities html]
//	[12] published [sec, nanos]      [13] updated [sec, nanos]
//
// data[2] carries the total match count. This layout is inherently fragile; a
// site change makes the adapter return errEmpty, which preserves last-good
// data rather than corrupting the board.
//
// Config keys: query (extra query string appended to the results URL — empty by
// default, i.e. every opening worldwide), maxJobs (default 8000).
type googlecareers struct{}

func init() { Register(googlecareers{}) }

func (googlecareers) Kind() string { return "googlecareers" }

var googDataRe = regexp.MustCompile(`(?s)AF_initDataCallback\(\{key: 'ds:1', hash: '[^']*', data:(.*?), sideChannel: \{\}\}\);`)

var googBrowserHeaders = map[string]string{
	"User-Agent": "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36",
	"Accept":     "text/html,application/xhtml+xml",
}

func (googlecareers) Fetch(ctx context.Context, f *Fetcher, c registry.Company) ([]model.Job, error) {
	query := configStr(c, "query", "")
	maxJobs := configInt(c, "maxJobs", 8000)
	const pageSize = 20 // fixed by the site

	var jobs []model.Job
	total := -1
	for page := 1; len(jobs) < maxJobs; page++ {
		pageURL := fmt.Sprintf("https://www.google.com/about/careers/applications/jobs/results?page=%d", page)
		if query != "" {
			pageURL += "&" + strings.TrimPrefix(query, "&")
		}
		html, err := f.Do(ctx, "GET", pageURL, nil, cloneHeaders(googBrowserHeaders))
		if err != nil {
			if len(jobs) > 0 {
				break
			}
			return nil, err
		}
		entries, pageTotal, err := parseGoogleJobs(html)
		if err != nil || len(entries) == 0 {
			if len(jobs) > 0 {
				break
			}
			if err == nil {
				err = fmt.Errorf("no job data found in page")
			}
			return nil, fmt.Errorf("parse google careers page: %w", err)
		}
		if total < 0 {
			total = pageTotal
		}
		jobs = append(jobs, entries...)
		if total > 0 && page*pageSize >= total {
			break
		}
	}
	return finalizeResult(c, jobs)
}

// parseGoogleJobs extracts the embedded job entries and total count from a
// jobs-results HTML page.
func parseGoogleJobs(html []byte) ([]model.Job, int, error) {
	m := googDataRe.FindSubmatch(html)
	if m == nil {
		return nil, 0, fmt.Errorf("ds:1 payload not found")
	}
	var data []any
	if err := json.Unmarshal(m[1], &data); err != nil {
		return nil, 0, fmt.Errorf("decode ds:1: %w", err)
	}
	if len(data) < 1 {
		return nil, 0, fmt.Errorf("ds:1 payload empty")
	}
	rawJobs, _ := data[0].([]any)
	total := 0
	if len(data) > 2 {
		total = int(getFloat(data[2]))
	}

	var out []model.Job
	for _, rj := range rawJobs {
		entry, ok := rj.([]any)
		if !ok || len(entry) < 15 {
			continue
		}
		id := getString(at(entry, 0))
		title := getString(at(entry, 1))
		company := getString(at(entry, 7))
		if id == "" || title == "" {
			continue
		}

		var locs []string
		if locArr, ok := at(entry, 9).([]any); ok {
			for _, l := range locArr {
				if parts, ok := l.([]any); ok && len(parts) > 0 {
					if disp := getString(parts[0]); disp != "" {
						locs = append(locs, disp)
					}
				}
			}
		}
		primary := ""
		if len(locs) > 0 {
			primary = locs[0]
		}

		desc := googHTMLPart(entry, 10) // intro
		if q := googHTMLPart(entry, 4); q != "" {
			desc += q
		}
		if q := googHTMLPart(entry, 19); q != "" {
			desc += q
		}
		if r := googHTMLPart(entry, 3); r != "" {
			desc += "<h3>Responsibilities</h3>" + r
		}

		job := model.Job{
			// ID left empty: finalize() derives a stable one from URL+title,
			// and the URL embeds Google's own job id.
			Company:     firstNonEmpty(company, "Google"),
			Title:       title,
			Location:    primary,
			Locations:   locs,
			URL:         "https://www.google.com/about/careers/applications/jobs/results/" + id + "-" + googSlug(title),
			Description: sanitizeHTML(desc),
			PostedAt:    googTime(at(entry, 12)),
			UpdatedAt:   googTime(at(entry, 13)),
		}
		out = append(out, job)
	}
	return out, total, nil
}

func at(arr []any, i int) any {
	if i < 0 || i >= len(arr) {
		return nil
	}
	return arr[i]
}

// googHTMLPart reads entry[i] shaped as [_, "<html>"].
func googHTMLPart(entry []any, i int) string {
	if pair, ok := at(entry, i).([]any); ok && len(pair) > 1 {
		return getString(pair[1])
	}
	return ""
}

// googTime reads a [seconds, nanos] pair.
func googTime(v any) time.Time {
	if pair, ok := v.([]any); ok && len(pair) > 0 {
		if secs := int64(getFloat(pair[0])); secs > 0 {
			return time.Unix(secs, 0).UTC()
		}
	}
	return time.Time{}
}

var googSlugRe = regexp.MustCompile(`[^a-z0-9]+`)

func googSlug(title string) string {
	s := googSlugRe.ReplaceAllString(strings.ToLower(title), "-")
	return url.PathEscape(strings.Trim(s, "-"))
}
