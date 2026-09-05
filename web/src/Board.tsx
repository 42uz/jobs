import { useMemo, useState } from 'react'
import type { DragEvent } from 'react'
import { STATUSES, STATUS_LABELS, tracker } from './applied'
import type { Status, TrackedEntry } from './applied'
import { cleanLocation, cleanTitle, relativeDate } from './format'
import { numberFmt } from './format'
import { ArrowUpRight, ChevronLeft, ChevronRight, GripIcon, XIcon } from './icons'

// A Trello-style board over the tracked applications: one column per pipeline
// stage, in progression order from Applied to Offer. Cards are dragged between
// columns; the ◀ ▶ buttons do the same thing for keyboard and touch, which
// HTML5 drag-and-drop does not serve.

const STATUS_ACCENT: Record<Status, string> = {
  applied: 'var(--blue)',
  interviewing: 'var(--yellow)',
  offer: 'var(--green)',
  rejected: 'var(--red)',
  ghosted: 'var(--text-3)',
}

// Only the id travels in the drag payload; the board looks the entry up.
const DRAG_TYPE = 'application/x-faangjobs-card'

type Props = { entries: TrackedEntry[]; totalTracked: number }

export function Board({ entries, totalTracked }: Props) {
  const [dragId, setDragId] = useState<string | null>(null)
  const [overColumn, setOverColumn] = useState<Status | null>(null)

  // Most recent first inside each column.
  const columns = useMemo(() => {
    const byStatus = {} as Record<Status, TrackedEntry[]>
    for (const s of STATUSES) byStatus[s] = []
    for (const e of entries) byStatus[e.status].push(e)
    for (const s of STATUSES) byStatus[s].sort((a, b) => b.at - a.at)
    return byStatus
  }, [entries])

  function onDrop(status: Status) {
    return (e: DragEvent) => {
      e.preventDefault()
      const id = e.dataTransfer.getData(DRAG_TYPE) || dragId
      if (id) tracker.move(id, status)
      setDragId(null)
      setOverColumn(null)
    }
  }

  if (totalTracked === 0) {
    return (
      <div className="state">
        <div className="h">Nothing tracked yet — click Apply on any job.</div>
        Applications land in the first column, then you drag them along as you go.
      </div>
    )
  }

  return (
    <div className="board" role="list" aria-label="Application pipeline">
      {STATUSES.map((status) => {
        const items = columns[status]
        return (
          <section
            key={status}
            role="listitem"
            className={'bcol' + (overColumn === status ? ' over' : '')}
            onDragOver={(e) => {
              e.preventDefault()
              e.dataTransfer.dropEffect = 'move'
              if (overColumn !== status) setOverColumn(status)
            }}
            onDragLeave={(e) => {
              // Ignore the leave events fired while crossing child elements.
              if (!e.currentTarget.contains(e.relatedTarget as Node | null)) {
                setOverColumn((c) => (c === status ? null : c))
              }
            }}
            onDrop={onDrop(status)}
            aria-label={`${STATUS_LABELS[status]} (${items.length})`}
          >
            <header className="bcol-head">
              <span className="bdot" style={{ background: STATUS_ACCENT[status] }} aria-hidden="true" />
              <span className="bcol-name">{STATUS_LABELS[status]}</span>
              <span className="bcol-count">{items.length ? numberFmt(items.length) : ''}</span>
            </header>
            <div className="bcol-body">
              {items.map((e) => (
                <Card
                  key={e.job.id}
                  entry={e}
                  status={status}
                  dragging={dragId === e.job.id}
                  onDragStart={(ev) => {
                    ev.dataTransfer.setData(DRAG_TYPE, e.job.id)
                    ev.dataTransfer.effectAllowed = 'move'
                    setDragId(e.job.id)
                  }}
                  onDragEnd={() => {
                    setDragId(null)
                    setOverColumn(null)
                  }}
                />
              ))}
              {items.length === 0 && (
                <div className="bempty">{overColumn === status ? 'Drop here' : '—'}</div>
              )}
            </div>
          </section>
        )
      })}
    </div>
  )
}

type CardProps = {
  entry: TrackedEntry
  status: Status
  dragging: boolean
  onDragStart: (e: DragEvent) => void
  onDragEnd: () => void
}

function Card({ entry, status, dragging, onDragStart, onDragEnd }: CardProps) {
  const { job, at } = entry
  const i = STATUSES.indexOf(status)
  const prev = i > 0 ? STATUSES[i - 1] : null
  const next = i < STATUSES.length - 1 ? STATUSES[i + 1] : null

  return (
    <article
      className={'bcard' + (dragging ? ' dragging' : '')}
      draggable
      onDragStart={onDragStart}
      onDragEnd={onDragEnd}
    >
      <div className="bcard-top">
        <span className="bgrip" aria-hidden="true"><GripIcon /></span>
        <span className="bcard-title" title={job.title}>{cleanTitle(job.title)}</span>
        <button
          className="bcard-x"
          onClick={() => tracker.remove(job.id)}
          aria-label={`Stop tracking ${job.title} at ${job.company}`}
          title="Remove from the board"
        >
          <XIcon />
        </button>
      </div>
      <div className="bcard-co" title={job.company}>{job.company}</div>
      <div className="bcard-meta">
        <span className="bcard-loc" title={job.location}>{cleanLocation(job.location)}</span>
        <span className="bcard-age" title={new Date(at).toLocaleString()}>{relativeDate(new Date(at).toISOString())}</span>
      </div>
      <div className="bcard-actions">
        <button
          className="bmove"
          disabled={!prev}
          onClick={() => prev && tracker.move(job.id, prev)}
          aria-label={prev ? `Move back to ${STATUS_LABELS[prev]}` : 'Already in the first stage'}
          title={prev ? `Back to ${STATUS_LABELS[prev]}` : ''}
        >
          <ChevronLeft />
        </button>
        <a
          className="bcard-open"
          href={job.url}
          target="_blank"
          rel="noopener noreferrer"
          title="Open the original posting"
        >
          Posting <ArrowUpRight />
        </a>
        <button
          className="bmove"
          disabled={!next}
          onClick={() => next && tracker.move(job.id, next)}
          aria-label={next ? `Move forward to ${STATUS_LABELS[next]}` : 'Already in the last stage'}
          title={next ? `Forward to ${STATUS_LABELS[next]}` : ''}
        >
          <ChevronRight />
        </button>
      </div>
    </article>
  )
}
