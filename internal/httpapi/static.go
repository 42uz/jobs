package httpapi

import (
	"io"
	"io/fs"
	"net/http"
	"os"
	"path"
	"strings"
)

// staticHandler serves the single-page app. Unknown non-API, non-asset paths
// fall back to index.html so client-side routing works. Assets are served with
// long cache lifetimes; index.html is served without caching so deploys take
// effect immediately.
type staticHandler struct {
	fsys      fs.FS
	indexHTML []byte
	log       func(string, ...any)
}

func newStaticHandler(fsys fs.FS, log func(string, ...any)) (*staticHandler, error) {
	index, err := fs.ReadFile(fsys, "index.html")
	if err != nil {
		return nil, err
	}
	return &staticHandler{fsys: fsys, indexHTML: index, log: log}, nil
}

func (h *staticHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	upath := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
	if upath == "" || upath == "index.html" {
		h.serveIndex(w)
		return
	}

	f, err := h.fsys.Open(upath)
	if err != nil {
		// A missing asset (something with a file extension, or under assets/) is a
		// real 404 — don't mask it by serving the SPA shell. Extension-less paths
		// are treated as client-side routes and fall back to index.html.
		if looksLikeAsset(upath) {
			http.NotFound(w, r)
			return
		}
		h.serveIndex(w)
		return
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil || info.IsDir() {
		h.serveIndex(w)
		return
	}

	if ct := contentTypeFor(upath); ct != "" {
		w.Header().Set("Content-Type", ct)
	}
	// Vite fingerprints asset filenames, so they can be cached aggressively.
	if strings.HasPrefix(upath, "assets/") {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		w.Header().Set("Cache-Control", "public, max-age=3600")
	}
	if rs, ok := f.(io.ReadSeeker); ok {
		http.ServeContent(w, r, upath, info.ModTime(), rs)
		return
	}
	_, _ = io.Copy(w, f)
}

func (h *staticHandler) serveIndex(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write(h.indexHTML)
}

// looksLikeAsset reports whether a path should resolve to a real file (and thus
// 404 when missing) rather than falling back to the SPA shell. Client-side
// routes have no file extension; assets do (or live under assets/).
func looksLikeAsset(p string) bool {
	return strings.HasPrefix(p, "assets/") || path.Ext(p) != ""
}

func contentTypeFor(p string) string {
	switch {
	case strings.HasSuffix(p, ".js"), strings.HasSuffix(p, ".mjs"):
		return "text/javascript; charset=utf-8"
	case strings.HasSuffix(p, ".css"):
		return "text/css; charset=utf-8"
	case strings.HasSuffix(p, ".json"):
		return "application/json; charset=utf-8"
	case strings.HasSuffix(p, ".svg"):
		return "image/svg+xml"
	case strings.HasSuffix(p, ".png"):
		return "image/png"
	case strings.HasSuffix(p, ".ico"):
		return "image/x-icon"
	case strings.HasSuffix(p, ".webp"):
		return "image/webp"
	case strings.HasSuffix(p, ".woff2"):
		return "font/woff2"
	case strings.HasSuffix(p, ".html"):
		return "text/html; charset=utf-8"
	}
	return ""
}

// diskFS returns an fs.FS for a directory on disk, used when a web dir override
// is provided (handy for frontend development against a running server).
func diskFS(dir string) (fs.FS, error) {
	if _, err := os.Stat(path.Join(dir, "index.html")); err != nil {
		return nil, err
	}
	return os.DirFS(dir), nil
}
