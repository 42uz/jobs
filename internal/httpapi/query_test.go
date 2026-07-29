package httpapi

import (
	"testing"
	"time"

	"faangjobs/internal/model"
	"faangjobs/internal/store"
)

func testSnapshot() *snapshot {
	now := time.Now()
	results := []store.CompanyResult{
		{CompanyID: "greenhouse-stripe", Company: "Stripe", ATS: "greenhouse", OK: true, Jobs: []model.Job{
			{ID: "greenhouse-stripe~1", CompanyID: "greenhouse-stripe", Company: "Stripe", Title: "Senior Software Engineer", Location: "San Francisco, CA", Source: "greenhouse", Categories: []string{"Software Engineering"}, PostedAt: now.AddDate(0, 0, -1), URL: "u1"},
			{ID: "greenhouse-stripe~2", CompanyID: "greenhouse-stripe", Company: "Stripe", Title: "Product Designer", Location: "Remote - US", Remote: true, Source: "greenhouse", Categories: []string{"Design"}, PostedAt: now.AddDate(0, 0, -10), URL: "u2"},
		}},
		{CompanyID: "ashby-ramp", Company: "Ramp", ATS: "ashby", OK: true, Jobs: []model.Job{
			{ID: "ashby-ramp~1", CompanyID: "ashby-ramp", Company: "Ramp", Title: "Machine Learning Engineer", Location: "New York, NY", Source: "ashby", Categories: []string{"Data & ML"}, PostedAt: now.AddDate(0, 0, -2), URL: "u3"},
		}},
	}
	return buildSnapshot(results, nil)
}

func TestQueryText(t *testing.T) {
	s := testSnapshot()
	r := s.Run(Query{Text: "engineer"})
	if r.Total != 2 {
		t.Errorf("text 'engineer' total = %d, want 2", r.Total)
	}
	r = s.Run(Query{Text: "software engineer"})
	if r.Total != 1 {
		t.Errorf("text 'software engineer' total = %d, want 1", r.Total)
	}
}

func TestQueryFilters(t *testing.T) {
	s := testSnapshot()
	if r := s.Run(Query{Companies: map[string]bool{"ashby-ramp": true}}); r.Total != 1 {
		t.Errorf("company filter total = %d, want 1", r.Total)
	}
	if r := s.Run(Query{Categories: map[string]bool{"Design": true}}); r.Total != 1 {
		t.Errorf("category filter total = %d, want 1", r.Total)
	}
	tru := true
	if r := s.Run(Query{Remote: &tru}); r.Total != 1 {
		t.Errorf("remote filter total = %d, want 1", r.Total)
	}
	if r := s.Run(Query{Location: "new york"}); r.Total != 1 {
		t.Errorf("location filter total = %d, want 1", r.Total)
	}
	if r := s.Run(Query{SinceDays: 3}); r.Total != 2 {
		t.Errorf("since 3 days total = %d, want 2", r.Total)
	}
}

func TestQuerySortAndPage(t *testing.T) {
	s := testSnapshot()
	r := s.Run(Query{Sort: "recent"})
	if len(r.Jobs) == 0 || r.Jobs[0].Title != "Senior Software Engineer" {
		t.Errorf("recent sort first = %v", r.Jobs)
	}
	r = s.Run(Query{Sort: "title"})
	if r.Jobs[0].Title != "Machine Learning Engineer" {
		t.Errorf("title sort first = %q", r.Jobs[0].Title)
	}
	r = s.Run(Query{PageSize: 1, Page: 1})
	if len(r.Jobs) != 1 || r.Total != 3 {
		t.Errorf("paging: got %d jobs total %d", len(r.Jobs), r.Total)
	}
	r2 := s.Run(Query{PageSize: 1, Page: 2})
	if r2.Jobs[0].ID == r.Jobs[0].ID {
		t.Error("page 2 should differ from page 1")
	}
}

