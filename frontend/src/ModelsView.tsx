import { useStats } from './api'
import { formatNumber, formatCurrency, sinceForFilter } from './format'
import { Card, CardHeader, Badge } from './ui/Card'
import { SortableTable, type Column } from './ui/SortableTable'
import type { ModelRow } from './types'

export default function ModelsView({ timeFilter, project, source }: { timeFilter: string; project: string; source: string }) {
  const since = sinceForFilter(timeFilter)
  const { data, isLoading } = useStats(since, project, source)

  if (isLoading) return <div className="p-5"><div className="h-64 animate-pulse rounded-[var(--radius)] bg-[var(--bg-fill-tertiary)]" /></div>
  if (!data) return null

  const columns: Column<ModelRow>[] = [
    { key: 'model', header: 'Model', render: (m) => <span className="font-medium text-[var(--text-strong)]">{m.model || '—'}</span>, sortValue: (m) => m.model },
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