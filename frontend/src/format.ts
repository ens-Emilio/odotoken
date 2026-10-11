export function formatNumber(n: number | undefined | null): string {
  return new Intl.NumberFormat().format(n ?? 0)
}

export function formatCurrency(n: number | undefined | null): string {
  return '$' + (n ?? 0).toFixed(4)
}

export function formatCompact(n: number): string {
  if (n >= 1_000_000) return (n / 1_000_000).toFixed(1) + 'M'
  if (n >= 1_000) return (n / 1_000).toFixed(1) + 'k'
  return String(n)
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