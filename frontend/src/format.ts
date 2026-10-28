export function formatNumber(n: number | undefined | null): string {
  return new Intl.NumberFormat().format(n ?? 0)
}

export function formatCurrency(n: number | undefined | null): string {
  const v = n ?? 0
  if (v === 0) return '$0.00'
  const abs = Math.abs(v)
  if (abs >= 1000) return '$' + formatNumberCompact(v)
  if (abs >= 1) return '$' + v.toFixed(2)
  if (abs >= 0.01) return '$' + v.toFixed(4)
  return '$' + v.toFixed(6)
}

export function formatNumberCompact(n: number | undefined | null): string {
  const v = n ?? 0
  const abs = Math.abs(v)
  if (abs >= 1_000_000) return (v / 1_000_000).toFixed(1).replace(/\.0$/, '') + 'M'
  if (abs >= 1_000) return (v / 1_000).toFixed(1).replace(/\.0$/, '') + 'k'
  return formatNumber(v)
}

export function formatLatency(ms: number): string {
  if (ms >= 1000) return (ms / 1000).toFixed(1) + 's'
  return Math.round(ms) + 'ms'
}
export function formatTps(n: number | null | undefined): string {
  const v = n ?? 0
  if (v >= 1000) return (v / 1000).toFixed(1).replace(/\.0$/, '') + 'k tok/s'
  if (v >= 10) return Math.round(v).toString() + ' tok/s'
  if (v > 0) return v.toFixed(1) + ' tok/s'
  return '—'
}

export function sinceForFilter(filter: string, timezone?: string): string | undefined {
  if (filter === 'today') {
    if (timezone && timezone !== 'UTC') {
      const p = parts(new Date(), timezone)
      if (!p) return undefined
      return zonedTimeToUtc(p.y, p.mo, p.d, 0, 0, 0, timezone).toISOString()
    }
    const d = new Date()
    d.setHours(0, 0, 0, 0)
    return d.toISOString()
  }
  const m = /^(\d+)d$/.exec(filter)
  if (m) return new Date(Date.now() - Number(m[1]) * 86400000).toISOString()
  return undefined
}

// Convert a wall-clock date in an IANA zone to its UTC instant. The offset can
// differ at the date boundary, so recalculate after the first estimate.
function zonedTimeToUtc(y: number, mo: number, d: number, h: number, mi: number, s: number, tz: string): Date {
  const desired = Date.UTC(y, mo - 1, d, h, mi, s)
  let instant = desired
  for (let i = 0; i < 3; i++) {
    const p = parts(new Date(instant), tz)
    if (!p) break
    const observed = Date.UTC(p.y, p.mo - 1, p.d, p.h, p.mi, p.s)
    const correction = desired - observed
    if (correction === 0) break
    instant += correction
  }
  return new Date(instant)
}
const dtfCache = new Map<string, Intl.DateTimeFormat>()
function getDtf(tz: string, opts: Intl.DateTimeFormatOptions): Intl.DateTimeFormat {
  const key = tz + '|' + JSON.stringify(opts)
  let f = dtfCache.get(key)
  if (!f) {
    f = new Intl.DateTimeFormat('en-CA', { timeZone: tz, ...opts })
    dtfCache.set(key, f)
  }
  return f
}


function parts(d: Date, tz: string): { y: number; mo: number; d: number; h: number; mi: number; s: number } | null {
  if (isNaN(d.getTime())) return null
  const fmt = getDtf(tz, { year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', second: '2-digit', hour12: false })
  const p = fmt.formatToParts(d)
  const g = (t: string) => parseInt(p.find(x => x.type === t)?.value ?? '0', 10)
  return { y: g('year'), mo: g('month'), d: g('day'), h: g('hour') % 24, mi: g('minute'), s: g('second') }
}

export function formatTimestamp(iso: string, tz: string): string {
  const p = parts(new Date(iso), tz)
  if (!p) return iso || '-'
  const f = (n: number) => n < 10 ? '0' + n : '' + n
  return `${p.y}-${f(p.mo)}-${f(p.d)} ${f(p.h)}:${f(p.mi)}:${f(p.s)}`
}

export function formatTime(iso: string, tz: string): string {
  const p = parts(new Date(iso), tz)
  if (!p) return iso || '-'
  const f = (n: number) => n < 10 ? '0' + n : '' + n
  return `${f(p.h)}:${f(p.mi)}:${f(p.s)}`
}

export function formatDate(iso: string, tz: string): string {
  const p = parts(new Date(iso), tz)
  if (!p) return iso || '-'
  const f = (n: number) => n < 10 ? '0' + n : '' + n
  return `${p.y}-${f(p.mo)}-${f(p.d)}`
}

export function formatBucket(bucket: string, tz: string): string {
  // Buckets may be partial ('2024-01-15T10') or full ISO; try parse, fallback to raw
  const d = new Date(bucket.includes('T') && !bucket.endsWith('Z') && !bucket.includes('+') ? bucket + ':00Z' : bucket)
  const p = parts(d, tz)
  if (!p) return bucket || '-'
  const f = (n: number) => n < 10 ? '0' + n : '' + n
  // If bucket has no time component (date-only), show date only
  if (/^\d{4}-\d{2}-\d{2}$/.test(bucket)) return `${p.y}-${f(p.mo)}-${f(p.d)}`
  return `${f(p.h)}:${f(p.mi)}`
}
