import { useMemo } from 'react'
import { useStats } from './api'
import { formatNumber, formatNumberCompact, formatCurrency, formatLatency, sinceForFilter } from './format'
import { Card, CardHeader, Badge } from './ui/Card'
import { SortableTable, type Column } from './ui/SortableTable'
import { BarChart, Bar, Line, XAxis, YAxis, CartesianGrid, Tooltip, Legend, ResponsiveContainer, PieChart, Pie, Cell } from 'recharts'
import type { EventRow, ProjectRow } from './types'

const PIE_COLORS = ['var(--chart-1)', 'var(--chart-2)', 'var(--chart-3)', 'var(--chart-4)', 'var(--chart-5)', 'var(--chart-6)', 'var(--chart-7)']


function KpiSkeleton() {
  return (
    <Card className="p-5">
      <div className="h-3 w-20 animate-pulse rounded bg-[var(--bg-fill-secondary)]" />
      <div className="mt-3 h-8 w-28 animate-pulse rounded bg-[var(--bg-fill-secondary)]" />
      <div className="mt-2 h-3 w-24 animate-pulse rounded bg-[var(--bg-fill-secondary)]" />
    </Card>
  )
}

function Kpi({ label, value, sub, color }: { label: string; value: string; sub: string; color: string }) {
  return (
    <Card className="p-5">
      <div className="text-[11px] font-semibold uppercase tracking-wider text-[var(--text-soft)]">{label}</div>
      <div className={`num mt-2 text-[28px] font-bold leading-none tabular-nums ${color}`}>{value}</div>
      <div className="mt-1 text-[11px] text-[var(--text-soft)]" title={sub}>{sub}</div>
    </Card>
  )
}

