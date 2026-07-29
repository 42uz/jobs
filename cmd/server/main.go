// Command server serves the FaangJobs web application: the embedded React
// frontend plus a JSON API backed by the crawled data folder. It is one of the
// two standalone binaries of FaangJobs.
package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"faangjobs/internal/dataset"
	"faangjobs/internal/httpapi"
	"faangjobs/internal/store"
)

// envOr returns the environment variable's value, or def when unset/empty.
func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// diskHasData reports whether dir/companies contains any company JSON files,
// without creating anything on disk.
func diskHasData(dir string) bool {
	entries, err := os.ReadDir(filepath.Join(dir, "companies"))
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

func main() {
	var (
		dataDir  = flag.String("data", "./data", "directory to read crawled data from")
		addr     = flag.String("addr", ":8080", "listen address")
		webDir   = flag.String("web-dir", "", "serve frontend from this directory instead of the embedded build")
		reload   = flag.Duration("reload", 30*time.Second, "how often to check the data folder for changes")
		embedded = flag.Bool("embedded", false, "serve the data snapshot embedded in the binary (ignore -data)")

		// 42.uz authentication. Auth is enabled iff a JWT secret is provided
		// (flag or FAANGJOBS_JWT_SECRET env); without one the board is open
		// (local dev / self-hosting).
		jwtSecret = flag.String("jwt-secret", envOr("FAANGJOBS_JWT_SECRET", ""), "HS256 secret for 42.uz access tokens (empty = auth disabled)")
		authAPI   = flag.String("auth-api", envOr("FAANGJOBS_AUTH_API", "https://api.42.uz"), "base URL of the 42.uz auth API")
		loginURL  = flag.String("login-url", envOr("FAANGJOBS_LOGIN_URL", "https://42.uz/login"), "where unauthenticated visitors are redirected")
		enrollURL = flag.String("enroll-url", envOr("FAANGJOBS_ENROLL_URL", "https://42.uz/course/devops"), "where authenticated non-enrollees are redirected")
	)
	flag.Parse()

	logger := log.New(os.Stderr, "", log.LstdFlags)

	// Data source selection: an explicit -embedded flag wins; otherwise prefer
	// a live ./data folder (fresher + hot-reloadable) and fall back to the
	// snapshot baked into the binary at build time.
	useEmbedded := *embedded
	if !useEmbedded && !diskHasData(*dataDir) && dataset.HasData() {
		useEmbedded = true
	}

	var st *store.Store
	if useEmbedded {
		fsys, err := dataset.FS()
		if err != nil {
			logger.Fatalf("embedded dataset: %v", err)
		}
		st = store.OpenFS(fsys)
		logger.Printf("serving the embedded data snapshot (baked in at build time)")
	} else {
		var err error
		st, err = store.New(*dataDir)
		if err != nil {
			logger.Fatalf("open store: %v", err)
		}
		logger.Printf("serving data from %s", *dataDir)
	}

	idx := httpapi.NewIndex(st, logger.Printf)

	// Hot reload only makes sense for the mutable on-disk folder.
	stopReload := make(chan struct{})
	if !useEmbedded {
		idx.StartAutoReload(*reload, stopReload)
	}

	handler, err := httpapi.Handler(idx, httpapi.Config{
		WebDir:    *webDir,
		JWTSecret: *jwtSecret,
		AuthAPI:   *authAPI,
		LoginURL:  *loginURL,
		EnrollURL: *enrollURL,
		Log:       logger.Printf,
	})
	if err != nil {
		logger.Fatalf("build handler: %v", err)
	}

	srv := &http.Server{
		Addr:              *addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       20 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		source := *dataDir
		if useEmbedded {
			source = "embedded snapshot"
		}
		logger.Printf("FaangJobs server listening on %s (data: %s)", *addr, source)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Fatalf("server error: %v", err)
		}
	}()

	<-ctx.Done()
	logger.Printf("shutting down…")
	close(stopReload)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Printf("graceful shutdown failed: %v", err)
	}
	logger.Printf("bye")
}
