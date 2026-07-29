// Package crawl orchestrates the concurrent crawl: it dispatches companies to a
// bounded worker pool, isolates per-company failures, preserves the last good
// snapshot when a source fails, and produces a run-status report.
package crawl

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"faangjobs/internal/model"
	"faangjobs/internal/registry"
	"faangjobs/internal/source"
	"faangjobs/internal/store"
)

// Options configures a crawl run.
type Options struct {
	Concurrency       int           // number of companies fetched in parallel
	PerCompanyTimeout time.Duration // hard deadline for one company
	// Filter, when set, transforms and filters each company's jobs (e.g. region
	// policy): it may adjust a job and returns false to drop it before persisting.
	Filter func(model.Job) (model.Job, bool)
	Log    func(format string, args ...any)
}

// Crawler runs crawls against a catalog of companies.
type Crawler struct {
	store   *store.Store
	fetcher *source.Fetcher
	opts    Options
}

// New builds a Crawler.
func New(st *store.Store, f *source.Fetcher, opts Options) *Crawler {
	if opts.Concurrency <= 0 {
		opts.Concurrency = 24
	}
	if opts.PerCompanyTimeout <= 0 {
		opts.PerCompanyTimeout = 120 * time.Second
	}
	if opts.Log == nil {
		opts.Log = func(string, ...any) {}
	}
	return &Crawler{store: st, fetcher: f, opts: opts}
}

// Run crawls all companies and returns the aggregated status. A canceled ctx
// stops the dispatch of new companies; in-flight companies finish or time out.
// Results are written to the store as they complete, so partial progress
// survives an interruption.
func (c *Crawler) Run(ctx context.Context, companies []registry.Company) store.RunStatus {
	start := time.Now().UTC()
	total := len(companies)

	var (
		mu        sync.Mutex
		sources   = map[string]int{}
		errs      []store.CompanyError
		totalJobs int
		succeeded int
		failed    int
		done      int64
	)

	sem := make(chan struct{}, c.opts.Concurrency)
	var wg sync.WaitGroup

	for _, comp := range companies {
		if ctx.Err() != nil {
			break
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(comp registry.Company) {
			defer wg.Done()
			defer func() { <-sem }()

			res := c.crawlOne(ctx, comp)
			if err := c.store.WriteCompany(res); err != nil {
				c.opts.Log("write %s failed: %v", comp.ID, err)
			}

			mu.Lock()
			sources[comp.ATS] += res.JobCount
			totalJobs += res.JobCount
			if res.OK {
				succeeded++
			} else {
				failed++
				errs = append(errs, store.CompanyError{
					CompanyID: comp.ID, Company: comp.Name, ATS: comp.ATS, Error: res.Error,
				})
			}
			mu.Unlock()

			n := atomic.AddInt64(&done, 1)
			status := "ok"
			if !res.OK {
				status = "FAIL: " + res.Error
			}
			c.opts.Log("[%4d/%d] %-28s %-14s %4d jobs  %s", n, total, comp.Name, comp.ATS, res.JobCount, status)
		}(comp)
	}
	wg.Wait()

	finish := time.Now().UTC()
	st := store.RunStatus{
		StartedAt:  start,
		FinishedAt: finish,
		DurationMs: finish.Sub(start).Milliseconds(),
		Companies:  total,
		Succeeded:  succeeded,
		Failed:     failed,
		TotalJobs:  totalJobs,
		Sources:    sources,
		Errors:     errs,
	}
	if err := c.store.WriteStatus(st); err != nil {
		c.opts.Log("write status failed: %v", err)
	}
	return st
}

// crawlOne fetches a single company, isolating panics and preserving prior data
// on failure.
func (c *Crawler) crawlOne(ctx context.Context, comp registry.Company) (res store.CompanyResult) {
	now := time.Now().UTC()
	base := store.CompanyResult{
		CompanyID: comp.ID,
		Company:   comp.Name,
		ATS:       comp.ATS,
		Slug:      comp.Slug,
		FetchedAt: now,
	}

	// A misbehaving adapter must not take down the whole crawl.
	defer func() {
		if r := recover(); r != nil {
			res = c.preserve(base, "panic in adapter")
			c.opts.Log("PANIC crawling %s: %v", comp.ID, r)
		}
	}()

	src, ok := source.Get(comp.ATS)
	if !ok {
		return c.preserve(base, "no adapter registered for ats "+comp.ATS)
	}

	cctx, cancel := context.WithTimeout(ctx, c.opts.PerCompanyTimeout)
	defer cancel()

	jobs, err := src.Fetch(cctx, c.fetcherFor(comp), comp)
	if err != nil {
		return c.preserve(base, err.Error())
	}

	if c.opts.Filter != nil {
		kept := jobs[:0]
		for _, j := range jobs {
			if j2, ok := c.opts.Filter(j); ok {
				kept = append(kept, j2)
			}
		}
		jobs = kept
	}

	base.OK = true
	base.Jobs = jobs
	base.JobCount = len(jobs)
	return base
}

// fetcherFor returns the shared fetcher, or a view of it with a per-company
// request deadline. A handful of boards stream megabytes over minutes (Gopuff's
// Lever board); raising the deadline globally for them would let any hung
// request tie up a crawl worker for just as long.
func (c *Crawler) fetcherFor(comp registry.Company) *source.Fetcher {
	v, ok := comp.Config["requestTimeout"]
	if !ok {
		return c.fetcher
	}
	d, err := time.ParseDuration(strings.TrimSpace(v))
	if err != nil || d <= 0 {
		c.opts.Log("%s: ignoring invalid requestTimeout %q", comp.ID, v)
		return c.fetcher
	}
	return c.fetcher.WithTimeout(d)
}

// preserve returns a failed result that keeps the last successfully-crawled
// jobs (if any) so transient outages don't blank a company on the board.
func (c *Crawler) preserve(base store.CompanyResult, errMsg string) store.CompanyResult {
	base.OK = false
	base.Error = errMsg
	if prev, err := c.store.ReadCompany(base.CompanyID); err == nil && len(prev.Jobs) > 0 {
		base.Jobs = prev.Jobs
		base.JobCount = len(prev.Jobs)
	}
	return base
}
