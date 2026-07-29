package registry

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureID(t *testing.T) {
	c := Company{ATS: "Greenhouse", Slug: "Stripe"}
	c.EnsureID()
	if c.ID != "greenhouse-stripe" {
		t.Errorf("EnsureID = %q", c.ID)
	}
}

func TestParseAndDedupe(t *testing.T) {
	data := []byte(`{"companies":[
		{"name":"Stripe","ats":"greenhouse","slug":"stripe"},
		{"name":"Stripe Dup","ats":"greenhouse","slug":"stripe"},
		{"name":"Bad","ats":"","slug":""},
		{"name":"Ramp","ats":"ashby","slug":"ramp"}
	]}`)
	companies, err := parse(data)
	if err != nil {
		t.Fatal(err)
	}
	cat, err := newCatalog(companies)
	if err != nil {
		t.Fatal(err)
	}
	if len(cat.Companies) != 2 {
		t.Errorf("expected 2 after dedupe/validate, got %d", len(cat.Companies))
	}
}

func TestParseArrayForm(t *testing.T) {
	data := []byte(`[{"name":"Ramp","ats":"ashby","slug":"ramp"}]`)
	companies, err := parse(data)
	if err != nil || len(companies) != 1 {
		t.Fatalf("array parse: %v %d", err, len(companies))
	}
}

func TestEmbeddedCatalogLoads(t *testing.T) {
	cat, err := Load("")
	if err != nil {
		t.Fatalf("embedded catalog failed to load: %v", err)
	}
	if len(cat.Companies) == 0 {
		t.Error("embedded catalog is empty")
	}
	for _, c := range cat.Companies {
		if c.ID == "" || c.ATS == "" {
			t.Errorf("invalid embedded company: %+v", c)
		}
	}
}

func TestLoadOverride(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "reg.json")
	_ = os.WriteFile(p, []byte(`[{"name":"Test","ats":"lever","slug":"test"}]`), 0o644)
	cat, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(cat.Companies) != 1 || cat.Companies[0].ATS != "lever" {
		t.Errorf("override load = %+v", cat.Companies)
	}
}

func TestDisabledFiltered(t *testing.T) {
	companies := []Company{
		{Name: "A", ATS: "lever", Slug: "a"},
		{Name: "B", ATS: "lever", Slug: "b", Disabled: true},
	}
	cat, _ := newCatalog(companies)
	if len(cat.Enabled()) != 1 {
		t.Errorf("Enabled should filter disabled, got %d", len(cat.Enabled()))
	}
}
