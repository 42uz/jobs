package updater

import (
	"archive/zip"
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
	"log"

	"faangjobs/internal/httpapi"
	"faangjobs/internal/store"
)

// helper to create a valid zip archive in memory containing a "data/" prefix
func createTestZip(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	for name, content := range files {
		f, err := zw.Create(name)
		if err != nil {
			t.Fatalf("failed to create zip entry %s: %v", name, err)
		}
		if _, err := f.Write([]byte(content)); err != nil {
			t.Fatalf("failed to write content to zip entry %s: %v", name, err)
		}
	}

	if err := zw.Close(); err != nil {
		t.Fatalf("failed to close zip writer: %v", err)
	}

	return buf.Bytes()
}

func TestSyncData(t *testing.T) {
	tempDir := t.TempDir()
	targetDir := filepath.Join(tempDir, "data")

	if err := os.MkdirAll(filepath.Join(targetDir, "companies"), 0755); err != nil {
		t.Fatalf("failed to create initial target dir: %v", err)
	}
	oldFile := filepath.Join(targetDir, "companies", "old.json")
	if err := os.WriteFile(oldFile, []byte(`{"name":"old"}`), 0644); err != nil {
		t.Fatalf("failed to write old file: %v", err)
	}

	zipBytes := createTestZip(t, map[string]string{
		"data/companies/acme.json": `{"name":"Acme Corp"}`,
		"data/config.json":         `{"version":"2.0"}`,
	})

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(zipBytes)
	}))
	defer ts.Close()

	st, err := store.New(targetDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	logger := log.New(os.Stderr, "", log.LstdFlags)
	idx := httpapi.NewIndex(st, logger.Printf)

	if err := SyncData(ts.URL, targetDir, idx, st); err != nil {
		t.Fatalf("SyncData failed: %v", err)
	}

	acmePath := filepath.Join(targetDir, "companies", "acme.json")
	content, err := os.ReadFile(acmePath)
	if err != nil {
		t.Fatalf("expected file %s to exist after sync, got err: %v", acmePath, err)
	}
	if string(content) != `{"name":"Acme Corp"}` {
		t.Errorf("unexpected content: got %s", string(content))
	}

	if _, err := os.Stat(oldFile); !os.IsNotExist(err) {
		t.Errorf("expected old file %s to be deleted during atomic swap", oldFile)
	}
}

func TestStartTickerSyncInitialRun(t *testing.T) {
	tempDir := t.TempDir()
	targetDir := filepath.Join(tempDir, "data")

	zipBytes := createTestZip(t, map[string]string{
		"data/companies/test.json": `{"name":"Test Corp"}`,
	})

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(zipBytes)
	}))
	defer ts.Close()

	st, err := store.New(targetDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	logger := log.New(os.Stderr, "", log.LstdFlags)
	idx := httpapi.NewIndex(st, logger.Printf)

	StartTickerSync(ts.URL, targetDir, idx, st)

	testFile := filepath.Join(targetDir, "companies", "test.json")
	done := make(chan bool)

	go func() {
		for range 20 {
			if _, err := os.Stat(testFile); err == nil {
				done <- true
				return
			}
			time.Sleep(50 * time.Millisecond)
		}
		done <- false
	}()

	if success := <-done; !success {
		t.Fatalf("StartTickerSync initial sync failed to download and extract file within timeout")
	}
}