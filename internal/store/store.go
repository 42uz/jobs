// Package store persists crawl results to the /data folder as JSON and reads
// them back for the server. Writes are atomic (temp file + rename) so a crash
// mid-write never corrupts existing data, and the crawler can safely preserve
// the last good snapshot when a source fails.
package store

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"faangjobs/internal/model"
)

// CompanyResult is the on-disk record for one company: its metadata, the jobs
// captured in the most recent successful crawl, and the outcome of the latest
// attempt.
type CompanyResult struct {
	CompanyID string      `json:"companyId"`
	Company   string      `json:"company"`
	ATS       string      `json:"ats"`
	Slug      string      `json:"slug"`
	FetchedAt time.Time   `json:"fetchedAt"`
	OK        bool        `json:"ok"`
	Error     string      `json:"error,omitempty"`
	JobCount  int         `json:"jobCount"`
	Jobs      []model.Job `json:"jobs"`
}

// CompanyError records a failed company for the run status report.
type CompanyError struct {
	CompanyID string `json:"companyId"`
	Company   string `json:"company"`
	ATS       string `json:"ats"`
	Error     string `json:"error"`
}

// RunStatus summarizes a crawl run and is written to status.json.
type RunStatus struct {
	StartedAt  time.Time      `json:"startedAt"`
	FinishedAt time.Time      `json:"finishedAt"`
	DurationMs int64          `json:"durationMs"`
	Companies  int            `json:"companies"`
	Succeeded  int            `json:"succeeded"`
	Failed     int            `json:"failed"`
	TotalJobs  int            `json:"totalJobs"`
	Sources    map[string]int `json:"sources"`
	Errors     []CompanyError `json:"errors"`
}

// Store is a handle to a data directory — either a writable directory on disk
// or a read-only fs.FS (e.g. the snapshot embedded in the server binary).
type Store struct {
	dir          string
	companiesDir string
	fsys         fs.FS // non-nil = read-only FS mode
}

// New opens (creating if necessary) a store rooted at dir.
func New(dir string) (*Store, error) {
	companiesDir := filepath.Join(dir, "companies")
	if err := os.MkdirAll(companiesDir, 0o755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}
	return &Store{dir: dir, companiesDir: companiesDir}, nil
}

// OpenFS opens a read-only store over an fs.FS whose root contains
// companies/*.json and status.json. Write methods return an error.
func OpenFS(fsys fs.FS) *Store {
	return &Store{fsys: fsys}
}

// Dir returns the root data directory ("" in FS mode).
func (s *Store) Dir() string { return s.dir }

// ReadOnly reports whether the store is backed by a read-only FS.
func (s *Store) ReadOnly() bool { return s.fsys != nil }

var errReadOnly = fmt.Errorf("store is read-only (embedded snapshot)")

func (s *Store) companyPath(id string) string {
	return filepath.Join(s.companiesDir, safeName(id)+".json")
}

// WriteCompany atomically persists a company's result.
func (s *Store) WriteCompany(res CompanyResult) error {
	if s.fsys != nil {
		return errReadOnly
	}
	res.JobCount = len(res.Jobs)
	data, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal company %s: %w", res.CompanyID, err)
	}
	return writeFileAtomic(s.companyPath(res.CompanyID), data)
}

// ReadCompany loads a company's result, or an error if it does not exist.
func (s *Store) ReadCompany(id string) (*CompanyResult, error) {
	var data []byte
	var err error
	if s.fsys != nil {
		data, err = fs.ReadFile(s.fsys, path.Join("companies", safeName(id)+".json"))
	} else {
		data, err = os.ReadFile(s.companyPath(id))
	}
	if err != nil {
		return nil, err
	}
	var res CompanyResult
	if err := json.Unmarshal(data, &res); err != nil {
		return nil, fmt.Errorf("parse company %s: %w", id, err)
	}
	return &res, nil
}

