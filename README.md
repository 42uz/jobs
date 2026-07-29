# FaangJobs

**Developer jobs at top tech companies — worldwide, one click to apply.**

FaangJobs is a self-hosted job board that aggregates live openings from
hundreds of top technology companies **everywhere in the world** — Europe, the
Americas, Asia, the Middle East, Africa and Oceania. It is built as **two
standalone Go binaries** with a minimal, Notion-inspired **React** frontend
(search, region/country/category filters, one-click apply, applied-tracking):

- **`crawler`** — concurrently fetches fresh postings from every company in the
  registry and writes normalized JSON into the `data/` folder.
- **`server`** — serves a React single-page app plus a JSON API that reads from
  the `data/` folder. Search, filter, bookmark, mark-as-applied, and more.

The two binaries share nothing but the `data/` folder, so you can crawl on a
schedule (cron) and serve continuously; each is independently restartable.

**Current catalog: 598 live-verified companies across 13 adapters**, including
the official career sites of **Google, Apple, Amazon, Netflix, Uber, Spotify**
and Workday-hosted giants (NVIDIA, Salesforce, Adobe, Intel, Cisco, PayPal,
Micron, Broadcom, HP, Philips, ASML, Autodesk, Zalando), plus Greenhouse (247),
Ashby (302), SmartRecruiters (21) and Lever boards. A full crawl takes about
**five minutes** and keeps **~34,000 fresh developer/IT postings** from the 80+
countries those companies hire in (596 of 598 boards answered on the last run),
and the server searches them in single-digit milliseconds.

The crawler keeps only developer/IT roles by default (`-jobs dev`; software,
data/ML, infra, security, QA, sysadmin) — pass `-jobs all` for every profession.
It crawls **globally by default** (`-region global`); pass `-region europe`,
`-region apac`, `-region "north america"`, a country (`-region germany`) or a US
state / Canadian province (`-region california`) to scope the board down.

---

## Why it works (and keeps working)

Most big-tech companies don't expose a single "careers API." Instead, hundreds
of them host their boards on a handful of **Applicant Tracking Systems (ATS)**
that *do* have stable, public JSON endpoints. FaangJobs targets those:

| Adapter          | Endpoint                                              | Notes |
|------------------|------------------------------------------------------|-------|
| `greenhouse`     | `boards-api.greenhouse.io`                            | full descriptions |
| `lever`          | `api.lever.co`                                        | full descriptions |
| `ashby`          | `api.ashbyhq.com/posting-api`                         | full descriptions |
| `smartrecruiters`| `api.smartrecruiters.com`                             | links out |
| `workable`       | `apply.workable.com/api/v3`                           | |
| `recruitee`      | `{slug}.recruitee.com/api/offers`                     | full descriptions |
| `workday`        | `{tenant}.myworkdayjobs.com/wday/cxs/...`             | config-driven; sweeps every country facet |
| `amazon`         | `amazon.jobs/en/search.json`                          | full descriptions; sharded by job category |
| `googlecareers`  | `google.com/about/careers/applications` (embedded data) | official site; full descriptions |
| `applecareers`   | `jobs.apple.com` (embedded data)                      | official site |
| `uber`           | `uber.com/api/loadSearchJobsResults`                  | official site; full descriptions |
| `spotify`        | `api.lifeatspotify.com`                               | official site |
| `generic`        | any JSON endpoint, driven entirely by config          | Netflix, TikTok/ByteDance, Booking.com |

Every company slug in the registry was **verified live** before being added, so
the catalog is real, not guessed.

**Official career pages.** The biggest names are crawled straight from their
own career sites: **Google** (jobs data embedded in the careers app pages),
**Apple** (jobs.apple.com embedded data), **Amazon** (amazon.jobs), **Netflix**
(explore.jobs.netflix.net), **Uber** (uber.com careers API), **Spotify**
(api.lifeatspotify.com), plus Workday-hosted giants. The embedded-data adapters
are written defensively: if a site changes its markup, the adapter reports a
soft failure and the board keeps the last good snapshot instead of corrupting
data.

**The two holdouts.** Meta and Microsoft (and Tesla) actively block
non-browser clients (TLS fingerprinting / WAF — verified: they 400/403 every
plain-HTTP client regardless of headers). Rather than ship a headless-browser
dependency, FaangJobs leaves them out; companies whose official career pages run
on a public ATS (OpenAI, Anthropic, Stripe, Databricks, and hundreds more) are
covered through those official backends.

### Getting *every* opening, not just the first page

Going worldwide means fighting each backend's paging limits. Three of them cap
what a single query can return, and the adapters work around it:

- **Amazon** refuses offsets past 10 000 and reports a capped `hits` of exactly
  10 000, which silently truncates its global listing. The adapter detects the
  cap and re-crawls **shard by shard across job categories**, each comfortably
  below the limit; their union is the full board (~8 400 dev roles).
