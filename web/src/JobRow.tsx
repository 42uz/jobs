import { useEffect, useState } from 'react'
import type { Job } from './api'
import { api } from './api'
import { cleanLocation, cleanTitle, relativeDate, sanitizeHTML } from './format'
import { STATUS_LABELS, STATUSES, tracker } from './applied'
import type { Status } from './applied'
import { ArrowUpRight, Check, Triangle } from './icons'

type Props = {
  job: Job
  status?: Status
  expanded: boolean
  onToggle: () => void
}

const STATUS_TAG_CLASS: Record<Status, string> = {
  applied: 'blue',
  interviewing: 'yellow',
  offer: 'green',
  rejected: 'red',
  ghosted: 'gray',
}

// A single database-style row: toggle triangle, title + company, right-aligned
// quiet properties, and a one-click Apply that opens the source posting and
// records the application. Tracked jobs show their pipeline status.
export function JobRow({ job, status, expanded, onToggle }: Props) {
  return (
    <div className="jitem">
      <div className={'jrow' + (status ? ' done' : '')}>
        <button
          className={'jtoggle' + (expanded ? ' open' : '')}
          onClick={onToggle}
          aria-expanded={expanded}
          aria-label={expanded ? 'Hide details' : 'Show details'}
        >
          <Triangle />
        </button>
        <button className="jmain" onClick={onToggle}>
          <span className="jtitle" title={job.title}>{cleanTitle(job.title)}</span>
          <span className="jco" title={job.company}>{job.company}</span>
        </button>
        <div className="jside">
          {status ? (
            <span className={'jtag ' + STATUS_TAG_CLASS[status]}>{STATUS_LABELS[status]}</span>
          ) : (
            <>
              {job.remote && <span className="jtag blue">Remote</span>}
              {job.relocation && <span className="jtag green">Relocation</span>}
            </>
          )}
          <span className="jloc" title={job.location}>{cleanLocation(job.location)}</span>
          <span className="jtime">{relativeDate(job.postedAt)}</span>
          <a
            className={'japply' + (status ? ' done' : '')}
            href={job.url}
            target="_blank"
            rel="noopener noreferrer"
            onClick={() => tracker.mark(job)}
            aria-label={`Apply at ${job.company} (opens the original posting)`}
          >
            {status ? <><Check /> Applied</> : <>Apply <ArrowUpRight /></>}
          </a>
        </div>
      </div>
      {expanded && <Detail job={job} status={status} />}
    </div>
  )
}

function Detail({ job, status }: { job: Job; status?: Status }) {
  const [full, setFull] = useState<Job | null>(null)
  const [state, setState] = useState<'loading' | 'ok' | 'error'>('loading')

  useEffect(() => {
    const ctrl = new AbortController()
    setState('loading')
    api
      .job(job.id, ctrl.signal)
      .then((j) => {
        setFull(j)
        setState('ok')
      })
      .catch((e: unknown) => {
        if ((e as Error).name !== 'AbortError') setState('error')
      })
    return () => ctrl.abort()
  }, [job.id])

  const desc = full?.description?.trim()
  const allLocations = (job.locations && job.locations.length > 1
    ? job.locations
    : [job.location]
  ).map(cleanLocation)

  return (
    <div className="jdetail">
      <div className="jdetail-meta">
        {(job.categories || []).map((c) => (
          <span className="jtag" key={c}>{c}</span>
        ))}
        {job.department && <span className="jtag">{job.department}</span>}
        <span>{allLocations.join(' · ')}</span>
      </div>

      {state === 'loading' && <div className="state" style={{ padding: '8px 0' }}>Loading…</div>}
      {state === 'error' && (
        <div className="state" style={{ padding: '8px 0' }}>
          Couldn’t load the description — open the posting with Apply.
        </div>
      )}
      {state === 'ok' &&
        (desc ? (
          <div className="jdesc" dangerouslySetInnerHTML={{ __html: sanitizeHTML(desc) }} />
        ) : (
          <div className="state" style={{ padding: '8px 0' }}>
            No description provided — open the posting with Apply.
          </div>
        ))}

      <div className="jdetail-actions">
        <a
          className="btn-primary"
          href={job.url}
          target="_blank"
          rel="noopener noreferrer"
          onClick={() => tracker.mark(job)}
        >
          Apply <ArrowUpRight />
        </a>
      </div>

      <div className="jstatus-row" role="group" aria-label="Application status">
        <span className="jstatus-label">Status</span>
        {STATUSES.map((s) => (
          <button
            key={s}
            className={'jstatus' + (status === s ? ' on ' + STATUS_TAG_CLASS[s] : '')}
            aria-pressed={status === s}
            onClick={() => (status === s ? tracker.remove(job.id) : tracker.setStatus(job, s))}
          >
            {STATUS_LABELS[s]}
          </button>
        ))}
        {status && (
          <button className="jstatus clear" onClick={() => tracker.remove(job.id)}>
            Clear
          </button>
        )}
      </div>
    </div>
  )
}
