package source

import (
	"html"
	"regexp"
	"strings"
	"time"
)

// timeLayouts are the formats we attempt, in order, when parsing timestamps
// from heterogeneous ATS payloads.
var timeLayouts = []string{
	time.RFC3339Nano,
	time.RFC3339,
	"2006-01-02T15:04:05Z0700",
	"2006-01-02T15:04:05.000Z0700",
	"2006-01-02T15:04:05",
	"2006-01-02 15:04:05",
	"2006-01-02",
	"January 2, 2006", // Amazon: "July 20, 2026"
	"Jan 2, 2006",
	"2006-01-02T15:04:05.999999",
	time.RFC1123Z,
	time.RFC1123,
}

// parseTime best-effort parses a timestamp string, returning the zero time when
// it cannot be understood (callers treat zero as "unknown").
func parseTime(s string) time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}
	}
	for _, l := range timeLayouts {
		if t, err := time.Parse(l, s); err == nil {
			return t.UTC()
		}
	}
	return time.Time{}
}

// parseEpoch interprets a numeric timestamp that may be in seconds or
// milliseconds since the Unix epoch.
func parseEpoch(n int64) time.Time {
	if n <= 0 {
		return time.Time{}
	}
	if n > 1e12 { // milliseconds
		return time.UnixMilli(n).UTC()
	}
	return time.Unix(n, 0).UTC()
}

var (
	scriptTag = regexp.MustCompile(`(?is)<script.*?</script>`)
	styleTag  = regexp.MustCompile(`(?is)<style.*?</style>`)
	onAttr    = regexp.MustCompile(`(?is)\son\w+\s*=\s*("[^"]*"|'[^']*'|[^\s>]+)`)
	jsHref    = regexp.MustCompile(`(?is)(href|src)\s*=\s*("javascript:[^"]*"|'javascript:[^']*')`)
)

// sanitizeHTML removes the most dangerous constructs from source-provided HTML
// descriptions (scripts, styles, inline event handlers, javascript: URIs). It
// is a defense-in-depth measure; the frontend performs its own sanitization
// too.
func sanitizeHTML(s string) string {
	if s == "" {
		return s
	}
	s = scriptTag.ReplaceAllString(s, "")
	s = styleTag.ReplaceAllString(s, "")
	s = onAttr.ReplaceAllString(s, "")
	s = jsHref.ReplaceAllString(s, `$1=""`)
	return strings.TrimSpace(s)
}

// unescapeAndClean turns entity-encoded HTML into plain HTML and sanitizes it.
func unescapeAndClean(s string) string {
	if s == "" {
		return s
	}
	// Some ATSes double-encode ("&lt;h2&gt;"). Unescape once; if it still looks
	// encoded, unescape again.
	out := html.UnescapeString(s)
	if strings.Contains(out, "&lt;") || strings.Contains(out, "&gt;") {
		out = html.UnescapeString(out)
	}
	return sanitizeHTML(out)
}

// firstNonEmpty returns the first non-empty, trimmed string.
func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if t := strings.TrimSpace(v); t != "" {
			return t
		}
	}
	return ""
}
