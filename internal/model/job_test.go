package model

import "testing"

func TestCategorize(t *testing.T) {
	cases := []struct {
		title, dept string
		want        string
	}{
		{"Senior Software Engineer, Backend", "Engineering", "Software Engineering"},
		{"Machine Learning Engineer", "AI", "Data & ML"},
		{"Product Designer", "Design", "Design"},
		{"Account Executive", "Sales", "Sales & Business"},
		{"Technical Recruiter", "People", "People & Recruiting"},
		{"Security Engineer", "", "Security"},
		{"Underwater Basket Weaver", "", "Other"},
	}
	for _, c := range cases {
		got := Categorize(c.title, c.dept)
		if len(got) == 0 || got[0] != c.want {
			t.Errorf("Categorize(%q,%q) = %v, want first %q", c.title, c.dept, got, c.want)
		}
	}
}

func TestDetectRemote(t *testing.T) {
	if !DetectRemote("San Francisco, CA", "Remote - US") {
		t.Error("expected remote detection")
	}
	if DetectRemote("New York, NY") {
		t.Error("did not expect remote detection")
	}
}

func TestStableID(t *testing.T) {
	a := StableID("greenhouse-stripe", "123")
	b := StableID("greenhouse-stripe", "123")
	c := StableID("greenhouse-stripe", "124")
	if a != b {
		t.Error("StableID must be deterministic")
	}
	if a == c {
		t.Error("StableID must differ for different inputs")
	}
	if a == "" {
		t.Error("StableID must be non-empty")
	}
}

func TestCleanText(t *testing.T) {
	if got := CleanText("  hello \n  world\t "); got != "hello world" {
		t.Errorf("CleanText = %q", got)
	}
}

func TestSlim(t *testing.T) {
	j := Job{Title: "x", Description: "big html"}
	if j.Slim().Description != "" {
		t.Error("Slim must drop description")
	}
	if j.Description == "" {
		t.Error("Slim must not mutate the receiver")
	}
}
