import { useStats } from './api'
import { formatNumber, formatCurrency, formatLatency, sinceForFilter } from './format'
import { Card, CardHeader, Badge } from './ui/Card'
import { SortableTable, type Column } from './ui/SortableTable'
import { BarChart, Bar, Line, XAxis, YAxis, CartesianGrid, Tooltip, Legend, ResponsiveContainer, PieChart, Pie, Cell } from 'recharts'
import type { EventRow } from './types'

const PIE_COLORS = ['var(--chart-1)', 'var(--chart-2)', 'var(--chart-3)', 'var(--chart-4)', 'var(--chart-5)', 'var(--chart-6)', 'var(--chart-7)']

function Kpi({ label, value, sub, color }: { label: string; value: string; sub: string; color: string }) {
  return (
    <Card className="p-5">
      <div className="text-[11px] font-semibold uppercase tracking-wider text-[var(--text-soft)]">{label}</div>
      <div className={`num mt-2 text-[26px] font-extrabold ${color}`}>{value}</div>
      <div className="mt-1 text-[11px] text-[var(--text-faint)]">{sub}</div>
    </Card>
  )
}

export default function OverviewView({ timeFilter }: { timeFilter: string }) {
  const since = sinceForFilter(timeFilter)
  const { data, isLoading, error } = useStats(since)

  if (isLoading) return <div className="p-5 text-[12px] text-[var(--text-soft)]">Loading…</div>
  if (error || !data) return <div className="p-5 text-[12px] text-[var(--danger-ink)]">Failed to load stats.</div>

  const { summary, by_model, by_client, recent_events, daily, guard } = data
  const guardTone = guard.status === 'breached' ? 'danger' : guard.status === 'warning' ? 'warn' : 'brand'

  const eventColumns: Column<EventRow>[] = [
    { key: 'timestamp', header: 'Time', render: (e) => <span className="text-[var(--text-soft)]">{e.timestamp.slice(11, 19)}</span>, sortValue: (e) => e.timestamp },
    { key: 'client_app', header: 'App', render: (e) => <Badge>{e.client_app}</Badge> },
    { key: 'provider', header: 'Provider', render: (e) => e.provider },
    { key: 'model', header: 'Model', render: (e) => <span className="font-semibold text-[var(--text-strong)]">{e.model || '—'}</span> },
    { key: 'prompt', header: 'Prompt', align: 'right', render: (e) => <span className="num">{formatNumber(e.prompt_tokens)}</span>, sortValue: (e) => e.prompt_tokens },
    { key: 'completion', header: 'Compl', align: 'right', render: (e) => <span className="num">{formatNumber(e.completion_tokens)}</span>, sortValue: (e) => e.completion_tokens },
    { key: 'latency', header: 'Latency', align: 'right', render: (e) => <span className="num text-[var(--text-soft)]">{formatLatency(e.latency_ms)}</span>, sortValue: (e) => e.latency_ms },
    { key: 'cost', header: 'Cost', align: 'right', render: (e) => <span className="num text-[var(--brand-ink)]">{formatCurrency(e.cost_usd)}</span>, sortValue: (e) => e.cost_usd },
  ]

  const pieData = by_model.map((m) => ({ name: m.model || 'unknown', value: m.cost_usd })).filter((d) => d.value > 0)

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

      {/* KPIs */}
      <div className="grid grid-cols-2 gap-4 lg:grid-cols-4">
        <Kpi label="Total Spent" value={formatCurrency(summary.total_cost_usd)} sub="estimated USD" color="text-[var(--chart-1)]" />
        <Kpi label="Total Tokens" value={formatNumber(summary.total_tokens)} sub="prompt + completion" color="text-[var(--text-strong)]" />
        <Kpi label="Cache Savings" value={formatNumber(summary.total_cache_read_tokens)} sub="cached read tokens" color="text-[var(--chart-6)]" />
        <Kpi label="Total Requests" value={formatNumber(summary.total_requests)} sub={`avg latency ${formatLatency(summary.avg_latency_ms)}`} color="text-[var(--text-strong)]" />
      </div>

      {/* Charts */}
      <div className="grid grid-cols-1 gap-5 lg:grid-cols-3">
        <Card className="lg:col-span-2">
          <CardHeader title="Daily Consumption" subtitle="tokens per day with cost overlay" />
          <div className="h-64 px-2 pb-4">
            <ResponsiveContainer width="100%" height="100%">
              <BarChart data={daily}>
                <CartesianGrid stroke="var(--border-soft)" vertical={false} />
                <XAxis dataKey="day" stroke="var(--text-faint)" fontSize={10} tickLine={false} axisLine={false} />
                <YAxis yAxisId="left" stroke="var(--text-faint)" fontSize={10} tickLine={false} axisLine={false} />
                <YAxis yAxisId="right" orientation="right" stroke="var(--text-faint)" fontSize={10} tickLine={false} axisLine={false} />
                <Tooltip
                  contentStyle={{ background: 'var(--bg-elevated)', border: '1px solid var(--border)', borderRadius: 8, fontSize: 12, color: 'var(--text-strong)' }}
                />
                <Legend wrapperStyle={{ fontSize: 11 }} />
                <Bar yAxisId="left" dataKey="total_tokens" name="Tokens" fill="var(--chart-2)" radius={[3, 3, 0, 0]} />
                <Line yAxisId="right" type="monotone" dataKey="cost_usd" name="Cost (USD)" stroke="var(--chart-1)" strokeWidth={2} dot={false} />
              </BarChart>
            </ResponsiveContainer>
          </div>
        </Card>

        <Card>
          <CardHeader title="Cost by Model" subtitle="distribution of spend" />
          <div className="h-64 px-5 pb-4">
            {pieData.length > 0 ? (
              <ResponsiveContainer width="100%" height="100%">
                <PieChart>
                  <Pie data={pieData} dataKey="value" nameKey="name" innerRadius={45} outerRadius={75} paddingAngle={2}>
                    {pieData.map((_, i) => <Cell key={i} fill={PIE_COLORS[i % PIE_COLORS.length]} />)}
                  </Pie>
                  <Tooltip
                    contentStyle={{ background: 'var(--bg-elevated)', border: '1px solid var(--border)', borderRadius: 8, fontSize: 12, color: 'var(--text-strong)]' }}
                  />
                  <Legend wrapperStyle={{ fontSize: 10 }} />
                </PieChart>
              </ResponsiveContainer>
            ) : (
              <div className="flex h-full items-center justify-center text-[12px] text-[var(--text-faint)]">No cost recorded</div>
            )}
          </div>
        </Card>
      </div>

      {/* Top models + recent stream */}
      <div className="grid grid-cols-1 gap-5 lg:grid-cols-2">
        <Card>
          <CardHeader title="Top Models" subtitle="by token volume" />
          <SortableTable
            rows={by_model}
            rowKey={(m) => m.model + m.provider}
            pageSize={8}
            columns={[
              { key: 'model', header: 'Model', render: (m) => <span className="font-medium text-[var(--text-strong)]">{m.model || '—'}</span>, sortValue: (m) => m.model },
              { key: 'tokens', header: 'Tokens', align: 'right', render: (m) => <span className="num">{formatNumber(m.total_tokens)}</span>, sortValue: (m) => m.total_tokens },
              { key: 'cost', header: 'Cost', align: 'right', render: (m) => <span className="num text-[var(--brand-ink)]">{formatCurrency(m.cost_usd)}</span>, sortValue: (m) => m.cost_usd },
            ]}
          />
        </Card>

        <Card>
          <CardHeader title="Client Apps" subtitle="usage per CLI agent" />
          <SortableTable
            rows={by_client}
            rowKey={(c) => c.client_app}
            pageSize={8}
            columns={[
              { key: 'app', header: 'App', render: (c) => <Badge>{c.client_app}</Badge>, sortValue: (c) => c.client_app },
              { key: 'tokens', header: 'Tokens', align: 'right', render: (c) => <span className="num">{formatNumber(c.total_tokens)}</span>, sortValue: (c) => c.total_tokens },
              { key: 'cost', header: 'Cost', align: 'right', render: (c) => <span className="num text-[var(--brand-ink)]">{formatCurrency(c.cost_usd)}</span>, sortValue: (c) => c.cost_usd },
            ]}
          />
        </Card>
      </div>

      {/* Recent events */}
      <Card>
        <CardHeader title="Recent Requests" subtitle="latest 50 events" right={<Badge tone="brand">live · 10s</Badge>} />
        <SortableTable rows={recent_events} rowKey={(e) => e.id} columns={eventColumns} pageSize={10} />
      </Card>
    </div>
  )
}