package source

import (
	"testing"
	"time"

	"faangjobs/internal/model"
	"faangjobs/internal/registry"
)

func TestSanitizeHTML(t *testing.T) {
	in := `<p>Hi</p><script>alert(1)</script><a href="javascript:evil()" onclick="x()">y</a>`
	out := sanitizeHTML(in)
	if contains(out, "<script") || contains(out, "onclick") || contains(out, "javascript:") {
		t.Errorf("sanitizeHTML left dangerous content: %q", out)
	}
	if !contains(out, "<p>Hi</p>") {
		t.Errorf("sanitizeHTML dropped safe content: %q", out)
	}
}

func TestUnescapeAndClean(t *testing.T) {
	in := "&lt;h2&gt;Title&lt;/h2&gt;"
	out := unescapeAndClean(in)
	if out != "<h2>Title</h2>" {
		t.Errorf("unescapeAndClean = %q", out)
	}
}

func TestParseTime(t *testing.T) {
	cases := []string{
		"2026-07-13T14:37:36-04:00",
		"July 20, 2026",
		"2026-07-13",
	}
	for _, c := range cases {
		if parseTime(c).IsZero() {
			t.Errorf("parseTime(%q) returned zero", c)
		}
	}
	if !parseTime("not a date").IsZero() {
		t.Error("parseTime of garbage should be zero")
	}
}

func TestParseEpoch(t *testing.T) {
	ms := parseEpoch(1700000000000)
	sec := parseEpoch(1700000000)
	if ms.Year() != sec.Year() {
		t.Errorf("epoch ms/sec mismatch: %v vs %v", ms, sec)
	}
}

func TestParseWorkdayPosted(t *testing.T) {
	now := time.Date(2026, 7, 21, 12, 0, 0, 0, time.UTC)
	if !parseWorkdayPosted("Posted Today", now).Equal(now) {
		t.Error("today")
	}
	if d := parseWorkdayPosted("Posted 3 Days Ago", now); d.Day() != 18 {
		t.Errorf("3 days ago = %v", d)
	}
	if d := parseWorkdayPosted("Posted 30+ Days Ago", now); d.Day() != 21 || d.Month() != 6 {
		t.Errorf("30+ days ago = %v", d)
	}
}

func TestGetPath(t *testing.T) {
	root := map[string]any{
		"data": map[string]any{
			"jobs": []any{
				map[string]any{"title": "Eng", "loc": []any{"NYC", "SF"}},
			},
		},
	}
	if getString(getPath(root, "data.jobs.0.title")) != "Eng" {
		t.Error("nested path failed")
	}
	if getString(getPath(root, "data.jobs.0.loc")) != "NYC, SF" {
		t.Error("array-join failed")
	}
	if getPath(root, "data.missing.x") != nil {
		t.Error("missing path should be nil")
	}
}

func TestFinalizeResultDropsAllToEmpty(t *testing.T) {
	c := registry.Company{Name: "Test", ATS: "greenhouse", Slug: "test"}
	c.EnsureID()
	// All jobs are malformed (empty URL) and get dropped by finalize. The result
	// must be errEmpty (soft failure that preserves last-good), NOT a successful
	// zero-job crawl. Regression for the "check len before finalize" bug.
	_, err := finalizeResult(c, []model.Job{
		{Title: "Engineer", URL: ""},
		{Title: "", URL: "http://x"},
	})
	if err == nil {
		t.Fatal("expected errEmpty when all jobs are dropped, got nil")
	}
	// A single good job survives.
	jobs, err := finalizeResult(c, []model.Job{{Title: "Engineer", URL: "http://x"}})
	if err != nil || len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d (err=%v)", len(jobs), err)
	}
	if jobs[0].CompanyID != c.ID {
		t.Errorf("finalize did not set CompanyID: %q", jobs[0].CompanyID)
	}
}

func TestRegisteredAdapters(t *testing.T) {
	for _, kind := range []string{"greenhouse", "lever", "ashby", "smartrecruiters", "workable", "recruitee", "workday", "amazon", "generic"} {
		if _, ok := Get(kind); !ok {
			t.Errorf("adapter %q not registered", kind)
		}
	}
}

func TestConfigHelpers(t *testing.T) {
	c := registry.Company{Config: map[string]string{"maxJobs": "50", "host": "h"}}
	if configInt(c, "maxJobs", 10) != 50 {
		t.Error("configInt")
	}
	if configInt(c, "absent", 10) != 10 {
		t.Error("configInt default")
	}
	if configStr(c, "host", "") != "h" {
		t.Error("configStr")
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
