import { useState } from 'react'
import { useSimulation, type SimulationRequest } from './api'
import { formatCurrency, formatCurrencyExact, formatNumber } from './format'
import { Card, CardHeader, Badge, Empty } from './ui/Card'
import { Button } from './ui/Button'
import { SortableTable, type Column } from './ui/SortableTable'
import type { SimulationRow } from './types'
import { useLocale } from './i18n/useLocale'

const FIELDS = [
  ['requests', 'common.requests'],
  ['prompt_tokens', 'models.prompt'],
  ['completion_tokens', 'models.completion'],
  ['cache_read_tokens', 'overview.cacheRead'],
  ['cache_write_tokens', 'sim.cacheWrite'],
] as const

/**
 * Prices one workload shape across the catalog. A simulation answers "what
 * would this cost elsewhere", so it never reads spend from the database.
 */
export default function SimulateView() {
  const { t, locale } = useLocale()
  const [form, setForm] = useState<Record<string, string>>({
    requests: '1',
    prompt_tokens: '1000',
    completion_tokens: '500',
    cache_read_tokens: '0',
    cache_write_tokens: '0',
  })
  const [input, setInput] = useState<SimulationRequest | null>(null)
  const { data, isFetching, isError } = useSimulation(input)

  const run = () => {
    const body: SimulationRequest = {
      requests: Number(form.requests) || 0,
      prompt_tokens: Number(form.prompt_tokens) || 0,
      completion_tokens: Number(form.completion_tokens) || 0,
      cache_read_tokens: Number(form.cache_read_tokens) || 0,
      cache_write_tokens: Number(form.cache_write_tokens) || 0,
      limit: 40,
    }
    if (body.requests > 0) setInput(body)
  }

  const columns: Column<SimulationRow>[] = [
    {
      key: 'model',
      header: t('common.model'),
      render: (r) => (
        <span className="flex items-center gap-2 font-medium text-[var(--text-strong)]">
          {r.model}
          {r.model === data?.baseline && <Badge tone="brand">{t('sim.baseline')}</Badge>}
          {!r.priced && <Badge tone="warn">{t('sim.unpriced')}</Badge>}
        </span>
      ),
      sortValue: (r) => r.model,
    },
    { key: 'input', header: t('prices.columnInput'), align: 'right', render: (r) => <span className="num tabular-nums">{formatCurrency(r.input_rate_per_1m, locale)}</span>, sortValue: (r) => r.input_rate_per_1m },
    { key: 'output', header: t('prices.columnOutput'), align: 'right', render: (r) => <span className="num tabular-nums">{formatCurrency(r.output_rate_per_1m, locale)}</span>, sortValue: (r) => r.output_rate_per_1m },
    { key: 'perRequest', header: t('sim.columnPerRequest'), align: 'right', render: (r) => <span className="num tabular-nums">{formatCurrencyExact(r.per_request_usd, locale)}</span>, sortValue: (r) => r.per_request_usd },
    { key: 'total', header: t('common.total'), align: 'right', render: (r) => <span className="num tabular-nums font-semibold text-[var(--brand-ink)]">{formatCurrency(r.total_usd, locale)}</span>, sortValue: (r) => r.total_usd },
    {
      key: 'delta',
      header: t('sim.columnDelta'),
      align: 'right',
      render: (r) => (
        <span className={`num tabular-nums ${r.delta_vs_baseline_usd > 0 ? 'text-[var(--danger-ink)]' : 'text-[var(--brand-ink)]'}`}>
          {r.delta_vs_baseline_usd === 0 ? '—' : `${r.delta_vs_baseline_usd > 0 ? '+' : ''}${formatCurrency(r.delta_vs_baseline_usd, locale)}`}
        </span>
      ),
      sortValue: (r) => r.delta_vs_baseline_usd,
    },
  ]

  return (
    <div className="space-y-5 p-5">
      <Card>
        <CardHeader title={t('nav.simulate')} subtitle={t('sim.subtitle')} />
        <div className="grid grid-cols-2 gap-3 px-5 pb-4 sm:grid-cols-3 lg:grid-cols-5">
          {FIELDS.map(([key, label]) => (
            <label key={key} className="flex flex-col gap-1">
              <span className="text-[11px] text-[var(--text-soft)]">{t(label)}</span>
              <input
                type="number"
                min="0"
                value={form[key]}
                onChange={(e) => setForm({ ...form, [key]: e.target.value })}
                className="rounded-md border border-[var(--border)] bg-[var(--bg)] px-2 py-1.5 text-[12px] focus:outline-none focus:ring-2 focus:ring-[var(--brand)]"
              />
            </label>
          ))}
        </div>
        <div className="px-5 pb-5">
          <Button variant="primary" onClick={run} disabled={isFetching}>
            {isFetching ? t('common.loading') : t('sim.run')}
          </Button>
        </div>
      </Card>

      {isError && <Card><Empty label={t('sim.failed')} hint={t('error.prices.unreadable')} /></Card>}
      {!input && !isError && <Card><Empty label={t('sim.empty')} hint={t('sim.subtitle')} /></Card>}
      {data && data.rows.length > 0 && (
        <Card>
          <CardHeader
            title={t('common.model')}
            subtitle={`${t('sim.baseline')}: ${data.baseline}`}
            right={<Badge tone="brand">{t('overview.totalRequests')} · {formatNumber(input?.requests ?? 0, locale)}</Badge>}
          />
          <SortableTable rows={data.rows} rowKey={(r) => r.model} columns={columns} pageSize={20} searchable searchKeys={['model']} />
        </Card>
      )}
    </div>
  )
}