func TestQueryPaginationOverflowSafe(t *testing.T) {
	s := testSnapshot()
	// Absurd page values must not panic on the slice bound (regression: int
	// overflow in (page-1)*size producing a negative slice index).
	for _, page := range []int{100000000000000001, 400000000000000001, 1 << 60} {
		r := s.Run(Query{Page: page, PageSize: 100})
		if r.Total != 3 {
			t.Errorf("page=%d: total=%d, want 3", page, r.Total)
		}
		if len(r.Jobs) != 0 {
			t.Errorf("page=%d: expected empty page, got %d jobs", page, len(r.Jobs))
		}
	}
	// A normal huge-but-valid pageSize is clamped, not panicking.
	if r := s.Run(Query{Page: 1, PageSize: 1 << 40}); len(r.Jobs) != 3 {
		t.Errorf("huge pageSize: got %d jobs, want 3", len(r.Jobs))
	}
}

func TestQueryOldestSortUnknownLast(t *testing.T) {
	// A job with a zero PostedAt must sort last even under "oldest".
	now := time.Now()
	results := []store.CompanyResult{{CompanyID: "c", Company: "C", OK: true, Jobs: []model.Job{
		{ID: "c~1", CompanyID: "c", Title: "Known Old", URL: "u1", PostedAt: now.AddDate(0, 0, -30)},
		{ID: "c~2", CompanyID: "c", Title: "Unknown Date", URL: "u2"}, // zero PostedAt
		{ID: "c~3", CompanyID: "c", Title: "Known New", URL: "u3", PostedAt: now.AddDate(0, 0, -1)},
	}}}
	s := buildSnapshot(results, nil)
	r := s.Run(Query{Sort: "oldest"})
	if r.Jobs[0].Title != "Known Old" || r.Jobs[len(r.Jobs)-1].Title != "Unknown Date" {
		t.Errorf("oldest order = %v", []string{r.Jobs[0].Title, r.Jobs[1].Title, r.Jobs[2].Title})
	}
}

func TestQueryFacets(t *testing.T) {
	s := testSnapshot()
	r := s.Run(Query{})
	if len(r.Facets.Companies) != 2 {
		t.Errorf("company facets = %d, want 2", len(r.Facets.Companies))
	}
	if len(r.Facets.Sources) != 2 {
		t.Errorf("source facets = %d, want 2", len(r.Facets.Sources))
	}
}

func TestQueryCountryFilter(t *testing.T) {
	s := testSnapshot()
	if r := s.Run(Query{Country: "United Kingdom"}); r.Total != 0 {
		// testSnapshot has SF/Remote-US/NYC locations only; no UK jobs.
		t.Errorf("UK filter total = %d, want 0", r.Total)
	}
	if r := s.Run(Query{Country: "United States"}); r.Total != 3 {
		t.Errorf("US filter total = %d, want 3", r.Total)
	}
	// All jobs matched → country facets must cover every job.
	all := s.Run(Query{})
	sum := 0
	for _, f := range all.Facets.Countries {
		sum += f.Count
	}
	if sum != all.Total {
		t.Errorf("country facet counts sum %d != total %d", sum, all.Total)
	}
}

func TestQueryRegionFilter(t *testing.T) {
	now := time.Now()
	results := []store.CompanyResult{{CompanyID: "c", Company: "C", OK: true, Jobs: []model.Job{
		{ID: "c~1", CompanyID: "c", Title: "Engineer NA", Location: "Seattle, WA", URL: "u1", PostedAt: now},
		{ID: "c~2", CompanyID: "c", Title: "Engineer EU", Location: "Berlin, Germany", URL: "u2", PostedAt: now},
		{ID: "c~3", CompanyID: "c", Title: "Engineer APAC", Location: "Tokyo, Japan", URL: "u3", PostedAt: now},
		{ID: "c~4", CompanyID: "c", Title: "Engineer Nowhere", Location: "Remote", Remote: true, URL: "u4", PostedAt: now},
	}}}
	s := buildSnapshot(results, nil)

	for region, want := range map[string]int{
		model.RegionEurope: 1, model.RegionNorthAmerica: 1, model.RegionAsia: 1,
		"Remote": 1, model.RegionAfrica: 0,
	} {
		if r := s.Run(Query{Region: region}); r.Total != want {
			t.Errorf("region %q total = %d, want %d", region, r.Total, want)
		}
	}
	// Region facets must account for every matched job, unknown ones included.
	all := s.Run(Query{})
	sum := 0
	for _, f := range all.Facets.Regions {
		sum += f.Count
	}
	if sum != all.Total {
		t.Errorf("region facet counts sum %d != total %d", sum, all.Total)
	}
}
