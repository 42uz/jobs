import { useEffect, useMemo, useRef, useState } from 'react'
import { api } from './api'
import type { Facet, Job, Me, Stats } from './api'
import { numberFmt, relativeDate } from './format'
import { JobRow } from './JobRow'
import { STATUS_LABELS, STATUSES, useTracker } from './applied'
import type { Status } from './applied'
import { Check, Menu, Moon, SearchIcon, Sun, XIcon } from './icons'

const PAGE_SIZE = 25
// How many countries the sidebar lists: a short global top-N, but every country
// of a region once one is selected (regions have at most a few dozen).
const COUNTRY_LIMIT = 16
const COUNTRY_LIMIT_IN_REGION = 40
const STATE_LIMIT = 60

type Tab = 'all' | 'tracker'

const SORTS: { value: string; label: string }[] = [
  { value: 'recent', label: 'Newest first' },
  { value: 'oldest', label: 'Oldest first' },
  { value: 'title', label: 'Title A–Z' },
  { value: 'company', label: 'Company A–Z' },
]

const STATUS_DOT: Record<Status, string> = {
  applied: 'var(--blue)',
  interviewing: 'var(--yellow)',
  offer: 'var(--green)',
  rejected: 'var(--red)',
  ghosted: 'var(--text-3)',
}

