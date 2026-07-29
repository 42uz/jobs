package model

import (
	"regexp"
	"strings"
)

// This file resolves free-form location strings ("Bengaluru, KA, IN",
// "Remote - EMEA", "Paris, TX") to a canonical country and region using the
// tables in geo.go. It is the single source of truth for geography questions
// asked by the crawler (optional -region filter) and the server (facets).
//
// The matcher is precision-oriented. Two families of collisions matter:
//
//   - cities that exist in several countries (Cambridge, Birmingham, Paris,
//     Vancouver, Santiago): resolved by the state/province marker that almost
//     always accompanies the non-dominant one ("Paris, TX" → United States);
//   - two-letter codes that are both a US state and a country ("IN" Indiana vs
//     India, "CA" California vs Canada, "DE" Delaware vs Germany): resolved in
//     favor of the country only when another part of the same string points at
//     that country ("Bengaluru, KA, IN" → India), and to the US otherwise.
//
// Resolution order: country name → state/province name → codes (last segment
// first, since that is the country slot) → city → region-level token.

var locSep = regexp.MustCompile(`[,/|()&·•]+|\s[-–—]\s`)
var wordSep = regexp.MustCompile(`[\s,/|()\-–—·•&]+`)

// maxPhraseWords is the longest multi-word key in any lookup table
// ("united states of america", "australian capital territory").
const maxPhraseWords = 4

