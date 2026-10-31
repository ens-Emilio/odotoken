import { useMemo } from 'react'
import { useCacheSavings } from './api'
import { formatCurrency, formatNumber, sinceForFilter } from './format'
import { Card, CardHeader, Badge, Empty } from './ui/Card'
import { SortableTable, type Column } from './ui/SortableTable'
import { useTimezone } from './context/useTimezone'
import { useLocale } from './i18n/useLocale'

type Row = { model: string; provider: string; requests: number; cache_read_tokens: number; cache_write_tokens: number; net_saved_usd: number; discount_pct: number; priced: boolean }

/**
 * The cache KPI answers one question: how much cheaper was this window because
 * of the cache. A cache write billed above the input rate is subtracted, so a
 * model whose cache costs more shows a negative saving instead of a flattering
 * token count.
 */
export default function CacheView({ timeFilter, project, source, tag }: { timeFilter: string; project: string; source: string; tag: string }) {
  const { timezone } = useTimezone()
  const { t, locale } = useLocale()
  const since = useMemo(() => sinceForFilter(timeFilter, timezone), [timeFilter, timezone])
  const { data, isLoading, isError, refetch } = useCacheSavings(since, project, source, tag)

  if (isLoading) return <div className="p-5"><div className="h-64 animate-pulse rounded-[var(--radius)] bg-[var(--bg-fill-tertiary)]" /></div>
  if (isError || !data) return <div className="p-5"><Card><Empty label={t('error.cache.unavailable')} hint={t('models.changeRange')} onRetry={() => { void refetch() }} /></Card></div>
  if (data.by_model.length === 0) return <div className="p-5"><Card><Empty label={t('cache.empty')} hint={t('models.changeRange')} /></Card></div>

  const kpis: [string, string, string?][] = [
    [t('cache.netSaved'), formatCurrency(data.net_saved_usd, locale), data.net_saved_usd >= 0 ? 'var(--brand-ink)' : 'var(--danger-ink)'],
    [t('cache.discount'), `${data.discount_pct}%`],
    [t('cache.actual'), formatCurrency(data.actual_cost_usd, locale)],
    [t('cache.uncached'), formatCurrency(data.uncached_cost_usd, locale)],
    [t('overview.cachedTokens'), formatNumber(data.cache_read_tokens, locale)],
  ]

  const columns: Column<Row>[] = [
    { key: 'model', header: t('common.model'), render: (r) => <span className="flex items-center gap-2 font-medium text-[var(--text-strong)]">{r.model || '—'}{!r.priced && <Badge tone="warn">{t('models.noCatalogPrice')}</Badge>}</span>, sortValue: (r) => r.model },
    { key: 'provider', header: t('common.provider'), render: (r) => <Badge>{r.provider}</Badge>, sortValue: (r) => r.provider },
    { key: 'requests', header: t('common.requests'), align: 'right', render: (r) => <span className="num tabular-nums">{formatNumber(r.requests, locale)}</span>, sortValue: (r) => r.requests },
    { key: 'read', header: t('overview.cacheRead'), align: 'right', render: (r) => <span className="num tabular-nums text-[var(--text-soft)]">{formatNumber(r.cache_read_tokens, locale)}</span>, sortValue: (r) => r.cache_read_tokens },
    { key: 'write', header: t('events.cacheWrite'), align: 'right', render: (r) => <span className="num tabular-nums text-[var(--text-soft)]">{formatNumber(r.cache_write_tokens, locale)}</span>, sortValue: (r) => r.cache_write_tokens },
    { key: 'saved', header: t('cache.netSaved'), align: 'right', render: (r) => <span className={`num tabular-nums font-semibold ${r.net_saved_usd < 0 ? 'text-[var(--danger-ink)]' : 'text-[var(--brand-ink)]'}`}>{formatCurrency(r.net_saved_usd, locale)}</span>, sortValue: (r) => r.net_saved_usd },
    { key: 'discount', header: t('cache.discount'), align: 'right', render: (r) => <span className="num tabular-nums">{r.priced ? `${r.discount_pct}%` : '—'}</span>, sortValue: (r) => r.discount_pct },
  ]

  return (
    <div className="space-y-5 p-5">
      <Card>
        <CardHeader title={t('nav.cache')} subtitle={t('cache.subtitle')} right={data.unpriced_models > 0 ? <Badge tone="warn">{t('models.unpriced', { count: data.unpriced_models })}</Badge> : undefined} />
        <div className="grid grid-cols-2 gap-3 px-5 pb-5 sm:grid-cols-3 lg:grid-cols-5">
          {kpis.map(([label, value, color]) => (
            <div key={label} className="rounded-md border border-[var(--border-soft)] bg-[var(--bg-fill-tertiary)] px-3 py-2.5">
              <div className="text-[10px] uppercase tracking-wider text-[var(--text-soft)]">{label}</div>
              <div className="num mt-0.5 text-[15px] font-semibold" style={color ? { color } : undefined}>{value}</div>
            </div>
          ))}
        </div>
      </Card>
      <Card>
        <CardHeader title={t('cache.byModel')} subtitle={t('models.clickHeaders')} />
        <SortableTable rows={data.by_model} rowKey={(r) => r.model + r.provider} columns={columns} pageSize={25} searchable searchKeys={['model', 'provider']} />
      </Card>
    </div>
  )
}
