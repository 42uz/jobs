package store

import (
	"os"
	"testing"
	"time"

	"faangjobs/internal/model"
)

func TestWriteReadCompany(t *testing.T) {
	dir := t.TempDir()
	s, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	res := CompanyResult{
		CompanyID: "greenhouse-stripe", Company: "Stripe", ATS: "greenhouse", OK: true,
		FetchedAt: time.Now().UTC(),
		Jobs:      []model.Job{{ID: "greenhouse-stripe~1", Title: "SWE", URL: "u"}},
	}
	if err := s.WriteCompany(res); err != nil {
		t.Fatal(err)
	}
	got, err := s.ReadCompany("greenhouse-stripe")
	if err != nil {
		t.Fatal(err)
	}
	if got.JobCount != 1 || got.Jobs[0].Title != "SWE" {
		t.Errorf("roundtrip mismatch: %+v", got)
	}
}

func TestLoadAllAndStatus(t *testing.T) {
	dir := t.TempDir()
	s, _ := New(dir)
	_ = s.WriteCompany(CompanyResult{CompanyID: "a", Company: "A", OK: true, Jobs: []model.Job{{ID: "a~1", Title: "x", URL: "u"}}})
	_ = s.WriteCompany(CompanyResult{CompanyID: "b", Company: "B", OK: true, Jobs: []model.Job{{ID: "b~1", Title: "y", URL: "u"}}})
	all, errs := s.LoadAll()
	if len(errs) != 0 {
		t.Fatalf("unexpected load errors: %v", errs)
	}
	if len(all) != 2 {
		t.Fatalf("LoadAll = %d, want 2", len(all))
	}
	st := RunStatus{Companies: 2, Succeeded: 2, TotalJobs: 2}
	if err := s.WriteStatus(st); err != nil {
		t.Fatal(err)
	}
	got, err := s.ReadStatus()
	if err != nil || got.Companies != 2 {
		t.Fatalf("status roundtrip: %v %+v", err, got)
	}
	if s.LatestMTime().IsZero() {
		t.Error("LatestMTime should be set")
	}
}

func TestSafeName(t *testing.T) {
	if safeName("a/b:c d") == "a/b:c d" {
		t.Error("safeName should sanitize path separators")
	}
}

func TestStoreFSMode(t *testing.T) {
	// Round-trip: write with a disk store, then read the same tree through the
	// read-only FS mode (as the embedded snapshot does).
	dir := t.TempDir()
	s, _ := New(dir)
	_ = s.WriteCompany(CompanyResult{CompanyID: "a", Company: "A", OK: true, Jobs: []model.Job{{ID: "a~1", Title: "SWE", URL: "u"}}})
	_ = s.WriteStatus(RunStatus{Companies: 1, Succeeded: 1, TotalJobs: 1})

	ro := OpenFS(os.DirFS(dir))
	if !ro.ReadOnly() {
		t.Fatal("OpenFS store must report ReadOnly")
	}
	ids, err := ro.ListCompanyIDs()
	if err != nil || len(ids) != 1 {
		t.Fatalf("ListCompanyIDs: %v %v", ids, err)
	}
	res, err := ro.ReadCompany("a")
	if err != nil || res.Jobs[0].Title != "SWE" {
		t.Fatalf("ReadCompany: %+v %v", res, err)
	}
	if st, err := ro.ReadStatus(); err != nil || st.TotalJobs != 1 {
		t.Fatalf("ReadStatus: %+v %v", st, err)
	}
	if !ro.LatestMTime().IsZero() {
		t.Error("FS mode LatestMTime should be zero (immutable)")
	}
	if err := ro.WriteCompany(CompanyResult{CompanyID: "x"}); err == nil {
		t.Error("writes must fail in FS mode")
	}
	all, errs := ro.LoadAll()
	if len(errs) != 0 || len(all) != 1 {
		t.Fatalf("LoadAll: %d results, %v", len(all), errs)
	}
}
