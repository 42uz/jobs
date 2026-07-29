package source

import (
	"testing"

	"faangjobs/internal/registry"
)

// A miniature of Apple's escaped-JSON-in-HTML embedding: two job objects, one
// with an escaped quote inside the summary text.
const appleFixture = `<html><script>x="{\"searchResults\":[` +
	`{\"jobSummary\":\"Build \\\"great\\\" things.\",\"locations\":[{\"name\":\"London\",\"city\":\"London\",\"countryName\":\"United Kingdom\"}],\"positionId\":\"111\",\"postingTitle\":\"Engineer One\",\"transformedPostingTitle\":\"engineer-one\",\"postDateInGMT\":\"2026-07-01T00:00:00.000Z\",\"team\":{\"teamName\":\"Software\"},\"homeOffice\":false},` +
	`{\"jobSummary\":\"Second role.\",\"locations\":[{\"name\":\"Munich\",\"city\":\"Munich\",\"countryName\":\"Germany\"}],\"positionId\":\"222\",\"postingTitle\":\"Engineer Two\",\"transformedPostingTitle\":\"engineer-two\",\"postDateInGMT\":\"2026-07-02T00:00:00.000Z\",\"team\":{\"teamName\":\"Hardware\"},\"homeOffice\":true}` +
	`]}"</script></html>`

func TestParseAppleJobs(t *testing.T) {
	c := registry.Company{Name: "Apple", ATS: "applecareers", Slug: "apple"}
	c.EnsureID()
	jobs := parseAppleJobs(unescapeOneLevel(appleFixture), c)
	if len(jobs) != 2 {
		t.Fatalf("parsed %d jobs, want 2", len(jobs))
	}
	j := jobs[0]
	if j.Title != "Engineer One" || j.Location != "London, London" && j.Location != "London" {
		t.Errorf("job0 = %q @ %q", j.Title, j.Location)
	}
	if j.URL != "https://jobs.apple.com/en-us/details/111/engineer-one" {
		t.Errorf("job0 url = %q", j.URL)
	}
	if j.PostedAt.IsZero() {
		t.Error("job0 date not parsed")
	}
	if jobs[1].Department != "Hardware" || !jobs[1].Remote {
		t.Errorf("job1 dept/remote = %q %v", jobs[1].Department, jobs[1].Remote)
	}
}

func TestParseAppleTotal(t *testing.T) {
	// The count arrives escaped inside the page and unescaped before parsing.
	text := unescapeOneLevel(`<div>Showing 1-20 of \"totalRecords\":6022 results</div>`)
	if got := parseAppleTotal(text); got != 6022 {
		t.Errorf("parseAppleTotal = %d, want 6022", got)
	}
	if got := parseAppleTotal("no count here"); got != 0 {
		t.Errorf("parseAppleTotal(missing) = %d, want 0", got)
	}
}
