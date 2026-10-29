import { useMemo } from 'react'
import { useStats } from './api'
import { formatNumber, formatCurrency, sinceForFilter } from './format'
import { Card, CardHeader, Badge, Empty } from './ui/Card'
import { SortableTable, type Column } from './ui/SortableTable'
import type { ClientRow } from './types'
import { useTimezone } from './context/useTimezone'
import { useLocale } from './i18n/useLocale'

export default function ClientsView({ timeFilter, project, source, tag }: { timeFilter: string; project: string; source: string; tag: string }) {
  const { timezone } = useTimezone()
  const { t, locale } = useLocale()
  const since = useMemo(() => sinceForFilter(timeFilter, timezone), [timeFilter, timezone])
  const { data, isLoading, isError, refetch } = useStats(since, project, source, tag, timezone)

  if (isLoading) return <div className="p-5"><div className="h-64 animate-pulse rounded-[var(--radius)] bg-[var(--bg-fill-tertiary)]" /></div>
  if (isError || !data) return <div className="p-5"><Card><Empty label={t('clients.loadFailed')} hint={t('error.stats.unavailable')} onRetry={() => { void refetch() }} /></Card></div>
  if (data.by_client.length === 0) return <div className="p-5"><Card><Empty label={t('clients.empty')} hint={t('models.changeRange')} /></Card></div>

  const columns: Column<ClientRow>[] = [
    { key: 'app', header: t('clients.clientApp'), render: (c) => <Badge>{c.client_app}</Badge>, sortValue: (c) => c.client_app },
    { key: 'requests', header: t('common.requests'), align: 'right', render: (c) => <span className="num tabular-nums">{formatNumber(c.requests, locale)}</span>, sortValue: (c) => c.requests },
    { key: 'total', header: t('events.totalTokens'), align: 'right', render: (c) => <span className="num tabular-nums font-semibold">{formatNumber(c.total_tokens, locale)}</span>, sortValue: (c) => c.total_tokens },
    { key: 'cost', header: t('models.costUsd'), align: 'right', render: (c) => <span className="num tabular-nums text-[var(--brand-ink)]">{formatCurrency(c.cost_usd, locale)}</span>, sortValue: (c) => c.cost_usd },
  ]

  return (
    <div className="p-5">
      <Card>
        <CardHeader
          title={t('clients.usageByClient')}
          subtitle={t('clients.agentsSeen', { count: data.by_client.length })}
        />
        <SortableTable rows={data.by_client} rowKey={(c) => c.client_app} columns={columns} pageSize={25} searchable searchKeys={['client_app']} />
      </Card>
    </div>
  )
}