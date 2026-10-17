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

export function sinceForFilter(filter: string): string | undefined {
  const now = new Date()
  if (filter === 'today') {
    return now.toISOString().slice(0, 10) + 'T00:00:00'
  }
  if (filter === '7d') return new Date(now.getTime() - 7 * 86400000).toISOString()
  if (filter === '30d') return new Date(now.getTime() - 30 * 86400000).toISOString()
  return undefined
}
