import { useMemo } from 'react'
import { useStats } from './api'
import { formatNumber, formatCurrency, sinceForFilter } from './format'
import { Card, CardHeader, Badge, Empty } from './ui/Card'
import { SortableTable, type Column } from './ui/SortableTable'
import type { ModelRow } from './types'
import { useTimezone } from './context/useTimezone'

export default function ModelsView({ timeFilter, project, source, tag }: { timeFilter: string; project: string; source: string; tag: string }) {
  const { timezone } = useTimezone()
  const since = useMemo(() => sinceForFilter(timeFilter, timezone), [timeFilter, timezone])
  const { data, isLoading, isError, refetch } = useStats(since, project, source, tag, timezone)

  if (isLoading) return <div className="p-5"><div className="h-64 animate-pulse rounded-[var(--radius)] bg-[var(--bg-fill-tertiary)]" /></div>
  if (isError || !data) return <div className="p-5"><Card><Empty label="Could not load model usage." hint="The stats service did not respond." onRetry={() => { void refetch() }} /></Card></div>
  if (data.by_model.length === 0) return <div className="p-5"><Card><Empty label="No model usage in this period." hint="Change the time range or send a proxied request." /></Card></div>

  const columns: Column<ModelRow>[] = [
    { key: 'model', header: 'Model', render: (m) => <span className="flex items-center gap-2 font-medium text-[var(--text-strong)]">{m.model || '—'}{!!m.price_missing_count && <Badge tone="warn" title="No catalog price; cost shown as zero">{m.price_missing_count} unpriced</Badge>}</span>, sortValue: (m) => m.model },
    { key: 'provider', header: 'Provider', render: (m) => <Badge>{m.provider}</Badge>, sortValue: (m) => m.provider },
    { key: 'requests', header: 'Requests', align: 'right', render: (m) => <span className="num tabular-nums">{formatNumber(m.requests)}</span>, sortValue: (m) => m.requests },
    { key: 'prompt', header: 'Prompt', align: 'right', render: (m) => <span className="num tabular-nums">{formatNumber(m.prompt_tokens)}</span>, sortValue: (m) => m.prompt_tokens },
    { key: 'completion', header: 'Completion', align: 'right', render: (m) => <span className="num tabular-nums">{formatNumber(m.completion_tokens)}</span>, sortValue: (m) => m.completion_tokens },
    { key: 'cache', header: 'Cache R', align: 'right', render: (m) => <span className="num tabular-nums text-[var(--text-soft)]">{formatNumber(m.cache_read_tokens)}</span>, sortValue: (m) => m.cache_read_tokens },
    { key: 'total', header: 'Total', align: 'right', render: (m) => <span className="num tabular-nums font-semibold">{formatNumber(m.total_tokens)}</span>, sortValue: (m) => m.total_tokens },
    { key: 'cost', header: 'Cost (USD)', align: 'right', render: (m) => <span className="num tabular-nums text-[var(--brand-ink)]">{formatCurrency(m.cost_usd)}</span>, sortValue: (m) => m.cost_usd },
  ]

  return (
    <div className="p-5">
      <Card>
        <CardHeader
          title="Model Breakdown"
          subtitle={`${data.by_model.length} models seen in this period · click headers to sort`}
        />
        <SortableTable rows={data.by_model} rowKey={(m) => m.model + m.provider} columns={columns} pageSize={25} searchable searchKeys={['model','provider']} />
      </Card>
    </div>
  )
}