export function App() {
  // ── theme ──────────────────────────────────────────────────────────
  const [theme, setTheme] = useState<'light' | 'dark'>(
    () => (document.documentElement.dataset.theme as 'light' | 'dark') || 'light',
  )
  useEffect(() => {
    document.documentElement.dataset.theme = theme
    try {
      localStorage.setItem('fb.theme', theme)
    } catch {
      /* ignore */
    }
  }, [theme])

  // ── page data ──────────────────────────────────────────────────────
  const [stats, setStats] = useState<Stats | null>(null)
  const [allCategories, setAllCategories] = useState<Facet[]>([])
  const [allCountries, setAllCountries] = useState<Facet[]>([])
  const [allRegions, setAllRegions] = useState<Facet[]>([])
  const [allStates, setAllStates] = useState<Facet[]>([])
  const [me, setMe] = useState<Me | null>(null)
  useEffect(() => {
    const ctrl = new AbortController()
    api.stats(ctrl.signal).then(setStats).catch(() => {})
    api.me(ctrl.signal).then(setMe).catch(() => {})
    api
      .filters(ctrl.signal)
      .then((f) => {
        setAllCategories(f.categories || [])
        setAllCountries(f.countries || [])
        setAllRegions(f.regions || [])
        setAllStates(f.states || [])
      })
      .catch(() => {})
    return () => ctrl.abort()
  }, [])

  // ── view state ─────────────────────────────────────────────────────
  const [tab, setTab] = useState<Tab>('all')
  const [statusFilter, setStatusFilter] = useState<'' | Status>('')
  const [qInput, setQInput] = useState('')
  const [q, setQ] = useState('')
  const [category, setCategory] = useState('')
  const [country, setCountry] = useState('')
  const [region, setRegion] = useState('')
  const [state, setState] = useState('')
  const [remote, setRemote] = useState(false)
  const [relocation, setRelocation] = useState(false)
  const [sort, setSort] = useState('recent')
  const [page, setPage] = useState(1)
  const [expandedId, setExpandedId] = useState<string | null>(null)
  const [sidebarOpen, setSidebarOpen] = useState(false)

  useEffect(() => {
    const t = setTimeout(() => {
      setQ(qInput.trim())
      setPage(1)
    }, 250)
    return () => clearTimeout(t)
  }, [qInput])

  useEffect(() => {
    if (!sidebarOpen) return
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setSidebarOpen(false)
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [sidebarOpen])

  // ── results (single full fetch, no pagination) ─────────────────────
  const [jobs, setJobs] = useState<Job[]>([])
  const [total, setTotal] = useState(0)
  const [liveCats, setLiveCats] = useState<Map<string, number>>(new Map())
  const [liveCountries, setLiveCountries] = useState<Map<string, number>>(new Map())
  const [liveRegions, setLiveRegions] = useState<Map<string, number>>(new Map())
  const [liveStates, setLiveStates] = useState<Map<string, number>>(new Map())
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [retryNonce, setRetryNonce] = useState(0)
  const reqId = useRef(0)

  useEffect(() => {
    if (tab !== 'all') return
    const id = ++reqId.current
    const ctrl = new AbortController()
    setLoading(true)
    setError(null)
    api
      .jobs(
        { q, category, country, region, state, remote, relocation, sort, page, pageSize: PAGE_SIZE },
        ctrl.signal,
      )
      .then((res) => {
        if (id !== reqId.current) return
        setTotal(res.total)
        setJobs((prev) => (page === 1 ? res.jobs : [...prev, ...res.jobs]))
        setLiveCats(new Map((res.facets?.categories || []).map((f) => [f.value, f.count])))
        setLiveCountries(new Map((res.facets?.countries || []).map((f) => [f.value, f.count])))
        setLiveRegions(new Map((res.facets?.regions || []).map((f) => [f.value, f.count])))
        setLiveStates(new Map((res.facets?.states || []).map((f) => [f.value, f.count])))
        setLoading(false)
      })
      .catch((e: unknown) => {
        if ((e as Error).name === 'AbortError' || id !== reqId.current) return
        setError(String((e as Error).message || e))
        setLoading(false)
      })
    return () => ctrl.abort()
  }, [tab, q, category, country, region, state, remote, relocation, sort, page, retryNonce])

  // ── tracker view ───────────────────────────────────────────────────
  const tracked = useTracker()
  const statusCounts = useMemo(() => {
    const counts = { applied: 0, interviewing: 0, offer: 0, rejected: 0, ghosted: 0 } as Record<
      Status,
      number
    >
    for (const e of Object.values(tracked)) counts[e.status]++
    return counts
  }, [tracked])
  const trackedCount = Object.keys(tracked).length

  const trackerJobs = useMemo(() => {
    const needle = q.toLowerCase()
    return Object.values(tracked)
      .filter((e) => !statusFilter || e.status === statusFilter)
      .sort((a, b) => b.at - a.at)
      .map((e) => e.job)
      .filter(
        (j) =>
          !needle ||
          j.title.toLowerCase().includes(needle) ||
          j.company.toLowerCase().includes(needle),
      )
  }, [tracked, statusFilter, q])

  const hasFilters =
    q !== '' ||
    category !== '' ||
    country !== '' ||
    region !== '' ||
    state !== '' ||
    remote ||
    relocation
  const shown = tab === 'all' ? jobs : trackerJobs
  const hasMore = tab === 'all' && !error && jobs.length < total

  function clearFilters() {
    setQInput('')
    setQ('')
    setCategory('')
    setCountry('')
    setRegion('')
    setState('')
    setRemote(false)
    setRelocation(false)
    setPage(1)
  }

  // Live counts overlay for sidebar facet lists.
  const categoryItems = useMemo(
    () =>
      allCategories.map((c) => ({
        value: c.value,
        count: liveCats.has(c.value) ? liveCats.get(c.value)! : hasFilters ? 0 : c.count,
      })),
    [allCategories, liveCats, hasFilters],
  )
  const regionItems = useMemo(
    () =>
      allRegions.map((r) => ({
        value: r.value,
        count: liveRegions.has(r.value) ? liveRegions.get(r.value)! : hasFilters ? 0 : r.count,
      })),
    [allRegions, liveRegions, hasFilters],
  )
  // With a region selected the country list is scoped to that region — picking
  // a country then narrows within it rather than replacing the region.
  const countryItems = useMemo(() => {
    // Parent-less facets (the Remote / Other buckets) belong to every region.
    const inRegion = region
      ? allCountries.filter((c) => !c.parent || c.parent === region)
      : allCountries
    return inRegion
      .slice(0, region ? COUNTRY_LIMIT_IN_REGION : COUNTRY_LIMIT)
      .map((c) => ({
        value: c.value,
        count: liveCountries.has(c.value) ? liveCountries.get(c.value)! : hasFilters ? 0 : c.count,
      }))
  }, [allCountries, liveCountries, hasFilters, region])

  // States/provinces exist only for the US and Canada, so this list appears
  // once the scope is one of them — which is what makes North America (two
  // countries, fifty-odd states) browsable.
  const countryParents = useMemo(
    () => new Map(allCountries.map((c) => [c.value, c.parent])),
    [allCountries],
  )
  const stateItems = useMemo(() => {
    const scoped = allStates.filter((s) =>
      country ? s.parent === country : region ? countryParents.get(s.parent ?? '') === region : false,
    )
    return scoped.slice(0, STATE_LIMIT).map((s) => ({
      value: s.value,
      count: liveStates.has(s.value) ? liveStates.get(s.value)! : hasFilters ? 0 : s.count,
    }))
  }, [allStates, countryParents, country, region, liveStates, hasFilters])

  // Any filter change starts back at page 1.
  function pickAndClose<T>(setter: (v: T) => void) {
    return (v: T) => {
      setter(v)
      setPage(1)
      setSidebarOpen(false)
    }
  }

  function toggleWithReset(setter: (fn: (v: boolean) => boolean) => void) {
    return () => {
      setter((v) => !v)
      setPage(1)
    }
  }

  function openTracker(status: '' | Status) {
    setTab('tracker')
    setStatusFilter(status)
    setSidebarOpen(false)
  }

  const sidebar = (
    <aside className={'sidebar' + (sidebarOpen ? ' open' : '')} aria-label="Filters">
      <div className="sb-brand">
        <span className="sb-logo" aria-hidden="true">F</span>
        <span className="sb-name">FaangJobs</span>
        <span className="grow" />
        <button
          className="sb-iconbtn"
          aria-label="Toggle theme"
          title="Toggle theme"
          onClick={() => setTheme((t) => (t === 'light' ? 'dark' : 'light'))}
        >
          {theme === 'light' ? <Moon /> : <Sun />}
        </button>
        <button
          className="sb-iconbtn sb-close"
          aria-label="Close filters"
          onClick={() => setSidebarOpen(false)}
        >
          <XIcon />
        </button>
      </div>

      <label className="sb-search">
        <SearchIcon />
        <input
          value={qInput}
          onChange={(e) => setQInput(e.target.value)}
          placeholder="Search jobs…"
          aria-label="Search jobs"
          autoComplete="off"
          spellCheck={false}
        />
        {qInput && (
          <button className="sb-x" aria-label="Clear search" onClick={() => setQInput('')}>
            <XIcon />
          </button>
        )}
      </label>

      <div className="sb-section" role="group" aria-label="Views">
        <div className="sb-label">Views</div>
        <button
          className={'sb-item' + (tab === 'all' ? ' on' : '')}
          onClick={() => pickAndClose(setTab)('all')}
        >
          <span className="sb-item-text">All jobs</span>
          <span className="sb-count">{stats ? numberFmt(stats.totalJobs) : ''}</span>
        </button>
        <button
          className={'sb-item' + (tab === 'tracker' && statusFilter === '' ? ' on' : '')}
          onClick={() => openTracker('')}
        >
          <span className="sb-item-text">My applications</span>
          <span className="sb-count">{trackedCount || ''}</span>
        </button>
      </div>

      <div className="sb-section" role="group" aria-label="Pipeline">
        <div className="sb-label">Pipeline</div>
        {STATUSES.map((s) => (
          <button
            key={s}
            className={'sb-item' + (tab === 'tracker' && statusFilter === s ? ' on' : '')}
            onClick={() => openTracker(s)}
          >
            <span className="sb-dot" style={{ background: STATUS_DOT[s] }} aria-hidden="true" />
            <span className="sb-item-text">{STATUS_LABELS[s]}</span>
            <span className="sb-count">{statusCounts[s] || ''}</span>
          </button>
        ))}
      </div>

      <div className="sb-section" role="group" aria-label="Region">
        <div className="sb-label">Region</div>
        <button
          className={'sb-item' + (region === '' && country === '' ? ' on' : '')}
          onClick={() => {
            setRegion('')
            setState('')
            pickAndClose(setCountry)('')
          }}
        >
          <span className="sb-item-text">Everywhere</span>
        </button>
        {regionItems.map((r) => (
          <button
            key={r.value}
            className={'sb-item' + (region === r.value ? ' on' : '')}
            onClick={() => {
              setCountry('')
              setState('')
              pickAndClose(setRegion)(region === r.value ? '' : r.value)
            }}
          >
            <span className="sb-item-text">{r.value}</span>
            <span className="sb-count">{r.count ? numberFmt(r.count) : ''}</span>
          </button>
        ))}
      </div>

      <div className="sb-section" role="group" aria-label="Country">
        <div className="sb-label">{region ? `Country in ${region}` : 'Country'}</div>
        {region && (
          <button
            className={'sb-item' + (country === '' ? ' on' : '')}
            onClick={() => {
              setState('')
              pickAndClose(setCountry)('')
            }}
          >
            <span className="sb-item-text">All of {region}</span>
          </button>
        )}
        {countryItems.map((c) => (
          <button
            key={c.value}
            className={'sb-item' + (country === c.value ? ' on' : '')}
            onClick={() => {
              setState('')
              pickAndClose(setCountry)(country === c.value ? '' : c.value)
            }}
          >
            <span className="sb-item-text">{c.value}</span>
            <span className="sb-count">{c.count ? numberFmt(c.count) : ''}</span>
          </button>
        ))}
      </div>

      {stateItems.length > 0 && (
        <div className="sb-section" role="group" aria-label="State">
          <div className="sb-label">{country === 'Canada' ? 'Province' : 'State'}</div>
          <button
            className={'sb-item' + (state === '' ? ' on' : '')}
            onClick={() => pickAndClose(setState)('')}
          >
            <span className="sb-item-text">All of {country || region}</span>
          </button>
          {stateItems.map((s) => (
            <button
              key={s.value}
              className={'sb-item' + (state === s.value ? ' on' : '')}
              onClick={() => pickAndClose(setState)(state === s.value ? '' : s.value)}
            >
              <span className="sb-item-text">{s.value}</span>
              <span className="sb-count">{s.count ? numberFmt(s.count) : ''}</span>
            </button>
          ))}
        </div>
      )}

      <div className="sb-section" role="group" aria-label="Category">
        <div className="sb-label">Category</div>
        <button
          className={'sb-item' + (category === '' ? ' on' : '')}
          onClick={() => pickAndClose(setCategory)('')}
        >
          <span className="sb-item-text">All categories</span>
        </button>
        {categoryItems.map((c) => (
          <button
            key={c.value}
            className={'sb-item' + (category === c.value ? ' on' : '')}
            onClick={() => pickAndClose(setCategory)(category === c.value ? '' : c.value)}
          >
            <span className="sb-item-text">{c.value}</span>
            <span className="sb-count">{c.count ? numberFmt(c.count) : ''}</span>
          </button>
        ))}
      </div>

      <div className="sb-section" role="group" aria-label="Filters">
        <div className="sb-label">Filters</div>
        <button
          className={'sb-item' + (remote ? ' on' : '')}
          onClick={toggleWithReset(setRemote)}
          aria-pressed={remote}
        >
          <span className="sb-check" aria-hidden="true">{remote && <Check />}</span>
          <span className="sb-item-text">Remote</span>
        </button>
        <button
          className={'sb-item' + (relocation ? ' on' : '')}
          onClick={toggleWithReset(setRelocation)}
          aria-pressed={relocation}
        >
          <span className="sb-check" aria-hidden="true">{relocation && <Check />}</span>
          <span className="sb-item-text">Relocation</span>
        </button>
      </div>

      <div className="sb-section" role="group" aria-label="Sort">
        <div className="sb-label">Sort</div>
        {SORTS.map((s) => (
          <button
            key={s.value}
            className={'sb-item' + (sort === s.value ? ' on' : '')}
            onClick={() => pickAndClose(setSort)(s.value)}
          >
            <span className="sb-item-text">{s.label}</span>
          </button>
        ))}
      </div>

      {hasFilters && (
        <div className="sb-section">
          <button className="sb-item sb-clear" onClick={clearFilters}>
            <XIcon /> Clear filters
          </button>
        </div>
      )}
    </aside>
  )

  const trackerTitle =
    statusFilter === '' ? 'My applications' : STATUS_LABELS[statusFilter as Status]

  // Profile badge: first 4 characters of the 42.uz username (sans "@").
  const avatarLabel = useMemo(() => {
    if (!me || me.anonymous) return ''
    return (me.username || me.name || me.id || '').replace(/^@/, '').slice(0, 4).toUpperCase()
  }, [me])
  const avatarTitle = me ? [me.name, me.username].filter(Boolean).join(' · ') : ''
  const avatar = avatarLabel ? (
    <span className="avatar" title={avatarTitle} aria-label={`Signed in as ${avatarTitle}`}>
      {avatarLabel}
    </span>
  ) : null

  return (
    <div className="shell">
      {sidebar}
      {sidebarOpen && <div className="backdrop" onClick={() => setSidebarOpen(false)} />}

      <main className="main">
        {avatar && <div className="avatar-corner">{avatar}</div>}
        <div className="mobilebar">
          <button className="sb-iconbtn" aria-label="Open filters" onClick={() => setSidebarOpen(true)}>
            <Menu />
          </button>
          <span className="sb-name">42 FaangJobs</span>
          <span className="grow" />
          {avatar}
        </div>

        <div className="page">
          <div className="page-icon" aria-hidden="true">🌍</div>
          <h1 className="page-title">
            {tab === 'all' ? '42 FaangJobs' : trackerTitle}
          </h1>
          {tab === 'all' && (
            <>
              <p className="page-desc">
                Software, data, infrastructure and security roles at top tech companies —
                every opening, everywhere. One click takes you to the original posting.
              </p>
              <p className="page-stats">
                {stats
                  ? `${numberFmt(stats.totalJobs)} roles · ${numberFmt(stats.companies)} companies · ` +
                    (relativeDate(stats.lastUpdated) === 'now'
                      ? 'updated just now'
                      : `updated ${relativeDate(stats.lastUpdated)} ago`)
                  : ' '}
              </p>
            </>
          )}
          {tab === 'tracker' && (
            <p className="page-desc">
              Track every application through its pipeline — set a status from any job’s details.
            </p>
          )}

          <div className="countline" role="status">
            {tab === 'tracker'
              ? `${numberFmt(trackerJobs.length)} ${trackerJobs.length === 1 ? 'job' : 'jobs'}`
              : loading
                ? 'Loading…'
                : `${numberFmt(total)} ${total === 1 ? 'job' : 'jobs'}`}
          </div>

          <div className="list">
            {tab === 'all' && error && (
              <div className="state">
                <div className="h">Couldn’t load jobs — {error}</div>
                <button onClick={() => setRetryNonce((n) => n + 1)}>Retry</button>
              </div>
            )}

            {tab === 'all' && !error && loading && page === 1 && (
              <>
                {Array.from({ length: 10 }).map((_, i) => (
                  <div className="skel-row" key={i}>
                    <div className="skel" style={{ width: 11, height: 11 }} />
                    <div className="skel" style={{ width: `${46 - (i % 4) * 7}%` }} />
                    <div className="skel" style={{ width: '10%', marginLeft: 'auto' }} />
                  </div>
                ))}
              </>
            )}

            {!error && !(tab === 'all' && loading && page === 1) && shown.length === 0 && (
              <div className="state">
                <div className="h">
                  {tab === 'tracker'
                    ? trackedCount === 0
                      ? 'Nothing tracked yet — click Apply on any job.'
                      : statusFilter
                        ? `No jobs marked ${STATUS_LABELS[statusFilter as Status].toLowerCase()}.`
                        : 'No tracked jobs match your search.'
                    : 'No jobs match.'}
                </div>
                {tab === 'all' && hasFilters && <button onClick={clearFilters}>Clear filters</button>}
              </div>
            )}

            {!(tab === 'all' && (error || (loading && page === 1))) &&
              shown.map((job) => (
                <JobRow
                  key={job.id}
                  job={job}
                  status={tracked[job.id]?.status}
                  expanded={expandedId === job.id}
                  onToggle={() => setExpandedId((id) => (id === job.id ? null : job.id))}
                />
              ))}

            {hasMore && !(loading && page === 1) && (
              <button className="more" disabled={loading} onClick={() => setPage((p) => p + 1)}>
                {loading
                  ? 'Loading…'
                  : `Load more  ·  ${numberFmt(total - jobs.length)} remaining`}
              </button>
            )}
          </div>
        </div>
      </main>
    </div>
  )
}