// splitLoc breaks a location into its comma/slash/paren-separated segments.
func splitLoc(l string) []string {
	parts := locSep.Split(l, -1)
	out := parts[:0]
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// tokenize splits a location into lowercase words.
func tokenizeLoc(l string) []string {
	fields := wordSep.Split(l, -1)
	out := fields[:0]
	for _, f := range fields {
		if f != "" {
			out = append(out, f)
		}
	}
	return out
}

// normPhrase renders a phrase in the same word-joined form used as lookup keys.
func normPhrase(s string) string {
	return strings.Join(tokenizeLoc(strings.ToLower(strings.TrimSpace(s))), " ")
}

// lookupPhrase finds the longest word-sequence of tokens present in m, scanning
// each length left to right.
func lookupPhrase(tokens []string, m map[string]string) (string, bool) {
	return lookupPhraseDir(tokens, m, false)
}

// lookupPhraseLast is lookupPhrase but prefers the rightmost match of a given
// length. Country names sit in the trailing slot of a location string, so
// "New South Wales, Australia" must resolve to Australia, not to Wales.
func lookupPhraseLast(tokens []string, m map[string]string) (string, bool) {
	return lookupPhraseDir(tokens, m, true)
}

func lookupPhraseDir(tokens []string, m map[string]string, fromEnd bool) (string, bool) {
	for n := maxPhraseWords; n >= 1; n-- {
		if n > len(tokens) {
			continue
		}
		last := len(tokens) - n
		for k := 0; k <= last; k++ {
			i := k
			if fromEnd {
				i = last - k
			}
			if v, ok := m[strings.Join(tokens[i:i+n], " ")]; ok {
				return v, true
			}
		}
	}
	return "", false
}

// CountryOf maps location strings to a canonical country name, returning the
// first one that resolves. It returns "" for locations that name no country
// (e.g. "Remote", "EMEA", "5 Locations").
func CountryOf(locs ...string) string {
	for _, loc := range locs {
		if c, _ := resolveLocation(loc); c != "" {
			return c
		}
	}
	return ""
}

// RegionOf maps location strings to a canonical region (see model.Regions),
// resolving region-level strings ("Remote - APAC", "EMEA") directly. It returns
// "" when nothing geographic is recognized.
func RegionOf(locs ...string) string {
	for _, loc := range locs {
		if _, r := resolveLocation(loc); r != "" {
			return r
		}
	}
	return ""
}

// ResolveLocations maps a job's locations to one country, region and state that
// agree with each other: the first location naming a country wins, and only when
// no location names one does a region-level string ("Remote - EMEA") supply the
// region on its own. Callers that need several of these values must use this
// rather than CountryOf + RegionOf + StateOf, which resolve independently and
// could pair the region of one location with the country of another.
func ResolveLocations(locs ...string) (country, region, state string) {
	for _, loc := range locs {
		c, r, s := resolveLocationFull(loc)
		if c != "" {
			return c, r, s
		}
		if region == "" {
			region = r
		}
	}
	return "", region, ""
}

// InRegion reports whether any of the locations resolves to the given region.
func InRegion(region string, locs ...string) bool {
	for _, loc := range locs {
		if _, r := resolveLocation(loc); r == region {
			return true
		}
	}
	return false
}

// resolveLocation is the core matcher: it returns the country (may be "") and
// the region (may be "") for one location string.
func resolveLocation(loc string) (string, string) {
	c, r, _ := resolveLocationFull(loc)
	return c, r
}

// resolveLocationFull also reports the state/province, for the countries the
// board tracks at that level (see states.go).
func resolveLocationFull(loc string) (country, region, state string) {
	l := strings.ToLower(strings.TrimSpace(loc))
	if l == "" {
		return "", "", ""
	}
	tokens := tokenizeLoc(l)
	segs := splitLoc(l)

	country = countryByName(tokens)
	if country == "" {
		country = countryBySubdivisionName(segs)
	}
	if country == "" {
		country = countryByCode(segs, tokens)
	}
	if country == "" {
		if c, ok := lookupPhrase(tokens, cityCountry); ok {
			country = c
		}
	}
	if country == "" {
		if r, ok := lookupPhrase(tokens, regionTokens); ok {
			return "", r, ""
		}
		return "", "", ""
	}
	return country, countryRegion[country], stateIn(country, segs, tokens)
}

// stateIn resolves the state/province within an already-identified country.
// Codes are read from whole segments (the state slot), then spelled-out names,
// then the city. Because the country is known, codes that are ambiguous at the
// country level ("CA", "IN", "DE") are unambiguous here.
//
// Unlike the country scan this reads left to right: the country sits in the
// trailing slot of a location string, but where several are listed the leading
// one is the primary office ("San Francisco, CA | New York, NY" → California).
func stateIn(country string, segs, tokens []string) string {
	codes, names, cities := statesOf(country)
	if codes == nil {
		return ""
	}
	for _, seg := range segs {
		if s, ok := codes[seg]; ok {
			return s
		}
	}
	if s, ok := lookupPhrase(tokens, names); ok {
		return s
	}
	if s, ok := lookupPhrase(tokens, cities); ok {
		return s
	}
	return ""
}

// StateOf maps location strings to a US state or Canadian province, returning
// the first one that resolves and "" for everywhere else.
func StateOf(locs ...string) string {
	for _, loc := range locs {
		if _, _, s := resolveLocationFull(loc); s != "" {
			return s
		}
	}
	return ""
}

// countryByName matches a spelled-out country name. "Georgia" is ambiguous (the
// country vs the US state); job locations mean the state far more often, so it
// resolves to the United States unless the string names a city in the country
// ("Tbilisi, Georgia").
func countryByName(tokens []string) string {
	c, ok := lookupPhraseLast(tokens, countryVariants)
	if !ok {
		return ""
	}
	if c == "Georgia" {
		if city, ok := lookupPhrase(tokens, cityCountry); ok && city == "Georgia" {
			return "Georgia"
		}
		return "United States"
	}
	return c
}

// countryBySubdivisionName matches a spelled-out state/province/region segment
// ("Austin, Texas", "London, Ontario", "Bengaluru, Karnataka"). Segments are
// scanned from the end because the country-ish slot comes last.
func countryBySubdivisionName(segs []string) string {
	for i := len(segs) - 1; i >= 0; i-- {
		if c, ok := subdivisionNames[segs[i]]; ok {
			return c
		}
	}
	return ""
}

// countryByCode matches ISO country codes and state/province codes that stand
// alone as a whole segment, scanning from the last segment (the country slot)
// backwards.
func countryByCode(segs []string, tokens []string) string {
	for i := len(segs) - 1; i >= 0; i-- {
		s := segs[i]
		if c, ok := codeCountry[s]; ok {
			return c
		}
		if c, ok := ambiguousCodes[s]; ok {
			// Both a country code and a US state code: take the country only
			// when another part of the string agrees with it.
			if city, ok := lookupPhrase(tokens, cityCountry); ok && city == c {
				return c
			}
			if fallback, ok := subdivisionCodes[s]; ok {
				return fallback
			}
			return "United States"
		}
		if c, ok := subdivisionCodes[s]; ok {
			return c
		}
	}
	return ""
}

// LocationFilter builds the crawler's location policy from a CLI spec. The spec
// may be a region ("europe", "apac", "north america"), a country ("germany",
// "united states"), a US state or Canadian province ("california", "ontario"),
// or a global/all value. It returns a nil filter for "keep everything" and
// reports whether the spec was understood.
//
// Broader scopes win where a name is shared: "georgia" is the country, not the
// US state (scope down with "united states" instead).
func LocationFilter(spec string) (func(Job) (Job, bool), bool) {
	s := normPhrase(spec)
	switch s {
	case "", "all", "any", "global", "world", "worldwide", "everywhere":
		return nil, true
	}
	if r, ok := regionSpec(s); ok {
		return locationJob(func(loc string) bool { return RegionOf(loc) == r }), true
	}
	if c, ok := countryVariants[s]; ok {
		return locationJob(func(loc string) bool { return CountryOf(loc) == c }), true
	}
	if st, ok := stateSpec(s); ok {
		return locationJob(func(loc string) bool { return StateOf(loc) == st }), true
	}
	return nil, false
}

// regionSpec resolves a region name or its common alias (emea, apac, latam …).
func regionSpec(s string) (string, bool) {
	for _, r := range Regions {
		if normPhrase(r) == s {
			return r, true
		}
	}
	if r, ok := regionTokens[s]; ok {
		return r, true
	}
	return "", false
}

// locationJob turns a per-location predicate into a job transform/keep filter:
// a job is kept when its primary location matches, or when one of its secondary
// locations does — in which case that location is promoted for display, so the
// board consistently shows an in-scope office for multi-office roles.
func locationJob(match func(string) bool) func(Job) (Job, bool) {
	return func(j Job) (Job, bool) {
		if match(j.Location) {
			return j, true
		}
		for _, loc := range j.Locations {
			if match(loc) {
				j.Location = loc
				return j, true
			}
		}
		return j, false
	}
}

// EuropeLocation reports whether any of the given locations is in Europe.
func EuropeLocation(locs ...string) bool { return InRegion(RegionEurope, locs...) }

// EuropeJob applies the Europe policy to a job (kept for callers and tests that
// want the region the board started with).
func EuropeJob(j Job) (Job, bool) {
	return locationJob(func(loc string) bool { return RegionOf(loc) == RegionEurope })(j)
}

var relocNegations = []string{
	"no relocation", "not offer relocation", "cannot offer relocation",
	"do not offer relocation", "does not offer relocation", "relocation is not",
	"relocation not available", "without relocation", "not provide relocation",
	"no relocation assistance", "not eligible for relocation", "unable to offer relocation",
	"relocation will not", "relocation is unavailable", "relocation not offered",
}

// MentionsRelocation reports whether any of the given texts positively indicate
// that relocation is offered/supported.
func MentionsRelocation(texts ...string) bool {
	for _, t := range texts {
		lt := strings.ToLower(t)
		if !strings.Contains(lt, "relocat") {
			continue
		}
		negated := false
		for _, n := range relocNegations {
			if strings.Contains(lt, n) {
				negated = true
				break
			}
		}
		if !negated {
			return true
		}
	}
	return false
}
