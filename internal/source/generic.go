package source

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"faangjobs/internal/model"
	"faangjobs/internal/registry"
)

// generic is a data-driven adapter for bespoke company endpoints (used mostly
// for the big-tech APIs that don't share a common ATS). Everything it needs is
// supplied through the company's Config, so new endpoints can be added by
// editing companies.json alone — no new Go code.
//
// Recognized Config keys:
//
//	method      GET (default) | POST
//	url         request URL; may contain {{offset}} {{page}} {{limit}}
//	headers     JSON object of request headers
//	body        POST body template; may contain the same placeholders
//	jobsPath    dot-path to the array of jobs in the response (e.g. "data.jobs")
//	totalPath   dot-path to the total count (optional, enables paging)
//	pageMode    none (default) | offset | page
//	pageSize    page size (default 100)
//	pageStart   first page/offset value (default 0)
//	maxJobs     safety cap (default 2000)
//	titlePath, locationPath, urlPath, deptPath, postedPath, descPath, idPath, remotePath
//	            dot-paths within each job object
//	urlPrefix   prepended to the extracted url if it is relative
//	company     overrides the display company name
type generic struct{}

func init() { Register(generic{}) }

func (generic) Kind() string { return "generic" }

func (generic) Fetch(ctx context.Context, f *Fetcher, c registry.Company) ([]model.Job, error) {
	urlTmpl := configStr(c, "url", "")
	if urlTmpl == "" {
		return nil, fmt.Errorf("generic company %q missing url config", c.ID)
	}
	method := strings.ToUpper(configStr(c, "method", "GET"))
	jobsPath := configStr(c, "jobsPath", "jobs")
	totalPath := configStr(c, "totalPath", "")
	pageMode := configStr(c, "pageMode", "none")
	pageSize := configInt(c, "pageSize", 100)
	pageStart := configInt(c, "pageStart", 0)
	maxJobs := configInt(c, "maxJobs", 2000)
	urlPrefix := configStr(c, "urlPrefix", "")

	var headers map[string]string
	configJSON(c, "headers", &headers)
	bodyTmpl := configStr(c, "body", "")

	fields := map[string]string{
		"title": configStr(c, "titlePath", "title"),
		"loc":   configStr(c, "locationPath", "location"),
		"url":   configStr(c, "urlPath", "url"),
		"dept":  configStr(c, "deptPath", "department"),
		"post":  configStr(c, "postedPath", "postedAt"),
		"desc":  configStr(c, "descPath", "description"),
		"id":    configStr(c, "idPath", "id"),
		"rem":   configStr(c, "remotePath", "remote"),
	}
	companyName := configStr(c, "company", c.Name)

	if pageSize < 1 {
		pageSize = 100
	}
	// Absolute page cap so a source that returns full pages of unusable items
	// (all skipped) can never loop indefinitely — len(jobs) alone would stay
	// below maxJobs forever in that case.
	maxPages := maxJobs/pageSize + 5

	var jobs []model.Job
	offset := pageStart
	page := pageStart
	for iter := 0; iter < maxPages && len(jobs) < maxJobs; iter++ {
		repl := func(s string) string {
			s = strings.ReplaceAll(s, "{{offset}}", strconv.Itoa(offset))
			s = strings.ReplaceAll(s, "{{page}}", strconv.Itoa(page))
			s = strings.ReplaceAll(s, "{{limit}}", strconv.Itoa(pageSize))
			return s
		}
		url := repl(urlTmpl)

		var root any
		var err error
		if method == "POST" {
			err = f.PostJSON(ctx, url, repl(bodyTmpl), cloneHeaders(headers), &root)
		} else {
			err = f.GetJSON(ctx, url, headers, &root)
		}
		if err != nil {
			if len(jobs) > 0 {
				break
			}
			return nil, err
		}

		arr, ok := getPath(root, jobsPath).([]any)
		if !ok || len(arr) == 0 {
			break
		}
		for _, item := range arr {
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}
			rawURL := getString(getPath(m, fields["url"]))
			if urlPrefix != "" && rawURL != "" && !strings.HasPrefix(rawURL, "http") {
				rawURL = strings.TrimRight(urlPrefix, "/") + "/" + strings.TrimLeft(rawURL, "/")
			}
			id := getString(getPath(m, fields["id"]))
			jobs = append(jobs, model.Job{
				ID:          c.ID + "~" + model.StableID(firstNonEmpty(id, rawURL)),
				Company:     companyName,
				Title:       getString(getPath(m, fields["title"])),
				Location:    getString(getPath(m, fields["loc"])),
				Department:  getString(getPath(m, fields["dept"])),
				Remote:      getBool(getPath(m, fields["rem"])),
				URL:         rawURL,
				Description: sanitizeHTML(getString(getPath(m, fields["desc"]))),
				PostedAt:    parseAnyTime(getPath(m, fields["post"])),
				UpdatedAt:   parseAnyTime(getPath(m, fields["post"])),
			})
			if len(jobs) >= maxJobs {
				break
			}
		}

		// Decide whether to continue paging.
		if pageMode == "none" {
			break
		}
		if totalPath != "" {
			if total := int(getFloat(getPath(root, totalPath))); total > 0 && offset+len(arr) >= total {
				break
			}
		}
		if len(arr) < pageSize {
			break
		}
		offset += pageSize
		page++
	}
	return finalizeResult(c, jobs)
}

func cloneHeaders(h map[string]string) map[string]string {
	if h == nil {
		return nil
	}
	out := make(map[string]string, len(h))
	for k, v := range h {
		out[k] = v
	}
	return out
}

// getPath navigates a decoded-JSON value by a dot-separated path. Numeric
// segments index into arrays; other segments index into objects.
func getPath(v any, path string) any {
	if path == "" {
		return v
	}
	cur := v
	for _, seg := range strings.Split(path, ".") {
		switch node := cur.(type) {
		case map[string]any:
			cur = node[seg]
		case []any:
			idx, err := strconv.Atoi(seg)
			if err != nil || idx < 0 || idx >= len(node) {
				return nil
			}
			cur = node[idx]
		default:
			return nil
		}
		if cur == nil {
			return nil
		}
	}
	return cur
}

func getString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(t)
	case []any:
		var parts []string
		for _, e := range t {
			if s := getString(e); s != "" {
				parts = append(parts, s)
			}
		}
		return strings.Join(parts, ", ")
	default:
		return ""
	}
}

func getBool(v any) bool {
	switch t := v.(type) {
	case bool:
		return t
	case string:
		return strings.EqualFold(t, "true") || strings.Contains(strings.ToLower(t), "remote")
	default:
		return false
	}
}

func getFloat(v any) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case string:
		n, _ := strconv.ParseFloat(t, 64)
		return n
	default:
		return 0
	}
}

func parseAnyTime(v any) time.Time {
	switch x := v.(type) {
	case string:
		return parseTime(x)
	case float64:
		return parseEpoch(int64(x))
	default:
		return time.Time{}
	}
}
