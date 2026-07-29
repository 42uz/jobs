export type Job = {
  id: string
  companyId: string
  company: string
  title: string
  location: string
  locations?: string[]
  remote: boolean
  relocation: boolean
  department?: string
  url: string
  description?: string
  postedAt: string
  updatedAt: string
  source: string
  categories?: string[]
}

export type Facet = { value: string; label?: string; count: number; parent?: string }

export type JobsResponse = {
  total: number
  page: number
  pageSize: number
  jobs: Job[]
  facets?: {
    companies: Facet[]
    categories: Facet[]
    sources: Facet[]
    locations: Facet[]
    countries?: Facet[]
    regions?: Facet[]
    states?: Facet[]
  }
}

export type Filters = {
  companies: Facet[]
  categories: Facet[]
  sources: Facet[]
  locations: Facet[]
  countries?: Facet[]
  regions?: Facet[]
  states?: Facet[]
}

export type Stats = {
  totalJobs: number
  companies: number
  lastUpdated: string
  builtAt: string
}

export type JobQuery = {
  q?: string
  category?: string
  country?: string
  region?: string
  state?: string
  remote?: boolean
  relocation?: boolean
  sort?: string
  page?: number
  pageSize?: number
}

function qs(params: JobQuery): string {
  const p = new URLSearchParams()
  if (params.q) p.set('q', params.q)
  if (params.category) p.set('category', params.category)
  if (params.country) p.set('country', params.country)
  if (params.region) p.set('region', params.region)
  if (params.state) p.set('state', params.state)
  if (params.remote) p.set('remote', 'true')
  if (params.relocation) p.set('relocation', 'true')
  if (params.sort) p.set('sort', params.sort)
  if (params.page) p.set('page', String(params.page))
  if (params.pageSize) p.set('pageSize', String(params.pageSize))
  return p.toString()
}

async function getJSON<T>(url: string, signal?: AbortSignal): Promise<T> {
  const res = await fetch(url, { signal, headers: { Accept: 'application/json' } })
  if (!res.ok) throw new Error(`${res.status} ${res.statusText}`)
  return (await res.json()) as T
}

export type Me = {
  anonymous?: boolean
  id?: string
  name?: string
  username?: string
}

export const api = {
  me: (signal?: AbortSignal) => getJSON<Me>('/api/me', signal),
  jobs: (params: JobQuery, signal?: AbortSignal) =>
    getJSON<JobsResponse>('/api/jobs?' + qs(params), signal),
  job: (id: string, signal?: AbortSignal) =>
    getJSON<Job>('/api/jobs/' + encodeURIComponent(id), signal),
  filters: (signal?: AbortSignal) => getJSON<Filters>('/api/filters', signal),
  stats: (signal?: AbortSignal) => getJSON<Stats>('/api/stats', signal),
}
