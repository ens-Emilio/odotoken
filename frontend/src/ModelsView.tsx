import { useMemo } from 'react'
import { useStats } from './api'
import { formatNumber, formatCurrency, sinceForFilter } from './format'
import { Card, CardHeader, Badge, Empty } from './ui/Card'
import { SortableTable, type Column } from './ui/SortableTable'
import type { ModelRow } from './types'
import { useTimezone } from './context/useTimezone'
import { useLocale } from './i18n/useLocale'

export default function ModelsView({ timeFilter, project, source, tag }: { timeFilter: string; project: string; source: string; tag: string }) {
  const { timezone } = useTimezone()
  const { t, locale } = useLocale()
  const since = useMemo(() => sinceForFilter(timeFilter, timezone), [timeFilter, timezone])
  const { data, isLoading, isError, refetch } = useStats(since, project, source, tag, timezone)

  if (isLoading) return <div className="p-5"><div className="h-64 animate-pulse rounded-[var(--radius)] bg-[var(--bg-fill-tertiary)]" /></div>
  if (isError || !data) return <div className="p-5"><Card><Empty label={t('models.loadFailed')} hint={t('error.stats.unavailable')} onRetry={() => { void refetch() }} /></Card></div>
  if (data.by_model.length === 0) return <div className="p-5"><Card><Empty label={t('models.empty')} hint={t('models.changeRange')} /></Card></div>

  const columns: Column<ModelRow>[] = [
    { key: 'model', header: t('models.headerModel'), render: (m) => <span className="flex items-center gap-2 font-medium text-[var(--text-strong)]">{m.model || '—'}{!!m.price_missing_count && <Badge tone="warn" title={t('models.noCatalogPrice')}>{t('models.unpriced', { count: m.price_missing_count })}</Badge>}</span>, sortValue: (m) => m.model },
    { key: 'provider', header: t('common.provider'), render: (m) => <Badge>{m.provider}</Badge>, sortValue: (m) => m.provider },
    { key: 'requests', header: t('common.requests'), align: 'right', render: (m) => <span className="num tabular-nums">{formatNumber(m.requests, locale)}</span>, sortValue: (m) => m.requests },
    { key: 'prompt', header: t('models.prompt'), align: 'right', render: (m) => <span className="num tabular-nums">{formatNumber(m.prompt_tokens, locale)}</span>, sortValue: (m) => m.prompt_tokens },
    { key: 'completion', header: t('models.completion'), align: 'right', render: (m) => <span className="num tabular-nums">{formatNumber(m.completion_tokens, locale)}</span>, sortValue: (m) => m.completion_tokens },
    { key: 'cache', header: t('models.cacheRead'), align: 'right', render: (m) => <span className="num tabular-nums text-[var(--text-soft)]">{formatNumber(m.cache_read_tokens, locale)}</span>, sortValue: (m) => m.cache_read_tokens },
    { key: 'total', header: t('common.total'), align: 'right', render: (m) => <span className="num tabular-nums font-semibold">{formatNumber(m.total_tokens, locale)}</span>, sortValue: (m) => m.total_tokens },
    { key: 'cost', header: t('models.costUsd'), align: 'right', render: (m) => <span className="num tabular-nums text-[var(--brand-ink)]">{formatCurrency(m.cost_usd, locale)}</span>, sortValue: (m) => m.cost_usd },
  ]

  return (
    <div className="p-5">
      <Card>
        <CardHeader
          title={t('models.breakdown')}
          subtitle={`${t('models.seen', { count: data.by_model.length })} · ${t('models.clickHeaders')}`}
        />
        <SortableTable rows={data.by_model} rowKey={(m) => m.model + m.provider} columns={columns} pageSize={25} searchable searchKeys={['model','provider']} />
      </Card>
    </div>
  )
}