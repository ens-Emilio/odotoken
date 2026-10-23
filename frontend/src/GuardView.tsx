import { useState } from 'react'
import { useStats } from './api'
import { formatNumber, formatCurrency } from './format'
import { Card, CardHeader, Badge } from './ui/Card'
import { toast } from './ui/Toast'

export default function GuardView() {
  const { data, isLoading } = useStats()
  if (isLoading) return <div className="p-5"><div className="h-48 animate-pulse rounded-[var(--radius)] bg-[var(--bg-fill-tertiary)]" /></div>
 if (!data) return <div className="p-5"><div className="h-48 animate-pulse rounded-[var(--radius)] bg-[var(--bg-fill-tertiary)]" /></div>
 if (!data.guard) return <div className="p-5 text-center text-[12px] text-[var(--text-soft)]">No spending limits configured — unlimited usage</div>
 const { guard } = data
 if (!guard.limits) return <div className="p-5 text-center text-[12px] text-[var(--text-soft)]">Guard not configured — add <code>limits</code> to config.json</div>
 const cur = guard.current ?? { daily_cost_usd: 0, monthly_cost_usd: 0, daily_tokens: 0, monthly_tokens: 0 }
 const tone = guard.status === 'breached' ? 'danger' : guard.status === 'warning' ? 'warn' : 'brand'
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
        <div className={`flex items-center gap-2 rounded-lg border border-transparent px-4 py-2.5 text-[12px] ${guard.status === 'breached' ? 'bg-[var(--danger-soft)] text-[var(--danger-ink)]' : 'bg-[var(--warn-soft)] text-[var(--warn-ink)]'}`}>
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
          <CardHeader title="Guard Configuration" subtitle="limits — PATCH /api/guard/limits → config.json (supports webhook_url)" />
          <GuardEditor limits={limits} />
          <div className="space-y-2 px-5 pb-3 text-[11px] leading-relaxed text-[var(--text-soft)]">
            <div className="flex items-center justify-between text-[12px]"><span className="text-[var(--text-muted)]">Blocking</span><Badge tone={guard.allowed ? 'brand' : 'danger'}>{guard.allowed ? 'no' : 'YES'}</Badge></div>
            <div className="flex items-start justify-between gap-4 text-[12px]"><span className="text-[var(--text-muted)]">Reason</span><span className="text-right text-[var(--text-strong)]">{guard.reason || '—'}</span></div>
            <p>Edits persist to <code>config.json</code> and hot-reload the guard. Add <code>webhook_url</code> to limits to fire HTTP callbacks on breach.</p>
          </div>
        </Card>
      </div>
    </div>
  )
}


function GuardEditor({ limits }: { limits: any }) {
  const [form, setForm] = useState({
    daily_cost_limit_usd: limits.daily_cost_limit_usd ?? '',
    monthly_cost_limit_usd: limits.monthly_cost_limit_usd ?? '',
    daily_tokens_limit: limits.daily_tokens_limit ?? '',
    monthly_tokens_limit: limits.monthly_tokens_limit ?? '',
    action_on_breach: limits.action_on_breach ?? 'alert',
  })
  const [saving, setSaving] = useState(false)
  const save = async () => {
    setSaving(true)
    const body: any = {}
    for (const k of ['daily_cost_limit_usd','monthly_cost_limit_usd','daily_tokens_limit','monthly_tokens_limit'] as const) {
      const v = form[k]
      body[k] = v === '' ? null : Number(v)
    }
    body.action_on_breach = form.action_on_breach
    const res = await fetch('/api/guard/limits', { method: 'PATCH', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) })
    setSaving(false)
    if (res.ok) toast('ok', 'Guard limits saved — config.json updated')
  }
  return (
    <div className="space-y-3 px-5 pb-2">
      <div className="grid grid-cols-2 gap-3">
        {([
          ['daily_cost_limit_usd','Daily $'],
          ['monthly_cost_limit_usd','Monthly $'],
          ['daily_tokens_limit','Daily tokens'],
          ['monthly_tokens_limit','Monthly tokens'],
        ] as const).map(([k,label]) => (
          <label key={k} className="flex flex-col gap-1">
            <span className="text-[11px] text-[var(--text-soft)]">{label} (0/null = ∞)</span>
            <input type="number" value={form[k]} onChange={(e) => setForm({ ...form, [k]: e.target.value })} placeholder="∞" className="rounded-md border border-[var(--border)] bg-[var(--bg)] px-2 py-1.5 text-[12px] focus:outline-none focus:ring-2 focus:ring-[var(--brand)]" />
          </label>
        ))}
      </div>
      <label className="flex flex-col gap-1">
        <span className="text-[11px] text-[var(--text-soft)]">Action on breach</span>
        <select value={form.action_on_breach} onChange={(e) => setForm({ ...form, action_on_breach: e.target.value })} className="max-w-[200px] rounded-md border border-[var(--border)] bg-[var(--bg)] px-2 py-1.5 text-[12px] focus:outline-none focus:ring-2 focus:ring-[var(--brand)]">
          <option value="alert">alert</option>
          <option value="block">block</option>
        </select>
      </label>
      <button onClick={save} disabled={saving} className="rounded-md bg-[var(--brand)] px-3 py-1.5 text-[12px] font-medium text-white hover:opacity-90 disabled:opacity-50">{saving ? 'Saving…' : 'Save limits'}</button>
    </div>
  )
}
