// Package dataset embeds a snapshot of the crawled data folder into the server
// binary, making it fully self-contained: frontend, API, and job data all ship
// in one file. `make binaries` syncs ./data into this package before building.
//
// The embedded snapshot is a fallback: when the server finds a live ./data
// folder on disk it prefers that (fresher, hot-reloadable); otherwise it serves
// the snapshot baked in at compile time.
package dataset

import (
	"embed"
	"io/fs"
	"strings"
)

//go:embed all:data
var embedded embed.FS

// FS returns the embedded data folder (companies/*.json + status.json).
func FS() (fs.FS, error) {
	return fs.Sub(embedded, "data")
}

// HasData reports whether the snapshot actually contains crawled companies
// (a fresh checkout without a synced crawl embeds only a placeholder).
func HasData() bool {
	sub, err := FS()
	if err != nil {
		return false
	}
	entries, err := fs.ReadDir(sub, "companies")
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			return true
		}
	}
	return false
}