- **Workday** reports `total` only on the *first* page of a sweep (later pages
  return `0`), and hides multi-office roles behind "5 Locations" strings. The
  adapter remembers the first-page total and sweeps **each country facet
  separately** — including tenants that expose a custom calculated country field
  instead of the standard one — so every job also gains a country attribution.
- **Apple** repeats postings across pages and occasionally serves an
  unparseable record, so a short page is not the end of the listing. The adapter
  paginates against the site's own `totalRecords`, counting *new* postings and
  stopping only after a run of pages with nothing new.

---

## Resilience (the "make no mistake" bits)

- **Isolated failures.** One company failing (timeout, 404, throttling, a panic
  in an adapter) never aborts the crawl or affects other companies.
- **Last-good data preserved.** If a source fails, the previous successful
  snapshot for that company is kept — the board never blanks out on a blip.
- **Atomic writes.** Every file is written to a temp file, fsynced, and renamed,
  so a crash mid-write can never corrupt existing data.
- **Polite & robust HTTP.** Per-host concurrency limiting and pacing (many
  companies share one ATS host), exponential backoff with jitter, `Retry-After`
  support, response size caps, and per-request/per-company timeouts.
- **Graceful shutdown.** Both binaries handle SIGINT/SIGTERM; the crawler saves
  partial progress, the server drains in-flight requests.
- **Hot reload.** The server watches `data/` and rebuilds its in-memory index
  when the crawler produces new data — no restart needed.
- **Zero external Go dependencies.** The entire backend is the Go standard
  library (no supply-chain surface). The frontend uses only React.
- **Single self-contained binary.** The built frontend AND a snapshot of the
  crawled data are embedded into the `server` binary via `go:embed` — copy the
  one file anywhere and it serves the full board with zero external files. The
  embedded snapshot is *slim* (job descriptions stripped, since a global crawl
  is hundreds of megabytes of description text); a live `./data` folder, when
  present, takes precedence and serves descriptions too. Build with
  `make binaries FULL=1` to embed descriptions as well.

---

## Quick start

```bash
# 1. Build the frontend (once) — embeds into the server binary
make web

# 2. Build both binaries (syncs ./data into the server binary as an embedded snapshot)
make binaries        # -> ./bin/crawler and ./bin/server (server is fully self-contained)

# 3. Crawl (writes ./data). Try a subset first if you're impatient:
./bin/crawler -only greenhouse,ashby,amazon,netflix,workday   # quick
./bin/crawler                                                 # everything, everywhere (~6 min)

# 4. Serve
./bin/server                 # http://localhost:8080
```

Or with `make`: `make build && make crawl && make server`.

### Frontend development

```bash
./bin/server &        # API on :8080
make dev-web          # Vite dev server proxies /api -> :8080
```

---

## CLI reference

### `crawler`

```
-data string              data directory (default "./data")
-registry string          registry override JSON (default: embedded catalog)
-concurrency int          companies crawled in parallel (default 24)
-per-host int             max concurrent requests to one host (default 6)
-per-host-gap duration    min spacing between requests to a host (default 120ms)
-timeout duration         per-company timeout (default 15m)
-request-timeout duration default per-HTTP-request timeout (default 90s; a company
                          can raise its own with a "requestTimeout" config value)
-attempts int             max attempts per request (default 4)
-only string              filter: keep companies whose id/ats/slug/name matches
-region string            'global' (default), a region ('europe', 'apac',
                          'north america', 'latin america', 'middle east',
                          'africa', 'oceania'), a country ('germany'), or a
                          US state / Canadian province ('california')
-jobs string              'dev' (default: developer/IT roles only) or 'all'
-list                     print the resolved catalog and exit
-sources                  print registered adapters and exit
```

### `server`

```
-data string        data directory to read (default "./data")
-addr string        listen address (default ":8080")
-web-dir string     serve frontend from disk instead of the embedded build
-reload duration    how often to check data/ for changes (default 30s)
-embedded           serve the data snapshot embedded in the binary (ignore -data)
```

### 42.uz authentication

The server integrates 42.uz platform auth: the browser carries the platform's
long-lived `refresh-token` cookie; the server exchanges it for a short-lived
access-token JWT at the auth API and validates it locally (HS256, implemented
on stdlib crypto — no new dependencies). Exchanges are cached per token
(singleflight, bounded cache, negative caching) so request bursts never hammer
the auth API. Unauthenticated page loads redirect to the login page; API calls
get 401 JSON with the login URL. Authenticated users outside the allowlist
(`internal/httpapi/auth.go`) are redirected to the enrollment page (403 on
APIs). `GET /api/me` reports the signed-in user. Health endpoints stay open.

