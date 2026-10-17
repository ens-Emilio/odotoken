import { useStats } from './api'
import { formatNumber, formatCurrency } from './format'
import { Card, CardHeader, Badge } from './ui/Card'

export default function GuardView() {
  const { data, isLoading } = useStats()

  if (isLoading) return <div className="p-5"><div className="h-48 animate-pulse rounded-[var(--radius)] bg-[var(--bg-fill-tertiary)]" /></div>
  if (!data || !data.guard.limits) return null

  const { guard } = data
  const tone = guard.status === 'breached' ? 'danger' : guard.status === 'warning' ? 'warn' : 'brand'
  const cur = guard.current ?? { daily_cost_usd: 0, monthly_cost_usd: 0, daily_tokens: 0, monthly_tokens: 0 }
  // 0 limit means disabled in the backend (falsy check) — display as ∞
  const nz = (v: number | null) => (v ? v : null)
  const limits = {
    daily_cost_limit_usd: nz(guard.limits.daily_cost_limit_usd),
    monthly_cost_limit_usd: nz(guard.limits.monthly_cost_limit_usd),
    daily_tokens_limit: nz(guard.limits.daily_tokens_limit),
    monthly_tokens_limit: nz(guard.limits.monthly_tokens_limit),
    action_on_breach: guard.limits.action_on_breach,
  }

  const limitRows: [string, number | null, number, 'usd' | 'num'][] = [
    ['Daily Cost', limits.daily_cost_limit_usd, cur.daily_cost_usd, 'usd'],
    ['Monthly Cost', limits.monthly_cost_limit_usd, cur.monthly_cost_usd, 'usd'],
    ['Daily Tokens', limits.daily_tokens_limit, cur.daily_tokens, 'num'],
    ['Monthly Tokens', limits.monthly_tokens_limit, cur.monthly_tokens, 'num'],
  ]

  return (
    <div className="space-y-5 p-5">
      {guard.status !== 'ok' && (
        <div className={`flex items-center gap-2 rounded-lg border border-transparent px-4 py-2.5 text-[12px] ${
          guard.status === 'breached'
            ? 'bg-[var(--danger-soft)] text-[var(--danger-ink)]'
            : 'bg-[var(--warn-soft)] text-[var(--warn-ink)]'
        }`}>
          <Badge tone={tone}>{guard.status}</Badge>
          <span>{guard.reason || 'Budget threshold reached'}</span>
        </div>
      )}

      <div className="grid grid-cols-1 gap-5 lg:grid-cols-2">
        <Card>
          <CardHeader title="Limits & Usage" subtitle="null limit = disabled (∞)" />
          <div className="space-y-3 px-5 pb-5 text-[12px]">
            {limitRows.map(([label, limit, used, kind]) => {
              const usedStr = kind === 'usd' ? formatCurrency(used) : formatNumber(used)
              const limitStr = limit === null ? '∞' : kind === 'usd' ? formatCurrency(limit) : formatNumber(limit)
              const pct = limit ? Math.min(100, (used / limit) * 100) : 0
              const barTone = pct >= 100 ? 'var(--danger)' : pct >= 80 ? 'var(--warn)' : 'var(--brand)'
              return (
                <div key={label}>
                  <div className="mb-1 flex items-center justify-between">
                    <span className="text-[var(--text-muted)]">{label}</span>
                    <span className="num text-[var(--text-strong)]">{usedStr} / {limitStr}{limit !== null ? ` · ${Math.round(pct)}%` : ''}</span>
                  </div>
                  {limit !== null && (
                    <div className="h-1.5 overflow-hidden rounded-full bg-[var(--bg-fill-secondary)]">
                      <div className="h-full rounded-full transition-all" style={{ width: `${pct}%`, background: barTone }} />
                    </div>
                  )}
                </div>
              )
            })}
          </div>
        </Card>

        <Card>
          <CardHeader title="Guard Configuration" subtitle="from config.json → limits" />
          <div className="space-y-2 px-5 pb-5 text-[12px]">
            <div className="flex items-center justify-between">
              <span className="text-[var(--text-muted)]">Status</span>
              <Badge tone={tone}>{guard.status}</Badge>
            </div>
            <div className="flex items-center justify-between">
              <span className="text-[var(--text-muted)]">Blocking requests</span>
              <Badge tone={guard.allowed ? 'brand' : 'danger'}>{guard.allowed ? 'no' : 'YES'}</Badge>
            </div>
            <div className="flex items-center justify-between">
              <span className="text-[var(--text-muted)]">Action on breach</span>
              <Badge>{limits.action_on_breach}</Badge>
            </div>
            <div className="flex items-start justify-between gap-4">
              <span className="text-[var(--text-muted)]">Reason</span>
              <span className="text-right text-[var(--text-strong)]">{guard.reason || '—'}</span>
            </div>
          </div>
        </Card>
      </div>
    </div>
  )
}