import { useSyncExternalStore } from 'react'
import type { Job } from './api'

// localStorage-backed application tracker: id → {at, status, job-slim}, with
// cross-tab sync. Wrapped in try/catch so private mode degrades to in-memory.

export const STATUSES = ['applied', 'interviewing', 'offer', 'rejected', 'ghosted'] as const
export type Status = (typeof STATUSES)[number]

export const STATUS_LABELS: Record<Status, string> = {
  applied: 'Applied',
  interviewing: 'Interviewing',
  offer: 'Offer',
  rejected: 'Rejected',
  ghosted: 'Ghosted',
}

export type TrackedEntry = { at: number; status: Status; job: Job }
type TrackedMap = Record<string, TrackedEntry>

const KEY = 'fb.applied.v3'
const OLD_KEY = 'fb.applied.v2'
const listeners = new Set<() => void>()

function read(): TrackedMap {
  try {
    const v3 = localStorage.getItem(KEY)
    if (v3) return JSON.parse(v3) || {}
    // One-time migration from the pre-status store.
    const v2 = localStorage.getItem(OLD_KEY)
    if (v2) {
      const old = JSON.parse(v2) || {}
      const migrated: TrackedMap = {}
      for (const [id, e] of Object.entries(old as Record<string, { at: number; job: Job }>)) {
        migrated[id] = { at: e.at, status: 'applied', job: e.job }
      }
      localStorage.setItem(KEY, JSON.stringify(migrated))
      return migrated
    }
    return {}
  } catch {
    return {}
  }
}

let cache = read()

function write(next: TrackedMap) {
  cache = next
  try {
    localStorage.setItem(KEY, JSON.stringify(next))
  } catch {
    /* in-memory only */
  }
  listeners.forEach((l) => l())
}

if (typeof window !== 'undefined') {
  window.addEventListener('storage', (e) => {
    if (e.key === KEY) {
      cache = read()
      listeners.forEach((l) => l())
    }
  })
}

function slim(job: Job): Job {
  const { description: _drop, ...rest } = job
  return rest
}

export const tracker = {
  subscribe(cb: () => void) {
    listeners.add(cb)
    return () => listeners.delete(cb)
  },
  snapshot(): TrackedMap {
    return cache
  },
  // mark records an application (idempotent; keeps an existing status).
  mark(job: Job) {
    if (cache[job.id]) return
    write({ ...cache, [job.id]: { at: Date.now(), status: 'applied', job: slim(job) } })
  },
  setStatus(job: Job, status: Status) {
    const prev = cache[job.id]
    write({
      ...cache,
      [job.id]: { at: prev?.at ?? Date.now(), status, job: prev?.job ?? slim(job) },
    })
  },
  // move restages an already-tracked application by id — what the board's
  // drag-and-drop needs, where only the card's id travels in the drag payload.
  move(id: string, status: Status) {
    const prev = cache[id]
    if (!prev || prev.status === status) return
    write({ ...cache, [id]: { ...prev, status } })
  },
  remove(id: string) {
    if (!cache[id]) return
    const next = { ...cache }
    delete next[id]
    write(next)
  },
}

export function useTracker(): TrackedMap {
  return useSyncExternalStore(tracker.subscribe, tracker.snapshot, tracker.snapshot)
}