export default function OverviewView({ timeFilter, project, source }: { timeFilter: string; project: string; source: string }) {
  const since = sinceForFilter(timeFilter)
  const { data, isLoading, error } = useStats(since, project, source)

  const pieData = useMemo(() => {
    const src = data?.by_model ?? []
    const sorted = src.map((m) => ({ name: m.model || 'unknown', value: m.cost_usd })).filter((d) => d.value > 0).sort((a, b) => b.value - a.value)
    if (sorted.length <= 6) return sorted
    const top = sorted.slice(0, 5)
    const rest = sorted.slice(5).reduce((acc, cur) => acc + cur.value, 0)
    return [...top, { name: 'other', value: rest }]
  }, [data?.by_model])
  const pieTotal = useMemo(() => pieData.reduce((a, d) => a + d.value, 0), [pieData])
  const eventColumns: Column<EventRow>[] = [
    { key: 'timestamp', header: 'Time', render: (e) => <span className="text-[var(--text-soft)]">{e.timestamp.slice(11, 19)}</span>, sortValue: (e) => e.timestamp },
    { key: 'client_app', header: 'App', render: (e) => <Badge>{e.client_app}</Badge> },
    { key: 'provider', header: 'Provider', render: (e) => e.provider },
    { key: 'model', header: 'Model', render: (e) => <span className="font-semibold text-[var(--text-strong)]">{e.model || '—'}</span> },
    { key: 'prompt', header: 'Prompt', align: 'right', render: (e) => <span className="num tabular-nums" title={String(e.prompt_tokens)}>{formatNumber(e.prompt_tokens)}</span>, sortValue: (e) => e.prompt_tokens },
    { key: 'completion', header: 'Completion', align: 'right', render: (e) => <span className="num tabular-nums" title={String(e.completion_tokens)}>{formatNumber(e.completion_tokens)}</span>, sortValue: (e) => e.completion_tokens },
    { key: 'latency', header: 'Latency', align: 'right', render: (e) => <span className="num tabular-nums text-[var(--text-soft)]">{formatLatency(e.latency_ms)}</span>, sortValue: (e) => e.latency_ms },
    { key: 'cost', header: 'Cost', align: 'right', render: (e) => <span className="num tabular-nums text-[var(--brand-ink)]">{formatCurrency(e.cost_usd)}</span>, sortValue: (e) => e.cost_usd },
  ]

  if (isLoading) return (
    <div className="space-y-5 p-5">
      <div className="grid grid-cols-2 gap-4 lg:grid-cols-5">
        <KpiSkeleton /><KpiSkeleton /><KpiSkeleton /><KpiSkeleton /><KpiSkeleton />
      </div>
      <div className="grid grid-cols-1 gap-5 lg:grid-cols-3">
        <Card className="h-64 animate-pulse bg-[var(--bg-fill-tertiary)] lg:col-span-2" />
        <Card className="h-64 animate-pulse bg-[var(--bg-fill-tertiary)]" />
      </div>
    </div>
  )
  if (error || !data) return <div className="p-5 text-[12px] text-[var(--danger-ink)]">Failed to load stats.</div>

  const { summary, by_model, by_project, savings, recent_events, daily, guard } = data
  const guardTone = guard.status === 'breached' ? 'danger' : guard.status === 'warning' ? 'warn' : 'brand'

  return (
    <div className="space-y-5 p-5">
      {/* Guard status banner */}
      {guard.status !== 'ok' && (
        <div className={`flex items-center gap-2 rounded-lg border px-4 py-2.5 text-[12px] ${
          guard.status === 'breached'
            ? 'border-transparent bg-[var(--danger-soft)] text-[var(--danger-ink)]'
            : 'border-transparent bg-[var(--warn-soft)] text-[var(--warn-ink)]'
        }`}>
          <Badge tone={guardTone}>{guard.status}</Badge>
          <span>{guard.reason || 'Budget threshold reached'}</span>
        </div>
      )}

      {/* KPIs — always 5, Saved shows $0.00 when absent */}
      <div className="flex justify-end">
        <a href={`/api/stats.csv${since ? `?since=${encodeURIComponent(since)}${source !== 'all' ? `&source=${source}` : ''}${project !== 'all' ? `&project=${project}` : ''}` : source !== 'all' || project !== 'all' ? `?${source !== 'all' ? `source=${source}` : ''}${project !== 'all' ? `${source !== 'all' ? '&' : ''}project=${project}` : ''}` : ''}`} className="rounded-md border border-[var(--border)] bg-[var(--bg-panel)] px-3 py-1.5 text-[11px] font-medium text-[var(--text-muted)] hover:text-[var(--text-strong)]">Export CSV</a>
      </div>
      <div className="grid grid-cols-2 gap-4 lg:grid-cols-5">
        <Kpi label="Total Spent" value={formatCurrency(summary.total_cost_usd)} sub="estimated USD" color="text-[var(--text-strong)]" />
        <Kpi label="Total Tokens" value={formatNumber(summary.total_tokens)} sub="prompt + completion" color="text-[var(--text-strong)]" />
        <Kpi label="Cache Read" value={formatNumber(summary.total_cache_read_tokens)} sub="cached tokens" color="text-[var(--text-strong)]" />
        <Kpi label="Total Requests" value={formatNumber(summary.total_requests)} sub={`avg ${formatLatency(summary.avg_latency_ms)}`} color="text-[var(--text-strong)]" />
        <Kpi label="Saved vs Cloud" value={formatCurrency(savings?.saved_usd ?? 0)} sub={savings ? `shadow ${formatCurrency(savings.shadow_usd)}` : '—'} color="text-[var(--chart-1)]" />
      </div>

      {/* Charts */}
      <div className="grid grid-cols-1 gap-5 lg:grid-cols-3">
        <Card className="lg:col-span-2">
          <CardHeader title="Daily Consumption" subtitle="tokens per day with cost overlay" />
          <div className="h-64 px-1 pb-4">
            <ResponsiveContainer width="100%" height="100%">
              <BarChart data={daily} margin={{ left: 4, right: 16, top: 4, bottom: 0 }} barCategoryGap="30%">
                <CartesianGrid stroke="var(--border-soft)" strokeDasharray="3 3" vertical={false} />
                <XAxis dataKey="day" stroke="var(--text-soft)" fontSize={10} tickLine={false} axisLine={false} tickFormatter={(v: string) => v.slice(5)} />
                <YAxis yAxisId="left" width={42} tickMargin={6} stroke="var(--text-soft)" fontSize={10} tickLine={false} axisLine={false} tickFormatter={(v: number) => formatNumberCompact(v)} />
                <YAxis yAxisId="right" width={52} tickMargin={6} orientation="right" stroke="var(--text-soft)" fontSize={10} tickLine={false} axisLine={false} tickFormatter={(v: number) => formatCurrency(v)} />
                <Tooltip contentStyle={{ background: 'var(--bg-elevated)', border: '1px solid var(--border)', borderRadius: 8, fontSize: 12, color: 'var(--text-strong)' }} labelStyle={{ color: 'var(--text-soft)' }} />
                <Legend wrapperStyle={{ fontSize: 11, paddingTop: 8 }} />
                <Bar yAxisId="left" dataKey="total_tokens" name="Tokens" fill="var(--chart-2)" radius={[4, 4, 0, 0]} />
                <Line yAxisId="right" type="monotone" dataKey="cost_usd" name="Cost" stroke="var(--chart-1)" strokeWidth={2} dot={{ r: 2, fill: 'var(--chart-1)' }} activeDot={{ r: 4 }} connectNulls />
              </BarChart>
            </ResponsiveContainer>
          </div>
        </Card>

        <Card>
          <CardHeader title="Cost by Model" subtitle={`${formatCurrency(pieTotal)} total`} />
          <div className="h-64 px-2 pb-2">
            {pieData.length === 1 ? (
              <div className="flex h-full flex-col items-center justify-center gap-1">
                <div className="text-[11px] uppercase tracking-wider text-[var(--text-soft)]">100% · {pieData[0].name}</div>
                <div className="num text-[22px] font-bold text-[var(--text-strong)]">{formatCurrency(pieData[0].value)}</div>
              </div>
            ) : pieData.length > 0 ? (
              <ResponsiveContainer width="100%" height="100%">
                <PieChart>
                  <Pie data={pieData} dataKey="value" nameKey="name" innerRadius={52} outerRadius={78} paddingAngle={2} labelLine={false} label={({ percent }) => (percent ?? 0) >= 0.05 ? `${Math.round((percent ?? 0) * 100)}%` : ''}>
                    {pieData.map((_, i) => <Cell key={i} fill={PIE_COLORS[i % PIE_COLORS.length]} />)}
                  </Pie>
                  <Tooltip contentStyle={{ background: 'var(--bg-elevated)', border: '1px solid var(--border)', borderRadius: 8, fontSize: 12, color: 'var(--text-strong)' }} formatter={(v) => formatCurrency(v as number)} />
                  <Legend wrapperStyle={{ fontSize: 11, paddingTop: 6 }} iconType="circle" />
                </PieChart>
              </ResponsiveContainer>
            ) : (
              <div className="flex h-full items-center justify-center text-[12px] text-[var(--text-faint)]">No cost recorded</div>
            )}
          </div>
        </Card>
      </div>

      {/* Top models + projects — mirrored columns */}
      <div className="grid grid-cols-1 gap-5 lg:grid-cols-2">
        <Card>
          <CardHeader title="Top Models" subtitle="by token volume" />
          <SortableTable
            rows={by_model}
            rowKey={(m) => m.model + m.provider}
            pageSize={8}
            columns={[
              { key: 'model', header: 'Model', render: (m) => <span className="font-medium text-[var(--text-strong)]">{m.model || '—'}</span>, sortValue: (m) => m.model },
              { key: 'requests', header: 'Requests', align: 'right', render: (m) => <span className="num tabular-nums">{formatNumber(m.requests)}</span>, sortValue: (m) => m.requests },
              { key: 'tokens', header: 'Tokens', align: 'right', render: (m) => <span className="num tabular-nums font-medium" title={String(m.total_tokens)}>{formatNumber(m.total_tokens)}</span>, sortValue: (m) => m.total_tokens },
              { key: 'cost', header: 'Cost', align: 'right', render: (m) => <span className="num tabular-nums text-[var(--brand-ink)]">{formatCurrency(m.cost_usd)}</span>, sortValue: (m) => m.cost_usd },
            ]}
          />
        </Card>

        <Card>
          <CardHeader title="Projects" subtitle="cost per X-Odo-Project tag" />
          <SortableTable
            rows={by_project ?? ([] as ProjectRow[])}
            rowKey={(p) => p.project_id}
            pageSize={8}
            columns={[
              { key: 'project', header: 'Project', render: (p) => <Badge>{p.project_id}</Badge>, sortValue: (p) => p.project_id },
              { key: 'requests', header: 'Requests', align: 'right', render: (p) => <span className="num tabular-nums">{formatNumber(p.requests)}</span>, sortValue: (p) => p.requests },
              { key: 'tokens', header: 'Tokens', align: 'right', render: (p) => <span className="num tabular-nums" title={String(p.total_tokens)}>{formatNumber(p.total_tokens)}</span>, sortValue: (p) => p.total_tokens },
              { key: 'cost', header: 'Cost', align: 'right', render: (p) => <span className="num tabular-nums text-[var(--brand-ink)]">{formatCurrency(p.cost_usd)}</span>, sortValue: (p) => p.cost_usd },
              { key: 'shadow', header: 'Shadow', align: 'right', render: (p) => <span className="num tabular-nums text-[var(--text-soft)]">{formatCurrency(p.shadow_cost_usd)}</span>, sortValue: (p) => p.shadow_cost_usd },
            ]}
          />
        </Card>
      </div>

      {/* By Source */}
      {data.by_source && data.by_source.length > 0 && (
        <Card>
          <CardHeader title="By Source" subtitle="proxy vs imported CLI sessions" />
          <SortableTable
            rows={data.by_source}
            rowKey={(s) => s.source}
            columns={[
              { key: 'source', header: 'Source', render: (s) => <Badge tone={s.source === 'proxy' ? 'brand' : 'neutral'}>{s.source}</Badge>, sortValue: (s) => s.source },
              { key: 'requests', header: 'Requests', align: 'right', render: (s) => <span className="num tabular-nums">{formatNumber(s.requests)}</span>, sortValue: (s) => s.requests },
              { key: 'tokens', header: 'Tokens', align: 'right', render: (s) => <span className="num tabular-nums">{formatNumber(s.total_tokens)}</span>, sortValue: (s) => s.total_tokens },
              { key: 'cost', header: 'Cost', align: 'right', render: (s) => <span className="num tabular-nums text-[var(--brand-ink)]">{formatCurrency(s.cost_usd)}</span>, sortValue: (s) => s.cost_usd },
            ]}
          />
        </Card>
      )}

      {/* Recent events */}
      <Card>
        <CardHeader title="Recent Requests" subtitle="latest 50 events" right={<Badge tone="brand">live · 10s</Badge>} />
        <SortableTable rows={recent_events} rowKey={(e) => e.id} columns={eventColumns} pageSize={10} />
      </Card>
    </div>
  )
}