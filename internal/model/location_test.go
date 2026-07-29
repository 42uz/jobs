package model

import "testing"

func TestCountryOfGlobal(t *testing.T) {
	cases := map[string]string{
		// Europe
		"London, UK":          "United Kingdom",
		"Berlin, Germany":     "Germany",
		"Munich":              "Germany",
		"Berlin, DE":          "Germany",
		"Kraków, PL":          "Poland",
		"Greater London Area": "United Kingdom",
		"Northern Ireland":    "United Kingdom",
		"Zürich, Switzerland": "Switzerland",
		"Denmark, Roskilde":   "Denmark",
		"Anywhere in France":  "France",
		"Gent, Belgium":       "Belgium",
		"Istanbul, Türkiye":   "Turkey",
		// North America
		"San Francisco, CA":                  "United States",
		"New York, NY":                       "United States",
		"Seattle, WA, US":                    "United States",
		"Remote - US":                        "United States",
		"Remote US (PT)":                     "United States",
		"Paris, TX":                          "United States",
		"Cambridge, MA":                      "United States",
		"Berlin, CT":                         "United States",
		"Dublin, OH":                         "United States",
		"Wilmington, DE":                     "United States",
		"Austin, Texas":                      "United States",
		"Atlanta, Georgia":                   "United States",
		"Washington, DC":                     "United States",
		"San Francisco, CA & Remote CA (PT)": "United States",
		"Toronto, Canada":                    "Canada",
		"London, ON":                         "Canada",
		"Vancouver":                          "Canada",
		// Rest of the world
		"Bangalore, IN":              "India",
		"Bengaluru, KA, IN":          "India",
		"Coimbatore, TN, IN":         "India",
		"Nashville, TN":              "United States",
		"Tunis, TN":                  "Tunisia",
		"India - Bangalore":          "India",
		"Tokyo, Japan":               "Japan",
		"Singapore":                  "Singapore",
		"Dubai, UAE":                 "United Arab Emirates",
		"Tel Aviv":                   "Israel",
		"São Paulo, Brazil":          "Brazil",
		"Argentina - Buenos Aires":   "Argentina",
		"Mexico City":                "Mexico",
		"Sydney, NSW":                "Australia",
		"New South Wales, Australia": "Australia",
		"Cape Town, South Africa":    "South Africa",
		"Tbilisi, Georgia":           "Georgia",
		"Hiram, Georgia":             "United States",
		// No country
		"Remote - Europe": "",
		"EMEA":            "",
		"Remote":          "",
		"5 Locations":     "",
		"":                "",
	}
	for loc, want := range cases {
		if got := CountryOf(loc); got != want {
			t.Errorf("CountryOf(%q) = %q, want %q", loc, got, want)
		}
	}
	// Multi-location: the first location that resolves wins.
	if got := CountryOf("Remote", "Lisbon, Portugal"); got != "Portugal" {
		t.Errorf("multi = %q", got)
	}
}

func TestRegionOf(t *testing.T) {
	cases := map[string]string{
		"Berlin, Germany":   RegionEurope,
		"Remote - Europe":   RegionEurope,
		"EMEA":              RegionEurope,
		"Remote (EU)":       RegionEurope,
		"Istanbul, Turkey":  RegionEurope,
		"Seattle, WA":       RegionNorthAmerica,
		"Toronto, Canada":   RegionNorthAmerica,
		"Remote - LATAM":    RegionLatinAmerica,
		"São Paulo, Brazil": RegionLatinAmerica,
		"Mexico City":       RegionLatinAmerica,
		"Bengaluru, India":  RegionAsia,
		"Remote - APAC":     RegionAsia,
		"Singapore":         RegionAsia,
		"Tel Aviv, Israel":  RegionMiddleEast,
		"Dubai, UAE":        RegionMiddleEast,
		"Nairobi, Kenya":    RegionAfrica,
		"Remote - Africa":   RegionAfrica,
		"Sydney, Australia": RegionOceania,
		"Auckland":          RegionOceania,
		"Remote":            "",
		"5 Locations":       "",
	}
	for loc, want := range cases {
		if got := RegionOf(loc); got != want {
			t.Errorf("RegionOf(%q) = %q, want %q", loc, got, want)
		}
	}
}

