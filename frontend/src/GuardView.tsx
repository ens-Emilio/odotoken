import { useLocale } from './i18n/useLocale'
import { guardEventReason, guardStatusReason } from './guard-reason'
import { useState } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { useGuardStatus, useStats } from './api'
import { formatNumber, formatCurrency, formatTimestamp } from './format'
import { useTimezone } from './context/useTimezone'
import { Card, CardHeader, Badge } from './ui/Card'
import { toast } from './ui/toast-store'

export default function GuardView() {
  const { data, isLoading } = useStats()
  const { timezone } = useTimezone()
  const { t, locale } = useLocale()
  const { data: guardStatus } = useGuardStatus()
  if (isLoading) return <div className="p-5"><div className="h-48 animate-pulse rounded-[var(--radius)] bg-[var(--bg-fill-tertiary)]" /></div>
  if (!data) return <div className="p-5"><div className="h-48 animate-pulse rounded-[var(--radius)] bg-[var(--bg-fill-tertiary)]" /></div>
  if (!data.guard) return <div className="p-5 text-center text-[12px] text-[var(--text-soft)]">{t('guard.noLimits')}</div>
  const { guard } = data
  if (!guard.limits) return <div className="p-5 text-center text-[12px] text-[var(--text-soft)]">{t('guard.notConfigured', { field: 'limits' })}</div>
  const cur = guard.current ?? { daily_cost_usd: 0, monthly_cost_usd: 0, daily_tokens: 0, monthly_tokens: 0 }
  const tone = guard.status === 'breached' ? 'danger' : guard.status === 'warning' ? 'warn' : 'brand'
  const nz = (v: number | null) => (v ? v : null)
  const limits: GuardLimits = {
    daily_cost_limit_usd: nz(guard.limits.daily_cost_limit_usd),
    monthly_cost_limit_usd: nz(guard.limits.monthly_cost_limit_usd),
    daily_tokens_limit: nz(guard.limits.daily_tokens_limit),
    monthly_tokens_limit: nz(guard.limits.monthly_tokens_limit),
    warning_threshold_pct: nz(guard.limits.warning_threshold_pct),
    action_on_breach: guard.limits.action_on_breach,
    timezone: guard.limits.timezone ?? null,
  }
  const limitRows: [string, number | null, number, 'usd' | 'num'][] = [
    [t('guard.dailyCost'), limits.daily_cost_limit_usd, cur.daily_cost_usd, 'usd'],
    [t('guard.monthlyCostLimit'), limits.monthly_cost_limit_usd, cur.monthly_cost_usd, 'usd'],
    [t('guard.dailyTokensLimit'), limits.daily_tokens_limit, cur.daily_tokens, 'num'],
    [t('guard.monthlyTokensLimit'), limits.monthly_tokens_limit, cur.monthly_tokens, 'num'],
  ]
  return (
    <div className="space-y-5 p-5">
      {guard.status !== 'ok' && (
        <div className={`flex items-center gap-2 rounded-lg border border-transparent px-4 py-2.5 text-[12px] ${guard.status === 'breached' ? 'bg-[var(--danger-soft)] text-[var(--danger-ink)]' : 'bg-[var(--warn-soft)] text-[var(--warn-ink)]'}`}>
          <Badge tone={tone}>{guard.status}</Badge>
          <span>{guardStatusReason(t, guardStatus) || t('guard.thresholdReached')}</span>
        </div>
      )}
      <div className="grid grid-cols-1 gap-5 lg:grid-cols-2">
        <Card>
          <CardHeader title={t('guard.limitsUsage')} subtitle={t('guard.nullLimit')} />
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
          <CardHeader title={t('guard.configuration')} subtitle={t('guard.configurationSub')} />
          <GuardEditor limits={limits} />
          <div className="space-y-2 px-5 pb-3 text-[11px] leading-relaxed text-[var(--text-soft)]">
            <div className="flex items-center justify-between text-[12px]"><span className="text-[var(--text-muted)]">{t('guard.blocking')}</span><Badge tone={guard.limits.action_on_breach === 'block' ? 'danger' : 'brand'}>{guard.limits.action_on_breach === 'block' ? t('guard.enabled') : t('guard.alertsOnly')}</Badge></div>
            <div className="flex items-start justify-between gap-4 text-[12px]"><span className="text-[var(--text-muted)]">{t('common.reason')}</span><span className="text-end text-[var(--text-strong)]">{guard.reason || '—'}</span></div>
            <p>{t('guard.editsPersist', { file: 'config.json', webhook: 'webhook_url' })}</p>
          </div>
        </Card>
      </div>
      <Card>
        <CardHeader title={t('guard.history')} subtitle={t('guard.historySub')} />
        {!guardStatus?.history?.length ? (
          <div className="px-5 pb-4 text-[12px] text-[var(--text-faint)]">{t('guard.noTransitions')}</div>
        ) : (
          <div className="overflow-x-auto px-2 pb-3">
            <table className="w-full text-start text-[12px]">
              <thead>
                <tr className="border-b border-[var(--border)] text-[11px] uppercase tracking-wider text-[var(--text-faint)]">
                  <th scope="col" className="py-2 ps-2 pe-3 font-semibold">{t('common.time')}</th>
                  <th scope="col" className="py-2 px-3 font-semibold">{t('common.status')}</th>
                  <th scope="col" className="py-2 px-3 font-semibold">{t('common.reason')}</th>
                  <th scope="col" className="py-2 ps-3 pe-2 text-end font-semibold">{t('guard.dailyCost')}</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-[var(--border)]">
                {guardStatus.history.map((event) => (
                  <tr key={event.id}>
                    <td className="py-2 ps-2 pe-3 num text-[var(--text-soft)]">{formatTimestamp(event.timestamp, timezone, locale)}</td>
                    <td className="py-2 px-3"><Badge tone={event.status === 'breached' ? 'danger' : 'warn'}>{event.status}</Badge></td>
                    <td className="py-2 px-3 text-[var(--text-strong)]">{guardEventReason(t, event)}</td>
                    <td className="py-2 ps-3 pe-2 text-end num">{formatCurrency(event.daily_cost_usd, locale)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </Card>
    </div>
  )
}

type GuardLimits = {
  daily_cost_limit_usd: number | null
  monthly_cost_limit_usd: number | null
  daily_tokens_limit: number | null
  monthly_tokens_limit: number | null
  warning_threshold_pct: number | null
  action_on_breach: string
  timezone: string | null
}


function GuardEditor({ limits }: { limits: GuardLimits }) {
  const { t } = useLocale()
  const queryClient = useQueryClient()
  const [form, setForm] = useState({
    daily_cost_limit_usd: limits.daily_cost_limit_usd ?? '',
    monthly_cost_limit_usd: limits.monthly_cost_limit_usd ?? '',
    daily_tokens_limit: limits.daily_tokens_limit ?? '',
    monthly_tokens_limit: limits.monthly_tokens_limit ?? '',
    warning_threshold_pct: limits.warning_threshold_pct ?? '',
    action_on_breach: limits.action_on_breach ?? 'block',
    timezone: limits.timezone ?? '',
  })
  const [saving, setSaving] = useState(false)
  const save = async () => {
    setSaving(true)
    const body: Record<string, number | string | null> = {}
    for (const k of ['daily_cost_limit_usd','monthly_cost_limit_usd','daily_tokens_limit','monthly_tokens_limit'] as const) {
      const v = form[k]
      body[k] = v === '' ? null : Number(v)
    }
    body.warning_threshold_pct = form.warning_threshold_pct === '' ? null : Number(form.warning_threshold_pct)
    body.action_on_breach = form.action_on_breach
    body.timezone = form.timezone || null
    try {
      const res = await fetch('/api/guard/limits', {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
      })
      const payload = await res.json().catch(() => null) as { error?: string; code?: string; params?: Record<string, string | number> } | null
      if (!res.ok) {
        toast('error', payload?.code ? t(payload.code, payload.params) : payload?.error || t('error.ui.unknown'))
        return
      }
      await queryClient.invalidateQueries({ queryKey: ['stats'] })
      toast('ok', t('guard.saved'))
    } catch (err) {
      toast('error', err instanceof Error ? `Could not save guard limits: ${err.message}` : 'Could not save guard limits')
    } finally {
      setSaving(false)
    }
  }
  return (
    <div className="space-y-3 px-5 pb-2">
      <div className="grid grid-cols-2 gap-3">
        {([
          ['daily_cost_limit_usd',t('guard.dailyCostLimit')],
          ['monthly_cost_limit_usd',t('guard.monthlyCostLimit')],
          ['daily_tokens_limit',t('guard.dailyTokensLimit')],
          ['monthly_tokens_limit',t('guard.monthlyTokensLimit')],
        ] as const).map(([k,label]) => (
          <label key={k} className="flex flex-col gap-1">
            <span className="text-[11px] text-[var(--text-soft)]">{label} (empty = ∞)</span>
            <input type="number" min="0" step="any" value={form[k]} onChange={(e) => setForm({ ...form, [k]: e.target.value })} placeholder="∞" className="rounded-md border border-[var(--border)] bg-[var(--bg)] px-2 py-1.5 text-[12px] focus:outline-none focus:ring-2 focus:ring-[var(--brand)]" />
          </label>
        ))}
      </div>
      <label className="flex flex-col gap-1">
        <span className="text-[11px] text-[var(--text-soft)]">{t('guard.warningThreshold')}</span>
        <input type="number" min="0" max="100" value={form.warning_threshold_pct} onChange={(e) => setForm({ ...form, warning_threshold_pct: e.target.value })} placeholder="80" className="max-w-[200px] rounded-md border border-[var(--border)] bg-[var(--bg)] px-2 py-1.5 text-[12px] focus:outline-none focus:ring-2 focus:ring-[var(--brand)]" />
      </label>
      <label className="flex flex-col gap-1">
        <span className="text-[11px] text-[var(--text-soft)]">{t('guard.actionOnBreach')}</span>
        <select value={form.action_on_breach} onChange={(e) => setForm({ ...form, action_on_breach: e.target.value })} className="max-w-[200px] rounded-md border border-[var(--border)] bg-[var(--bg)] px-2 py-1.5 text-[12px] focus:outline-none focus:ring-2 focus:ring-[var(--brand)]">
          <option value="alert">alert</option>
          <option value="block">block</option>
        </select>
      </label>
      <label className="flex flex-col gap-1">
        <span className="text-[11px] text-[var(--text-soft)]">{t('guard.timezone')}</span>
        <input type="text" value={form.timezone} onChange={(e) => setForm({ ...form, timezone: e.target.value })} placeholder="America/Rio_Branco" className="max-w-[300px] rounded-md border border-[var(--border)] bg-[var(--bg)] px-2 py-1.5 text-[12px] focus:outline-none focus:ring-2 focus:ring-[var(--brand)]" />
      </label>
      <button onClick={save} disabled={saving} className="rounded-md bg-[var(--brand)] px-3 py-1.5 text-[12px] font-medium text-white hover:opacity-90 disabled:opacity-50">{saving ? t('common.saving') : t('guard.saveLimits')}</button>
    </div>
  )
}
