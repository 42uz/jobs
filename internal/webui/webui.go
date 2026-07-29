// Package webui embeds the built React frontend so the server ships as a single
// self-contained binary. The Vite build writes into ./dist (see
// web/vite.config.ts, outDir). A placeholder index.html is committed so the
// server always builds even before the frontend is built.
package webui

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var embedded embed.FS

// FS returns the embedded frontend rooted at the dist directory.
func FS() (fs.FS, error) {
	return fs.Sub(embedded, "dist")
}
