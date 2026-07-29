// Package registry holds the catalog of companies to crawl. The default
// catalog is embedded in the binary (companies.json) so the crawler works with
// zero configuration, but an operator can supply an override file to add,
// remove, or replace entries at runtime.
package registry

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
)

//go:embed companies.json
var embedded []byte

// Company describes one crawl target: which ATS/source hosts its board and the
// slug/token used to address it. Config carries extra per-source parameters
// (used by the generic/custom adapters).
type Company struct {
	ID     string            `json:"id"`
	Name   string            `json:"name"`
	ATS    string            `json:"ats"`
	Slug   string            `json:"slug"`
	Config map[string]string `json:"config,omitempty"`
	// Disabled lets an entry stay in the catalog while being skipped.
	Disabled bool `json:"disabled,omitempty"`
}

var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

// EnsureID fills a deterministic ID when one is not provided. The id is stable
// as long as ats+slug are stable, and is safe to use as a filename.
func (c *Company) EnsureID() {
	if c.ID != "" {
		return
	}
	base := strings.ToLower(c.ATS + "-" + c.Slug)
	c.ID = strings.Trim(nonSlug.ReplaceAllString(base, "-"), "-")
}

// Catalog is a validated, de-duplicated list of companies.
type Catalog struct {
	Companies []Company
}

// Load returns the catalog. If overridePath is non-empty and readable, its
// contents replace the embedded catalog; otherwise the embedded catalog is
// used. Both formats accept either a bare JSON array of companies or an object
// with a top-level "companies" array.
func Load(overridePath string) (*Catalog, error) {
	data := embedded
	if overridePath != "" {
		b, err := os.ReadFile(overridePath)
		if err != nil {
			return nil, fmt.Errorf("read registry override %q: %w", overridePath, err)
		}
		data = b
	}
	companies, err := parse(data)
	if err != nil {
		return nil, err
	}
	return newCatalog(companies)
}

func parse(data []byte) ([]Company, error) {
	trimmed := strings.TrimSpace(string(data))
	var companies []Company
	if strings.HasPrefix(trimmed, "{") {
		var wrapper struct {
			Companies []Company `json:"companies"`
		}
		if err := json.Unmarshal(data, &wrapper); err != nil {
			return nil, fmt.Errorf("parse registry object: %w", err)
		}
		companies = wrapper.Companies
	} else {
		if err := json.Unmarshal(data, &companies); err != nil {
			return nil, fmt.Errorf("parse registry array: %w", err)
		}
	}
	return companies, nil
}

func newCatalog(companies []Company) (*Catalog, error) {
	seen := map[string]bool{}
	out := make([]Company, 0, len(companies))
	for i := range companies {
		c := companies[i]
		c.Name = strings.TrimSpace(c.Name)
		c.ATS = strings.ToLower(strings.TrimSpace(c.ATS))
		c.Slug = strings.TrimSpace(c.Slug)
		if c.ATS == "" || (c.Slug == "" && c.Config == nil) {
			// Skip malformed entries rather than failing the whole run.
			continue
		}
		if c.Name == "" {
			c.Name = c.Slug
		}
		c.EnsureID()
		if seen[c.ID] {
			continue
		}
		seen[c.ID] = true
		out = append(out, c)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("registry is empty after validation")
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return &Catalog{Companies: out}, nil
}

// Enabled returns the subset of companies that are not disabled.
func (c *Catalog) Enabled() []Company {
	out := make([]Company, 0, len(c.Companies))
	for _, comp := range c.Companies {
		if !comp.Disabled {
			out = append(out, comp)
		}
	}
	return out
}
