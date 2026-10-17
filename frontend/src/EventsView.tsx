import { useState } from 'react'
import { useStats } from './api'
import { formatNumber, formatCurrency, formatLatency, sinceForFilter } from './format'
import { Card, CardHeader, Badge } from './ui/Card'
import { SortableTable, type Column } from './ui/SortableTable'
import { Dialog } from './ui/Dialog'
import type { EventRow } from './types'

export default function EventsView({ timeFilter, project }: { timeFilter: string; project: string }) {
  const since = sinceForFilter(timeFilter)
  const { data, isLoading } = useStats(since, project)
  const [selected, setSelected] = useState<EventRow | null>(null)

  if (isLoading) return <div className="p-5"><div className="h-64 animate-pulse rounded-[var(--radius)] bg-[var(--bg-fill-tertiary)]" /></div>
  if (!data) return null

  const columns: Column<EventRow>[] = [
    { key: 'timestamp', header: 'Time', render: (e) => <span className="num text-[var(--text-soft)]">{e.timestamp.slice(0, 19).replace('T', ' ')}</span>, sortValue: (e) => e.timestamp },
    { key: 'client_app', header: 'App', render: (e) => <Badge>{e.client_app}</Badge>, sortValue: (e) => e.client_app },
    { key: 'provider', header: 'Provider', render: (e) => e.provider, sortValue: (e) => e.provider },
    { key: 'model', header: 'Model', render: (e) => <span className="font-semibold text-[var(--text-strong)]">{e.model || '—'}</span>, sortValue: (e) => e.model },
    { key: 'prompt', header: 'Prompt', align: 'right', render: (e) => <span className="num tabular-nums">{formatNumber(e.prompt_tokens)}</span>, sortValue: (e) => e.prompt_tokens },
    { key: 'completion', header: 'Completion', align: 'right', render: (e) => <span className="num tabular-nums">{formatNumber(e.completion_tokens)}</span>, sortValue: (e) => e.completion_tokens },
    { key: 'cache', header: 'Cache R/W', align: 'right', render: (e) => <span className="num tabular-nums text-[var(--text-soft)]">{formatNumber(e.cache_read_tokens)}/{formatNumber(e.cache_write_tokens)}</span>, sortValue: (e) => e.cache_read_tokens },
    { key: 'status', header: 'Status', align: 'right', render: (e) => <span className={`inline-flex h-5 w-5 items-center justify-center rounded-full text-[11px] font-bold ${e.status_code < 400 ? 'bg-[var(--brand-surface)] text-[var(--brand-ink)]' : e.status_code < 500 ? 'bg-[var(--warn-soft)] text-[var(--warn-ink)]' : 'bg-[var(--danger-soft)] text-[var(--danger-ink)]'}`}>{e.status_code < 400 ? '·' : e.status_code}</span>, sortValue: (e) => e.status_code },
    { key: 'latency', header: 'Latency', align: 'right', render: (e) => { const c = e.latency_ms > 5000 ? 'text-[var(--danger-ink)]' : e.latency_ms > 1000 ? 'text-[var(--warn-ink)]' : 'text-[var(--brand-ink)]'; return <span className={`num tabular-nums ${c}`}>{formatLatency(e.latency_ms)}</span> }, sortValue: (e) => e.latency_ms },
    { key: 'cost', header: 'Cost', align: 'right', render: (e) => <span className="num tabular-nums text-[var(--brand-ink)]">{formatCurrency(e.cost_usd)}</span>, sortValue: (e) => e.cost_usd },
  ]

  return (
    <div className="p-5">
      <Card>
        <CardHeader
          title="Request Stream"
          subtitle="click a row for details · sorted by newest first"
          right={<Badge tone="brand">live · 10s</Badge>}
        />
        <SortableTable
          rows={data.recent_events}
          rowKey={(e) => e.id}
          columns={columns}
          pageSize={25}
          onRowClick={setSelected}
          activeRowKey={selected?.id}
        />
      </Card>

      <Dialog open={selected !== null} onClose={() => setSelected(null)} title={`Event #${selected?.id ?? ''}`}>
        {selected && (
          <div className="space-y-4 text-[12px]">
            <div className="grid grid-cols-3 gap-3">
              {[
                ['Timestamp', selected.timestamp],
                ['Client App', selected.client_app],
                ['Provider', selected.provider],
                ['Model', selected.model || '—'],
                ['Status Code', String(selected.status_code)],
                ['Latency', formatLatency(selected.latency_ms)],
                ['Prompt Tokens', formatNumber(selected.prompt_tokens)],
                ['Completion Tokens', formatNumber(selected.completion_tokens)],
                ['Cache Read', formatNumber(selected.cache_read_tokens)],
                ['Cache Write', formatNumber(selected.cache_write_tokens)],
                ['Total Tokens', formatNumber(selected.total_tokens)],
                ['Cost', formatCurrency(selected.cost_usd)],
              ].map(([k, v]) => (
                <div key={k} className="rounded-md border border-[var(--border-soft)] bg-[var(--bg-fill-tertiary)] px-3 py-2">
                  <div className="text-[10px] uppercase tracking-wider text-[var(--text-soft)]">{k}</div>
                  <div className="num mt-0.5 font-medium text-[var(--text-strong)]">{v}</div>
                </div>
              ))}
            </div>
            <div>
              <div className="mb-1 text-[10px] uppercase tracking-wider text-[var(--text-soft)]">Raw Event (JSON)</div>
              <pre className="max-h-56 overflow-auto rounded-md border border-[var(--border-soft)] bg-[var(--bg)] p-3 font-mono text-[11px] leading-relaxed text-[var(--text-muted)]">
                {JSON.stringify(selected, null, 2)}
              </pre>
            </div>
          </div>
        )}
      </Dialog>
    </div>
  )
}