// ListCompanyIDs returns the ids of all persisted companies.
func (s *Store) ListCompanyIDs() ([]string, error) {
	var entries []fs.DirEntry
	var err error
	if s.fsys != nil {
		entries, err = fs.ReadDir(s.fsys, "companies")
	} else {
		entries, err = os.ReadDir(s.companiesDir)
	}
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var ids []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".json") {
			continue
		}
		ids = append(ids, strings.TrimSuffix(name, ".json"))
	}
	sort.Strings(ids)
	return ids, nil
}

// LoadAll reads every persisted company result. Individual parse failures are
// skipped (and reported via the returned error slice) rather than aborting the
// whole load — resilience over strictness.
func (s *Store) LoadAll() ([]CompanyResult, []error) {
	ids, err := s.ListCompanyIDs()
	if err != nil {
		return nil, []error{err}
	}
	var results []CompanyResult
	var errs []error
	for _, id := range ids {
		res, err := s.ReadCompany(id)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		results = append(results, *res)
	}
	return results, errs
}

// WriteStatus atomically writes the run status report.
func (s *Store) WriteStatus(st RunStatus) error {
	if s.fsys != nil {
		return errReadOnly
	}
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(filepath.Join(s.dir, "status.json"), data)
}

// ReadStatus loads the last run status, if present.
func (s *Store) ReadStatus() (*RunStatus, error) {
	var data []byte
	var err error
	if s.fsys != nil {
		data, err = fs.ReadFile(s.fsys, "status.json")
	} else {
		data, err = os.ReadFile(filepath.Join(s.dir, "status.json"))
	}
	if err != nil {
		return nil, err
	}
	var st RunStatus
	if err := json.Unmarshal(data, &st); err != nil {
		return nil, err
	}
	return &st, nil
}

// LatestMTime returns the most recent modification time across the company
// files and status.json. The server polls this to detect fresh data. In FS
// mode the snapshot is immutable, so the zero time is returned.
func (s *Store) LatestMTime() time.Time {
	if s.fsys != nil {
		return time.Time{}
	}
	var latest time.Time
	consider := func(p string) {
		if fi, err := os.Stat(p); err == nil && fi.ModTime().After(latest) {
			latest = fi.ModTime()
		}
	}
	consider(filepath.Join(s.dir, "status.json"))
	if entries, err := os.ReadDir(s.companiesDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				if info, err := e.Info(); err == nil && info.ModTime().After(latest) {
					latest = info.ModTime()
				}
			}
		}
	}
	return latest
}

// --- helpers ---

// writeFileAtomic writes data to path via a temp file in the same directory
// followed by an atomic rename, fsyncing the file first.
func writeFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	tmpName := tmp.Name()
	defer func() {
		// best-effort cleanup if we didn't rename
		_ = os.Remove(tmpName)
	}()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write temp: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("sync temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp: %w", err)
	}
	if err := os.Chmod(tmpName, 0o644); err != nil {
		return fmt.Errorf("chmod temp: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("rename temp: %w", err)
	}
	// Fsync the directory so the rename itself is durable across a crash or
	// power loss (an fsync of the file alone does not guarantee the rename is
	// persisted).
	if d, err := os.Open(dir); err == nil {
		_ = d.Sync()
		_ = d.Close()
	}
	return nil
}

var unsafeName = strings.NewReplacer("/", "_", "\\", "_", "..", "_", ":", "_", " ", "_")

// safeName sanitizes an id for use as a filename. Because the sanitization is
// lossy (several characters collapse to '_'), a short hash of the original id is
// appended whenever any character was replaced, so two distinct ids can never
// map to the same file. IDs produced by registry.EnsureID are already
// filename-safe ([a-z0-9-]) and pass through unchanged.
func safeName(id string) string {
	clean := unsafeName.Replace(id)
	if clean != id {
		clean += "-" + model.StableID(id)
	}
	return clean
}