func TestEuropeLocation(t *testing.T) {
	europe := []string{
		"London, UK", "Berlin, Germany", "Berlin", "Munich", "Paris, France",
		"Amsterdam, Netherlands", "Dublin, Ireland", "Madrid, Spain", "Remote - Europe",
		"Remote (EU)", "Zurich, Switzerland", "Warsaw, Poland", "Stockholm", "Lisbon, Portugal",
		"Cambridge, UK", "Milan, Italy", "EMEA", "Greater London Area", "Kraków, PL",
		"Berlin, DE", "Munich, DE",
	}
	for _, loc := range europe {
		if !EuropeLocation(loc) {
			t.Errorf("expected Europe: %q", loc)
		}
	}
	notEurope := []string{
		"San Francisco, CA", "New York, NY", "Seattle, WA, US", "Remote - US",
		"Paris, TX", "Cambridge, MA", "Berlin, CT", "Dublin, OH", "Austin, TX",
		"Bangalore, IN", "Toronto, Canada", "Tokyo, Japan", "", "5 Locations",
		"Wilmington, DE", "Argentina - Buenos Aires", "New South Wales, Australia",
		"São Paulo, Brazil", "Singapore", "India - Bangalore", "Dubai, UAE",
		"San Francisco, CA & Remote CA (PT)", "Remote US (PT)",
	}
	for _, loc := range notEurope {
		if EuropeLocation(loc) {
			t.Errorf("did NOT expect Europe: %q", loc)
		}
	}
	if !EuropeLocation("Remote", "Munich, Germany") {
		t.Error("any European location among several should match")
	}
}

func TestLocationFilter(t *testing.T) {
	// Global specs keep everything (nil filter).
	for _, spec := range []string{"", "global", "all", "world", "everywhere"} {
		f, ok := LocationFilter(spec)
		if !ok || f != nil {
			t.Errorf("LocationFilter(%q): filter=%v ok=%v; want nil filter, ok", spec, f != nil, ok)
		}
	}
	if _, ok := LocationFilter("atlantis"); ok {
		t.Error("unknown spec should be rejected")
	}

	// Region spec: a multi-office role gets its in-region office promoted.
	eu, ok := LocationFilter("europe")
	if !ok || eu == nil {
		t.Fatal("europe spec should build a filter")
	}
	j, keep := eu(Job{Location: "San Francisco", Locations: []string{"San Francisco", "London"}})
	if !keep || j.Location != "London" {
		t.Errorf("expected keep with promoted London, got keep=%v loc=%q", keep, j.Location)
	}
	if _, keep = eu(Job{Location: "Singapore", Relocation: true}); keep {
		t.Error("out-of-region relocation job should be dropped")
	}
	if j, keep = eu(Job{Location: "Berlin, Germany"}); !keep || j.Location != "Berlin, Germany" {
		t.Errorf("in-region job should be kept unchanged, got keep=%v loc=%q", keep, j.Location)
	}

	// Alias and country specs.
	apac, ok := LocationFilter("apac")
	if !ok {
		t.Fatal("apac alias should resolve")
	}
	if _, keep := apac(Job{Location: "Tokyo, Japan"}); !keep {
		t.Error("Tokyo should pass the APAC filter")
	}
	de, ok := LocationFilter("germany")
	if !ok {
		t.Fatal("country spec should resolve")
	}
	if _, keep := de(Job{Location: "Munich"}); !keep {
		t.Error("Munich should pass the Germany filter")
	}
	if _, keep := de(Job{Location: "Paris, France"}); keep {
		t.Error("Paris should not pass the Germany filter")
	}

	// State / province specs.
	ca, ok := LocationFilter("California")
	if !ok {
		t.Fatal("state spec should resolve")
	}
	for loc, want := range map[string]bool{
		"Los Gatos, California, United States of America": true,
		"Palo Alto":         true,
		"San Francisco, CA": true,
		"Seattle, WA":       false,
		"Toronto, ON":       false,
		"Munich":            false,
	} {
		if _, keep := ca(Job{Location: loc}); keep != want {
			t.Errorf("California filter kept %q = %v, want %v", loc, keep, want)
		}
	}
	on, ok := LocationFilter("ontario")
	if !ok {
		t.Fatal("province spec should resolve")
	}
	if _, keep := on(Job{Location: "Toronto, Canada"}); !keep {
		t.Error("Toronto should pass the Ontario filter")
	}
	// State codes are not accepted: "CA"/"IN" are a coin flip against
	// Canada/India, so they must not silently pick one.
	for _, spec := range []string{"ca", "ny", "on"} {
		if f, ok := LocationFilter(spec); ok && f != nil {
			if _, keep := f(Job{Location: "New York, NY"}); keep && spec != "ny" {
				t.Errorf("spec %q resolved to something unexpected", spec)
			}
		}
	}
	// Broader scopes win a shared name: "georgia" is the country.
	ge, ok := LocationFilter("georgia")
	if !ok {
		t.Fatal("georgia should resolve")
	}
	if _, keep := ge(Job{Location: "Tbilisi, Georgia"}); !keep {
		t.Error("Tbilisi should pass the Georgia (country) filter")
	}
	if _, keep := ge(Job{Location: "Atlanta, Georgia"}); keep {
		t.Error("Atlanta should not pass the Georgia (country) filter")
	}
}

