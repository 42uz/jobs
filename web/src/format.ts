export function numberFmt(n: number): string {
  return n.toLocaleString('en-US')
}

function parseDate(iso: string): Date | null {
  if (!iso) return null
  const d = new Date(iso)
  if (isNaN(d.getTime()) || d.getUTCFullYear() < 2000) return null
  return d
}

export function relativeDate(iso: string): string {
  const d = parseDate(iso)
  if (!d) return '—'
  const mins = Math.floor((Date.now() - d.getTime()) / 60000)
  if (mins < 1) return 'now'
  if (mins < 60) return `${mins}m`
  const hrs = Math.floor(mins / 60)
  if (hrs < 24) return `${hrs}h`
  const days = Math.floor(hrs / 24)
  if (days < 30) return `${days}d`
  const months = Math.floor(days / 30)
  if (months < 12) return `${months}mo`
  return `${Math.floor(days / 365)}y`
}

// Tidy a location string: collapse empty comma segments ("Remote, , Romania")
// and normalize comma spacing ("Warsaw,Poland" → "Warsaw, Poland").
export function cleanLocation(loc: string): string {
  if (!loc) return '—'
  return (
    loc
      .replace(/\s*,\s*(,\s*)+/g, ', ')
      .replace(/,(?=\S)/g, ', ')
      .replace(/^[\s,;]+|[\s,;]+$/g, '')
      .trim() || '—'
  )
}

// Some sources prefix titles with emoji ("🌐 Senior Care Ops"). Strip leading
// emoji/symbols only — never punctuation, so titles like "[Nyx Earth] Systems
// Engineer" keep their brackets.
export function cleanTitle(title: string): string {
  return title.replace(/^[\p{Extended_Pictographic}\p{Sk}\p{So}️‍\s]+/u, '').trim() || title
}

// Defense-in-depth client sanitizer (the server already strips scripts).
export function sanitizeHTML(html: string): string {
  if (!html) return ''
  return html
    .replace(/<script[\s\S]*?<\/script>/gi, '')
    .replace(/<style[\s\S]*?<\/style>/gi, '')
    .replace(/\son\w+\s*=\s*("[^"]*"|'[^']*'|[^\s>]+)/gi, '')
    .replace(/(href|src)\s*=\s*("javascript:[^"]*"|'javascript:[^']*')/gi, '$1="#"')
}
