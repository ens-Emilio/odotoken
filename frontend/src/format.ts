import type { LocaleCode } from './i18n/types'

// Formatting follows the language chosen in Settings, never the browser
// default: Intl would otherwise silently use the OS locale. Omitting `locale`
// keeps the historical ISO output for exports and tests.
const numberCache = new Map<string, Intl.NumberFormat>()
function numberFormat(locale: LocaleCode | undefined, options?: Intl.NumberFormatOptions): Intl.NumberFormat {
  const key = `${locale ?? 'default'}|${JSON.stringify(options ?? {})}`
  let f = numberCache.get(key)
  if (!f) {
    f = new Intl.NumberFormat(locale, options)
    numberCache.set(key, f)
  }
  return f
}

export function formatNumber(n: number | undefined | null, locale?: LocaleCode): string {
  return numberFormat(locale ?? ('en-US' as LocaleCode)).format(n ?? 0)
}

export function formatCurrency(n: number | undefined | null, locale?: LocaleCode): string {
  const v = n ?? 0
  if (v === 0) return '$0.00'
  const abs = Math.abs(v)
  if (abs >= 1000) return '$' + formatNumberCompact(v, locale)
  // Costs are dollars and are never converted, so the symbol and the digit
  // count stay fixed. Without an explicit locale the historical `toFixed`
  // output is kept: callers that pass no locale (exports, tests) must not
  // silently inherit the OS decimal separator.
  const digits = abs >= 1 ? 2 : abs >= 0.01 ? 4 : 6
  if (!locale) return '$' + v.toFixed(digits)
  return '$' + numberFormat(locale, { minimumFractionDigits: digits, maximumFractionDigits: digits }).format(v)
}

// Totals that get compared row by row are read, not scanned: compacting
// $1,048.72 to "$1k" hides the digits the reader is looking at.
export function formatCurrencyExact(n: number | undefined | null, locale?: LocaleCode): string {
  // The historical no-locale output is en-US: a locale-less caller must not
  // inherit the OS formatting.
  return numberFormat(locale ?? ('en-US' as LocaleCode), { style: 'currency', currency: 'USD' }).format(n ?? 0)
}

export function formatNumberCompact(n: number | undefined | null, locale?: LocaleCode): string {
  const v = n ?? 0
  const abs = Math.abs(v)
  if (abs >= 1_000_000) return (v / 1_000_000).toFixed(1).replace(/\.0$/, '') + 'M'
  if (abs >= 1_000) return (v / 1_000).toFixed(1).replace(/\.0$/, '') + 'k'
  return formatNumber(v, locale)
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

const localizedDateCache = new Map<string, Intl.DateTimeFormat>()
function localizedDate(tz: string, locale: LocaleCode, options: Intl.DateTimeFormatOptions): Intl.DateTimeFormat {
  const key = `${tz}|${locale}|${JSON.stringify(options)}`
  let f = localizedDateCache.get(key)
  if (!f) {
    f = new Intl.DateTimeFormat(locale, { timeZone: tz, ...options })
    localizedDateCache.set(key, f)
  }
  return f
}

export function formatTimestamp(iso: string, tz: string, locale?: LocaleCode): string {
  const d = new Date(iso)
  if (isNaN(d.getTime())) return iso || '-'
  if (locale) return localizedDate(tz, locale, { dateStyle: 'short', timeStyle: 'medium' }).format(d)
  const p = parts(d, tz)
  if (!p) return iso || '-'
  return `${p.y}-${pad(p.mo)}-${pad(p.d)} ${pad(p.h)}:${pad(p.mi)}:${pad(p.s)}`
}

export function formatTime(iso: string, tz: string, locale?: LocaleCode): string {
  const d = new Date(iso)
  if (isNaN(d.getTime())) return iso || '-'
  if (locale) return localizedDate(tz, locale, { timeStyle: 'medium' }).format(d)
  const p = parts(d, tz)
  if (!p) return iso || '-'
  return `${pad(p.h)}:${pad(p.mi)}:${pad(p.s)}`
}

export function formatDate(iso: string, tz: string, locale?: LocaleCode): string {
  const d = new Date(iso)
  if (isNaN(d.getTime())) return iso || '-'
  if (locale) return localizedDate(tz, locale, { dateStyle: 'medium' }).format(d)
  const p = parts(d, tz)
  if (!p) return iso || '-'
  return `${p.y}-${pad(p.mo)}-${pad(p.d)}`
}

export function formatBucket(bucket: string, tz: string, locale?: LocaleCode): string {
  // Buckets may be partial ('2024-01-15T10') or full ISO; try parse, fallback to raw
  const d = new Date(bucket.includes('T') && !bucket.endsWith('Z') && !bucket.includes('+') ? bucket + ':00Z' : bucket)
  const p = parts(d, tz)
  if (!p) return bucket || '-'
  // A date-only bucket has no time component to show.
  const dateOnly = /^\d{4}-\d{2}-\d{2}$/.test(bucket)
  if (locale) {
    return localizedDate(tz, locale, dateOnly ? { dateStyle: 'medium' } : { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' }).format(d)
  }
  if (dateOnly) return `${p.y}-${pad(p.mo)}-${pad(p.d)}`
  return `${pad(p.h)}:${pad(p.mi)}`
}

function pad(n: number): string {
  return String(n).padStart(2, '0')
}

/** Month and year for the calendar header, in the reader's language. */
export function formatMonthYear(date: Date, locale: LocaleCode): string {
  return new Intl.DateTimeFormat(locale, { month: 'long', year: 'numeric' }).format(date)
}

/** Weekday initials in the reader's language, starting on Sunday. */
export function weekdayInitials(locale: LocaleCode): string[] {
  // 2023-01-01 was a Sunday; the year is irrelevant, only the weekday order is.
  const format = new Intl.DateTimeFormat(locale, { weekday: 'narrow', timeZone: 'UTC' })
  return Array.from({ length: 7 }, (_, i) => format.format(new Date(Date.UTC(2023, 0, 1 + i))))
}
