import { useState } from 'react'
import { useTps } from './api'
import { formatTps, formatNumber, sinceForFilter, formatBucket } from './format'
import { Card } from './ui/Card'
import { SortableTable, type Column } from './ui/SortableTable'
import { AreaChart, Area, BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer, Legend } from 'recharts'
import type { TpsModelRow, TpsProjectRow } from './types'
import { useTimezone } from './context/TimezoneContext'

export default function ThroughputView({ timeFilter, project, source }: { timeFilter: string; project: string; source: string; tag?: string }) {
  const [interval, setInterval] = useState<'1m' | '1h' | '1d'>('1h')
 const { timezone } = useTimezone()
  const since = sinceForFilter(timeFilter)
  const { data, isLoading, error } = useTps(since, interval, project, source)

  if (isLoading) return <div className="space-y-4 p-5"><Card className="h-64 animate-pulse bg-[var(--bg-fill-secondary)]" /></div>
  if (error || !data) return <div className="p-5 text-[12px] text-[var(--danger-ink)]">Failed to load TPS.</div>

  const modelCols: Column<TpsModelRow>[] = [
    { key: 'model', header: 'Model', render: (r) => <span className="font-medium text-[var(--text-strong)]">{r.model}</span>, sortValue: (r) => r.model },
    { key: 'provider', header: 'Provider', render: (r) => r.provider, sortValue: (r) => r.provider },
    { key: 'tps', header: 'TPS', align: 'right', render: (r) => formatTps(r.tps), sortValue: (r) => r.tps },
    { key: 'avg_latency_ms', header: 'Avg latency', align: 'right', render: (r) => Math.round(r.avg_latency_ms) + 'ms', sortValue: (r) => r.avg_latency_ms },
    { key: 'requests', header: 'Reqs', align: 'right', render: (r) => formatNumber(r.requests), sortValue: (r) => r.requests },
    { key: 'completion_tokens', header: 'Out tokens', align: 'right', render: (r) => formatNumber(r.completion_tokens), sortValue: (r) => r.completion_tokens },
  ]
  const projCols: Column<TpsProjectRow>[] = [
    { key: 'project_id', header: 'Project', render: (r) => r.project_id, sortValue: (r) => r.project_id },
    { key: 'tps', header: 'TPS', align: 'right', render: (r) => formatTps(r.tps), sortValue: (r) => r.tps },
    { key: 'avg_latency_ms', header: 'Avg latency', align: 'right', render: (r) => Math.round(r.avg_latency_ms) + 'ms', sortValue: (r) => r.avg_latency_ms },
    { key: 'requests', header: 'Reqs', align: 'right', render: (r) => formatNumber(r.requests), sortValue: (r) => r.requests },
  ]
  const s = data.summary

  return (
    <div className="space-y-5 p-5">
      <div className="flex flex-wrap items-center gap-2">
        <h2 className="text-[14px] font-semibold text-[var(--text-strong)]">Throughput</h2>
        <span className="rounded-full bg-[var(--accent)] px-2 py-0.5 text-[11px] font-medium text-[var(--accent-contrast)]">live {formatTps(data.live_tps)}</span>
        <div className="ml-auto flex items-center gap-1 rounded-lg border border-[var(--border)] bg-[var(--bg-panel)] p-0.5">
          {(['1m', '1h', '1d'] as const).map((v) => (
            <button key={v} onClick={() => setInterval(v)} className={`rounded-md px-2.5 py-1 text-[11px] font-medium ${interval === v ? 'bg-[var(--accent)] text-[var(--accent-contrast)]' : 'text-[var(--text-muted)]'}`}>{v}</button>
          ))}
        </div>
      </div>

      <div className="grid grid-cols-2 gap-3 md:grid-cols-4">
        <Card className="p-4"><div className="text-[11px] text-[var(--text-soft)]">Avg TPS</div><div className="text-[20px] font-semibold text-[var(--text-strong)]">{formatTps(s.avg_tps)}</div><div className="text-[11px] text-[var(--text-muted)]">{formatNumber(s.requests)} reqs · {Math.round(s.avg_latency_ms)}ms avg</div></Card>
        <Card className="p-4"><div className="text-[11px] text-[var(--text-soft)]">Peak TPS</div><div className="text-[20px] font-semibold text-[var(--text-strong)]">{formatTps(s.peak_tps)}</div><div className="text-[11px] text-[var(--text-muted)]">max bucket</div></Card>
        <Card className="p-4"><div className="text-[11px] text-[var(--text-soft)]">Completion tokens</div><div className="text-[20px] font-semibold text-[var(--text-strong)]">{formatNumber(s.completion_tokens)}</div><div className="text-[11px] text-[var(--text-muted)]">{formatNumber(s.total_tokens)} total</div></Card>
        <Card className="p-4"><div className="text-[11px] text-[var(--text-soft)]">Buckets</div><div className="text-[20px] font-semibold text-[var(--text-strong)]">{data.series.length}</div><div className="text-[11px] text-[var(--text-muted)]">interval {interval}</div></Card>
      </div>

      <Card className="p-4">
        <div className="mb-2 text-[12px] font-medium text-[var(--text-strong)]">TPS over time</div>
        <div className="h-56">
          <ResponsiveContainer width="100%" height="100%">
            <AreaChart data={data.series}>
              <CartesianGrid strokeDasharray="3 3" stroke="var(--border)" />
 <XAxis dataKey="bucket" tick={{ fontSize: 10 }} tickFormatter={(v: string) => formatBucket(v, timezone)} />
              <YAxis tick={{ fontSize: 10 }} />
              <Tooltip formatter={(v) => formatTps(v as number)} />
              <Legend />
              <Area type="monotone" dataKey="avg_tps" name="avg tok/s" stroke="var(--chart-1)" fill="var(--chart-1)" fillOpacity={0.15} />
              <Area type="monotone" dataKey="peak_tps" name="peak tok/s" stroke="var(--chart-2)" fill="transparent" strokeDasharray="4 3" />
            </AreaChart>
          </ResponsiveContainer>
        </div>
      </Card>

      <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
        <Card className="p-4">
          <div className="mb-2 text-[12px] font-medium text-[var(--text-strong)]">TPS by model</div>
          <div className="h-56">
            <ResponsiveContainer width="100%" height="100%">
              <BarChart data={data.by_model.slice(0, 8)} layout="vertical">
                <CartesianGrid strokeDasharray="3 3" stroke="var(--border)" />
                <XAxis type="number" tick={{ fontSize: 10 }} />
                <YAxis type="category" dataKey="model" width={120} tick={{ fontSize: 10 }} />
                <Tooltip formatter={(v) => formatTps(v as number)} />
                <Bar dataKey="tps" fill="var(--chart-3)" radius={[0, 4, 4, 0]} />
              </BarChart>
            </ResponsiveContainer>
          </div>
        </Card>
        <Card className="p-4">
          <div className="mb-2 text-[12px] font-medium text-[var(--text-strong)]">Details by model</div>
          <SortableTable rows={data.by_model} rowKey={(r) => r.model + r.provider} columns={modelCols} pageSize={6} />
        </Card>
      </div>

      <Card className="p-4">
        <div className="mb-2 text-[12px] font-medium text-[var(--text-strong)]">TPS by project</div>
        <SortableTable rows={data.by_project} rowKey={(r) => r.project_id} columns={projCols} pageSize={6} />
      </Card>
    </div>
  )
}