```
-jwt-secret  / FAANGJOBS_JWT_SECRET   HS256 secret (empty = auth disabled, open board)
-auth-api    / FAANGJOBS_AUTH_API     auth API base URL (default https://api.42.uz)
-login-url   / FAANGJOBS_LOGIN_URL    login redirect (default https://42.uz/login)
-enroll-url  / FAANGJOBS_ENROLL_URL   non-enrollee redirect (default https://42.uz/course/devops)
```

### Scheduling

Run the crawler from cron to keep the board fresh, e.g. every hour:

```cron
0 * * * * cd /path/to/faang-board && ./bin/crawler >> crawl.log 2>&1
```

The server picks up the new data automatically.

---

## API

| Method & path            | Description |
|--------------------------|-------------|
| `GET /api/jobs`          | search/filter/sort/paginate; returns jobs + live facet counts |
| `GET /api/jobs/{id}`     | one job with full (sanitized) description |
| `GET /api/companies`     | catalog with per-company job counts and crawl status |
| `GET /api/stats`         | totals, per-source counts, last-updated |
| `GET /api/filters`       | global facet values to seed the filter UI |
| `GET /healthz`           | health check |

`GET /api/jobs` query params: `q`, `company` (repeatable/CSV, by id), `source`,
`category`, `region`, `country`, `state`, `location`, `remote=true|false`,
`relocation=true`, `since=<days>`, `sort=recent|oldest|title|company`, `page`,
`pageSize`.

`region`, `country` and `state` compose
(`?region=North America&country=United States&state=California`), which is what
the sidebar drills through: picking a region narrows the country list to that
region, and picking the US or Canada (or just North America, whose two countries
would otherwise be a dead end) reveals a state/province list. Every location
facet names the facet it nests under in a `parent` field — a country's parent is
its region, a state's is its country — so the UI scopes each list without
another request. A job's region, country and state always come from the same
location string, so they can never disagree.

### Location model

`internal/model/geo.go` is the single source of truth for geography: every
recognized country with its region, ISO-2/ISO-3 codes, name variants and major
cities. `internal/model/location.go` resolves free-form location strings against
those tables and powers both the crawler's optional `-region` filter and the
server's region/country facets.

The matcher is precision-oriented, because two families of collisions dominate
real job locations:

- **cities that exist in several countries** (Cambridge, Birmingham, Paris,
  Vancouver, Santiago) are resolved by the state/province marker that
  accompanies the non-dominant one — "Paris, TX" is Texas, "Paris, France" is
  France;
- **two-letter codes that are both a US state and a country** (`IN` Indiana vs
  India, `CA` California vs Canada, `DE` Delaware vs Germany) resolve to the
  country only when the rest of the string agrees — "Bengaluru, KA, IN" is
  India, "Wilmington, DE" is Delaware.

Region-level strings ("Remote - EMEA", "APAC", "LATAM") resolve to a region with
no country. Jobs whose location names no place at all fall into the `Remote` or
`Other` facet bucket, so the region and country counts always add up to the
result total.

`internal/model/states.go` adds a third level for the two countries whose
postings routinely name one — US states and Canadian provinces. Because the
country is already known by then, codes that are ambiguous at the country level
("CA", "IN", "DE", "ON") are unambiguous, and a bare city still lands in a state
("Palo Alto" → California). About 88% of US/Canadian postings resolve to a
state; the rest say only "United States", and are simply absent from the state
facet rather than bucketed.

---

## Project layout

```
cmd/crawler         crawler binary
cmd/server          server binary
internal/model      normalized Job schema + geography + categorization
internal/registry   company catalog (embedded companies.json)
internal/source     ATS adapters (one file per adapter) + resilient HTTP client
internal/crawl      concurrent orchestrator
internal/store      atomic JSON persistence
internal/httpapi    in-memory search index, JSON API, SPA serving, middleware
internal/webui      embedded built frontend
internal/dataset    embedded data snapshot
tools/snapshot      build helper that syncs ./data into the embedded snapshot
web/                React + TypeScript + Vite source
data/               crawler output (git-ignored)
```

## Adding a company

Add an entry to `internal/registry/companies.json` (or a `-registry` override):

```json
{ "name": "Acme", "ats": "greenhouse", "slug": "acme" }
```

For a bespoke JSON endpoint, use the `generic` adapter with a `config` block —
see the Netflix entry in `companies.json` for a complete example.

Two `config` keys are understood for every adapter:

| Key              | Meaning |
|------------------|---------|
| `maxJobs`        | safety cap on how many postings to pull for this company |
| `requestTimeout` | per-request deadline for this company (e.g. `"8m"`), overriding `-request-timeout`. Used for boards that stream megabytes over minutes — Gopuff's Lever board takes ~4.5 minutes — so the global default can stay tight |

---

Built with Go (stdlib only) + React. No database, no external services.