func TestMentionsRelocation(t *testing.T) {
	yes := []string{
		"We offer relocation assistance to Berlin.",
		"Relocation package available.",
		"Willing to relocate the right candidate.",
	}
	for _, s := range yes {
		if !MentionsRelocation(s) {
			t.Errorf("expected relocation: %q", s)
		}
	}
	no := []string{
		"Great engineering role.",
		"No relocation is offered for this position.",
		"We do not offer relocation assistance.",
	}
	for _, s := range no {
		if MentionsRelocation(s) {
			t.Errorf("did NOT expect relocation: %q", s)
		}
	}
}

func TestResolveLocationsAgree(t *testing.T) {
	// A country-bearing location wins over an earlier region-only one, and the
	// region and state reported are that country's — never a mix of the two.
	if c, r, st := ResolveLocations("Remote - EMEA", "New York, NY"); c != "United States" || r != RegionNorthAmerica || st != "New York" {
		t.Errorf("ResolveLocations(EMEA, NY) = %q/%q/%q, want United States/North America/New York", c, r, st)
	}
	if c, r, st := ResolveLocations("Remote - EMEA"); c != "" || r != RegionEurope || st != "" {
		t.Errorf("region-only = %q/%q/%q, want \"\"/Europe/\"\"", c, r, st)
	}
	if c, r, st := ResolveLocations("Remote", "Munich"); c != "Germany" || r != RegionEurope || st != "" {
		t.Errorf("ResolveLocations(Remote, Munich) = %q/%q/%q, want Germany/Europe/\"\"", c, r, st)
	}
	if c, r, st := ResolveLocations("Remote"); c != "" || r != "" || st != "" {
		t.Errorf("unresolvable = %q/%q/%q, want empty", c, r, st)
	}
}

func TestStateOf(t *testing.T) {
	cases := map[string]string{
		// Codes are unambiguous once the country is known.
		"San Francisco, CA":           "California",
		"Cambridge, MA":               "Massachusetts",
		"Seattle, WA, US":             "Washington",
		"Wilmington, DE":              "Delaware",
		"Indianapolis, IN":            "Indiana",
		"Washington, DC":              "District of Columbia",
		"Austin, Texas":               "Texas",
		"Atlanta, Georgia":            "Georgia",
		"New York, NY, United States": "New York",
		// Several offices listed: the leading one is the primary.
		"San Francisco, CA | New York, NY": "California",
		// Bare cities fall back to the city → state table.
		"Palo Alto": "California",
		"Boston":    "Massachusetts",
		"Nashville": "Tennessee",
		// Canada resolves to provinces.
		"Toronto, ON":      "Ontario",
		"Vancouver":        "British Columbia",
		"Montreal, Quebec": "Quebec",
		"London, ON":       "Ontario",
		// Everywhere else has no state.
		"Berlin, Germany":   "",
		"Bengaluru, KA, IN": "",
		"Remote - EMEA":     "",
		"Paris, France":     "",
		"":                  "",
	}
	for loc, want := range cases {
		if got := StateOf(loc); got != want {
			t.Errorf("StateOf(%q) = %q, want %q", loc, got, want)
		}
	}
}
