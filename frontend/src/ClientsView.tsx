import { useStats } from './api'
import { formatNumber, formatCurrency, sinceForFilter } from './format'
import { Card, CardHeader, Badge } from './ui/Card'
import { SortableTable, type Column } from './ui/SortableTable'
import type { ClientRow } from './types'

export default function ClientsView({ timeFilter }: { timeFilter: string }) {
  const since = sinceForFilter(timeFilter)
  const { data, isLoading } = useStats(since)

  if (isLoading) return <div className="p-5 text-[12px] text-[var(--text-soft)]">Loading…</div>
  if (!data) return null

  const columns: Column<ClientRow>[] = [
    { key: 'app', header: 'Client App', render: (c) => <Badge>{c.client_app}</Badge>, sortValue: (c) => c.client_app },
    { key: 'requests', header: 'Requests', align: 'right', render: (c) => <span className="num">{formatNumber(c.requests)}</span>, sortValue: (c) => c.requests },
    { key: 'prompt', header: 'Prompt', align: 'right', render: (c) => <span className="num">{formatNumber(c.prompt_tokens)}</span>, sortValue: (c) => c.prompt_tokens },
    { key: 'completion', header: 'Completion', align: 'right', render: (c) => <span className="num">{formatNumber(c.completion_tokens)}</span>, sortValue: (c) => c.completion_tokens },
    { key: 'total', header: 'Total Tokens', align: 'right', render: (c) => <span className="num font-semibold">{formatNumber(c.total_tokens)}</span>, sortValue: (c) => c.total_tokens },
    { key: 'cost', header: 'Cost (USD)', align: 'right', render: (c) => <span className="num text-[var(--brand-ink)]">{formatCurrency(c.cost_usd)}</span>, sortValue: (c) => c.cost_usd },
  ]

  return (
    <div className="p-5">
      <Card>
        <CardHeader
          title="Usage by Client App"
          subtitle={`${data.by_client.length} CLI agents seen · claude-cli, crush, opencode, omp…`}
        />
        <SortableTable rows={data.by_client} rowKey={(c) => c.client_app} columns={columns} pageSize={25} />
      </Card>
    </div>
  )
}