// Command crawler concurrently fetches fresh job postings from every company in
// the registry and writes normalized JSON into the data folder. It is one of
// the two standalone binaries of FaangJobs.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"
	"time"

	"faangjobs/internal/crawl"
	"faangjobs/internal/model"
	"faangjobs/internal/registry"
	"faangjobs/internal/source"
	"faangjobs/internal/store"
)

// professionFilter returns the per-job filter for the requested profession
// scope, or nil to keep all professions.
func professionFilter(jobs string) func(model.Job) (model.Job, bool) {
	switch strings.ToLower(strings.TrimSpace(jobs)) {
	case "", "all", "any":
		return nil
	default: // "dev" and anything else defaults to the developer/IT policy
		return model.DevJob
	}
}

// chainFilters composes transform/keep filters; a job must pass every one.
func chainFilters(filters ...func(model.Job) (model.Job, bool)) func(model.Job) (model.Job, bool) {
	var active []func(model.Job) (model.Job, bool)
	for _, f := range filters {
		if f != nil {
			active = append(active, f)
		}
	}
	if len(active) == 0 {
		return nil
	}
	return func(j model.Job) (model.Job, bool) {
		for _, f := range active {
			var ok bool
			if j, ok = f(j); !ok {
				return j, false
			}
		}
		return j, true
	}
}

func main() {
	var (
		dataDir      = flag.String("data", "./data", "directory to write crawled data into")
		registryPath = flag.String("registry", "", "path to a registry override JSON file (defaults to the embedded catalog)")
		concurrency  = flag.Int("concurrency", 24, "number of companies to crawl in parallel")
		perHost      = flag.Int("per-host", 6, "max concurrent requests to a single host")
		limit        = flag.Int("limit", 0, "if >0, crawl at most this many companies (handy for demos)")
		perHostGap   = flag.Duration("per-host-gap", 120*time.Millisecond, "minimum spacing between requests to the same host")
		timeout      = flag.Duration("timeout", 15*time.Minute, "per-company timeout (the biggest career sites page through thousands of jobs)")
		attempts     = flag.Int("attempts", 4, "max HTTP attempts per request (retries)")
		reqTimeout   = flag.Duration("request-timeout", 90*time.Second, "default per-HTTP-request timeout; a company can raise it with a requestTimeout config value")
		only         = flag.String("only", "", "comma-separated filter: keep companies whose id/ats/slug matches any token")
		region       = flag.String("region", "global", "location filter: 'global' (everywhere), a region ('europe', 'apac', 'north america', …), a country ('germany') or a US state / Canadian province ('california')")
		jobsScope    = flag.String("jobs", "dev", "profession filter: 'dev' (software/IT roles only) or 'all'")
		listOnly     = flag.Bool("list", false, "print the resolved catalog and exit")
		showSources  = flag.Bool("sources", false, "print registered source adapters and exit")
	)
	flag.Parse()

	logger := log.New(os.Stderr, "", log.LstdFlags)

	// The location policy ("global" by default, or a region/country name).
	locFilter, ok := model.LocationFilter(*region)
	if !ok {
		logger.Fatalf("unknown -region %q: use 'global', a region (%s), a country name, or a US state / Canadian province name",
			*region, strings.Join(model.Regions, ", "))
	}

	if *showSources {
		fmt.Println("registered adapters:", strings.Join(source.Kinds(), ", "))
		return
	}

	cat, err := registry.Load(*registryPath)
	if err != nil {
		logger.Fatalf("load registry: %v", err)
	}
	companies := filterCompanies(cat.Enabled(), *only)
	if len(companies) == 0 {
		logger.Fatalf("no companies to crawl (after --only filter)")
	}
	if *limit > 0 && *limit < len(companies) {
		companies = companies[:*limit]
	}

	if *listOnly {
		printCatalog(companies)
		return
	}

	st, err := store.New(*dataDir)
	if err != nil {
		logger.Fatalf("open store: %v", err)
	}

	fetcher := source.NewFetcher(source.FetcherOptions{
		Timeout:       *reqTimeout,
		MaxAttempts:   *attempts,
		PerHostConc:   *perHost,
		PerHostMinGap: *perHostGap,
		Log:           func(f string, a ...any) { logger.Printf("http: "+f, a...) },
	})

	crawler := crawl.New(st, fetcher, crawl.Options{
		Concurrency:       *concurrency,
		PerCompanyTimeout: *timeout,
		Filter:            chainFilters(locFilter, professionFilter(*jobsScope)),
		Log:               logger.Printf,
	})

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	logger.Printf("crawling %d companies (concurrency=%d, per-host=%d, region=%s, jobs=%s) into %s", len(companies), *concurrency, *perHost, *region, *jobsScope, *dataDir)
	start := time.Now()
	status := crawler.Run(ctx, companies)
	elapsed := time.Since(start).Round(time.Millisecond)

	logger.Printf("──────────────────────────────────────────────")
	logger.Printf("done in %s: %d companies, %d ok, %d failed, %d jobs", elapsed, status.Companies, status.Succeeded, status.Failed, status.TotalJobs)
	printSources(status.Sources)
	if status.Failed > 0 {
		logger.Printf("%d companies failed (see status.json). First few:", status.Failed)
		for i, e := range status.Errors {
			if i >= 8 {
				break
			}
			logger.Printf("  - %s (%s): %s", e.Company, e.ATS, truncate(e.Error, 120))
		}
	}
	if ctx.Err() != nil {
		logger.Printf("note: crawl was interrupted; partial results were saved")
	}
}

func filterCompanies(companies []registry.Company, only string) []registry.Company {
	only = strings.TrimSpace(only)
	if only == "" {
		return companies
	}
	var tokens []string
	for _, t := range strings.Split(only, ",") {
		if t = strings.ToLower(strings.TrimSpace(t)); t != "" {
			tokens = append(tokens, t)
		}
	}
	var out []registry.Company
	for _, c := range companies {
		hay := strings.ToLower(c.ID + " " + c.ATS + " " + c.Slug + " " + c.Name)
		for _, t := range tokens {
			if strings.Contains(hay, t) {
				out = append(out, c)
				break
			}
		}
	}
	return out
}

func printCatalog(companies []registry.Company) {
	byATS := map[string]int{}
	for _, c := range companies {
		byATS[c.ATS]++
	}
	fmt.Printf("%d companies:\n", len(companies))
	for _, c := range companies {
		fmt.Printf("  %-30s %-16s %s\n", c.Name, c.ATS, c.Slug)
	}
	fmt.Println("by adapter:")
	printSources(byATS)
}

func printSources(m map[string]int) {
	type kv struct {
		k string
		v int
	}
	var xs []kv
	for k, v := range m {
		xs = append(xs, kv{k, v})
	}
	sort.Slice(xs, func(i, j int) bool { return xs[i].v > xs[j].v })
	for _, x := range xs {
		fmt.Printf("    %-16s %d\n", x.k, x.v)
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
