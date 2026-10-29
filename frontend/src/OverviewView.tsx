import { useMemo, useState } from 'react'
import { useForecast, useLiveSessions, useStats, useDailyDetail } from './api'
import type { DailyRow, ModelRow, EventRow, SourceRow, LiveSession, ProjectRow } from './types'
import type { ForecastPoint, Anomaly } from './types'
import { Card, CardHeader, Badge } from './ui/Card'
import { SortableTable } from './ui/SortableTable'
import type { Column } from './ui/SortableTable'
import { Pie, PieChart, Cell, Tooltip, Legend, ResponsiveContainer, BarChart, CartesianGrid, XAxis, YAxis, Bar, Line } from 'recharts'
import { formatCurrency, formatNumber, formatNumberCompact, formatLatency, formatTimestamp, formatTime, formatTps, sinceForFilter } from './format'
import { useTimezone } from './context/useTimezone'
import { useLocale } from './i18n/useLocale'
import { formatMonthYear, weekdayInitials } from './format'

const PIE_COLORS = ['#10b981', '#3b82f6', '#f59e0b', '#ef4444', '#8b5cf6', '#ec4899', '#14b8a6', '#f97316']
function DailyConsumption({ daily }: { daily: DailyRow[] }) {
  const [viewDate, setViewDate] = useState(new Date())
  const [selected, setSelected] = useState<string | null>(null)
  const { timezone } = useTimezone()
  const { t, locale } = useLocale()
  const { data: detail, isLoading: detailLoading } = useDailyDetail(selected, timezone)

  const year = viewDate.getFullYear()
  const month = viewDate.getMonth()
  const firstDay = new Date(year, month, 1).getDay()
  const daysInMonth = new Date(year, month + 1, 0).getDate()

  const dailyMap = useMemo(() => {
    const m = new Map<string, DailyRow>()
    for (const d of daily) m.set(d.day, d)
    return m
  }, [daily])

  const maxCost = useMemo(() => Math.max(...daily.map(d => d.cost_usd), 0.01), [daily])

  const cells: { day: number; dateStr: string; row?: DailyRow }[] = []
  for (let i = 0; i < firstDay; i++) cells.push({ day: 0, dateStr: '' })
  for (let d = 1; d <= daysInMonth; d++) {
    const dateStr = `${year}-${String(month + 1).padStart(2, '0')}-${String(d).padStart(2, '0')}`
    cells.push({ day: d, dateStr, row: dailyMap.get(dateStr) })
  }

  const intensity = (cost: number) => {
    if (cost <= 0) return 'bg-[var(--bg-fill)] text-[var(--text-soft)]'
    const ratio = cost / maxCost
    if (ratio > 0.6) return 'bg-emerald-600/80 text-white/90'
    if (ratio > 0.3) return 'bg-emerald-500/60 text-white/80'
    return 'bg-emerald-400/40 text-white/70'
  }

  const weekdays = weekdayInitials(locale)
  const today = new Date()
  const isTodayVisible = year === today.getFullYear() && month === today.getMonth()
  const todayStr = `${today.getFullYear()}-${String(today.getMonth() + 1).padStart(2, '0')}-${String(today.getDate()).padStart(2, '0')}`

  const prevMonth = () => setViewDate(new Date(year, month - 1, 1))
  const nextMonth = () => setViewDate(new Date(year, month + 1, 1))
  const goToday = () => { setViewDate(new Date()); setSelected(null) }

  const monthLabel = formatMonthYear(viewDate, locale)

  // Use detail from API if available, otherwise fall back to dailyMap row
  const selectedRow = selected ? dailyMap.get(selected) : undefined
  const displayCost = detail?.summary?.total_cost_usd ?? selectedRow?.cost_usd ?? 0
  const displayTokens = detail?.summary?.total_tokens ?? selectedRow?.total_tokens ?? 0
  const displayReqs = detail?.summary?.total_requests ?? selectedRow?.requests ?? 0

  return (
    <Card className="w-full">
      <div className="flex items-start justify-between px-5 pt-4 pb-3">
        <div>
          <h3 className="text-[13px] font-semibold text-[var(--text-strong)]">{t('calendar.title')}</h3>
          <p className="mt-0.5 text-[11px] text-[var(--text-soft)]">{t('calendar.subtitle')}</p>
        </div>
        <div className="flex items-center gap-1">
          <button onClick={prevMonth} className="flex h-6 w-6 items-center justify-center rounded text-[var(--text-soft)] hover:bg-[var(--bg-fill)] hover:text-[var(--text-strong)]" title={t('overview.previousMonth')}><span className="rtl:rotate-180">‹</span></button>
          <span className="min-w-[110px] text-center text-[11px] font-medium capitalize text-[var(--text-strong)]">{monthLabel}</span>
          <button onClick={nextMonth} className="flex h-6 w-6 items-center justify-center rounded text-[var(--text-soft)] hover:bg-[var(--bg-fill)] hover:text-[var(--text-strong)]" title={t('overview.nextMonth')}><span className="rtl:rotate-180">›</span></button>
          {!isTodayVisible && (
            <button onClick={goToday} className="ms-1 rounded bg-[var(--bg-fill)] px-1.5 py-0.5 text-[9px] font-medium text-[var(--text-soft)] hover:text-[var(--text-strong)]">{t('overview.today')}</button>
          )}
        </div>
      </div>
      <div className="px-3 pb-2">
        <div className="mb-1.5 grid grid-cols-7 gap-1 text-center text-[9px] font-semibold uppercase tracking-wider text-[var(--text-faint)]">
          {weekdays.map((w, i) => <div key={i} className="py-0.5">{w}</div>)}
        </div>
        <div className="grid grid-cols-7 gap-1">
          {cells.map((cell, i) => {
            if (!cell.day) return <button key={i} disabled className="h-9 rounded bg-transparent text-transparent cursor-default" />
            const isToday = cell.dateStr === todayStr
            const isSelected = cell.dateStr === selected
            const cost = cell.row?.cost_usd ?? 0
            return (
              <button
                key={i}
                onClick={() => setSelected(isSelected ? null : cell.dateStr)}
                title={`${cell.dateStr}: ${formatCurrency(cost, locale)} · ${t('overview.tokensSuffix', { value: formatNumberCompact(cell.row?.total_tokens ?? 0, locale) })}`}
                className={`flex h-9 items-center justify-center rounded text-[11px] font-medium tabular-nums transition-colors cursor-pointer hover:brightness-110 ${isSelected ? 'ring-2 ring-[var(--brand)] ring-offset-1 ring-offset-[var(--bg-card)] ' : isToday ? 'ring-2 ring-emerald-400 ring-offset-1 ring-offset-[var(--bg-panel)] ' : ''}${intensity(cost)}`}
              >
                {cell.day}
              </button>
            )
          })}
        </div>
        {selected && (
          <div className="mt-3 flex items-center gap-4 rounded border border-[var(--border-soft)] bg-[var(--bg-fill)] px-3 py-2 text-[11px]">
            <span className="font-semibold text-[var(--text-strong)]">{selected}</span>
            {detailLoading ? (
              <span className="text-[var(--text-faint)]">{t('common.loading')}</span>
            ) : (
              <>
                <span className="text-[var(--text-soft)]">{formatCurrency(displayCost, locale)}</span>
                <span className="text-[var(--text-soft)]">{t('overview.tokensSuffix', { value: formatNumberCompact(displayTokens, locale) })}</span>
                <span className="text-[var(--text-soft)]">{t('overview.reqsSuffix', { value: formatNumber(displayReqs, locale) })}</span>
              </>
            )}
          </div>
        )}
        <div className="mt-2 flex items-center justify-end gap-1 text-[9px] text-[var(--text-faint)]">
          <span>{t('overview.less')}</span>
          {[0, 0.3, 0.6, 1].map((r, i) => (
            <span key={i} className={`inline-block h-2.5 w-2.5 rounded-sm ${r === 0 ? 'bg-[var(--bg-fill)]' : r > 0.6 ? 'bg-emerald-600/80' : r > 0.3 ? 'bg-emerald-500/60' : 'bg-emerald-400/40'}`} />
          ))}
          <span>{t('overview.more')}</span>
        </div>
      </div>
    </Card>
  )
}

function KpiSkeleton() {
  return (
    <Card className="p-5">
      <div className="h-3 w-20 animate-pulse rounded bg-[var(--bg-fill-secondary)]" />
      <div className="mt-3 h-8 w-28 animate-pulse rounded bg-[var(--bg-fill-secondary)]" />
      <div className="mt-2 h-3 w-24 animate-pulse rounded bg-[var(--bg-fill-secondary)]" />
    </Card>
  )
}

function Kpi({ label, value, sub, color }: { label: string; value: string; sub?: string; color?: string }) {
  return (
    <Card className="p-5">
      <div className="text-[11px] font-medium uppercase tracking-wider text-[var(--text-soft)]">{label}</div>
      <div className={`mt-2 text-[22px] font-bold tabular-nums ${color ?? 'text-[var(--text-strong)]'}`}>{value}</div>
      {sub && <div className="mt-1 text-[11px] text-[var(--text-soft)]">{sub}</div>}
    </Card>
  )
}

export default function OverviewView({ timeFilter, project, source, tag }: { timeFilter: string; project: string; source: string; tag: string }) {
 const { timezone } = useTimezone()
 const { t, locale } = useLocale()
 const since = useMemo(() => sinceForFilter(timeFilter, timezone), [timeFilter, timezone])
 const { data, isLoading, error } = useStats(since, project, source, tag, timezone)
 const { data: forecastData } = useForecast(tag !== 'all' ? tag : undefined, timezone)
  const { data: liveSessions = [] } = useLiveSessions()
 // ponytail: 35-day window for calendar heatmap, using midnight UTC of today minus 35 days for stable cache key
 const todayStr = new Date().toISOString().slice(0, 10)
 const calendarSince = useMemo(() => {
   const d = new Date(todayStr + 'T00:00:00Z')
   d.setDate(d.getDate() - 35)
   return d.toISOString()
 }, [todayStr])
  const { data: calendarData } = useStats(calendarSince, project, source, tag, timezone)

 const recentEvents = data?.recent_events ?? []
 const pieData = useMemo(() => {
   const src = data?.by_model ?? []
   const sorted = src.map((m: ModelRow) => ({ name: m.model || 'unknown', value: m.cost_usd })).filter((d) => d.value > 0).sort((a, b) => b.value - a.value)
   if (sorted.length <= 6) return sorted
   const top = sorted.slice(0, 5)
   const rest = sorted.slice(5).reduce((acc, cur) => acc + cur.value, 0)
   return [...top, { name: 'other', value: rest }]
 }, [data?.by_model])
 const pieTotal = useMemo(() => pieData.reduce((a, d) => a + d.value, 0), [pieData])
  const eventColumns: Column<EventRow>[] = [
    { key: 'timestamp', header: t('common.time'), render: (e) => <span className="text-[var(--text-soft)]">{formatTime(e.timestamp, timezone, locale)}</span>, sortValue: (e) => e.timestamp },
    { key: 'client_app', header: t('events.app'), render: (e) => <Badge>{e.client_app}</Badge> },
    { key: 'provider', header: t('common.provider'), render: (e) => e.provider },
    { key: 'model', header: t('common.model'), render: (e) => <span className="font-semibold text-[var(--text-strong)]">{e.model || '—'}</span> },
    { key: 'prompt', header: t('models.prompt'), align: 'right', render: (e) => <span className="num tabular-nums" title={String(e.prompt_tokens)}>{formatNumber(e.prompt_tokens, locale)}</span>, sortValue: (e) => e.prompt_tokens },
    { key: 'completion', header: t('models.completion'), align: 'right', render: (e) => <span className="num tabular-nums" title={String(e.completion_tokens)}>{formatNumber(e.completion_tokens, locale)}</span>, sortValue: (e) => e.completion_tokens },
    { key: 'latency', header: t('events.latency'), align: 'right', render: (e) => <span className="num tabular-nums text-[var(--text-soft)]">{formatLatency(e.latency_ms)}</span>, sortValue: (e) => e.latency_ms },
    { key: 'cost', header: t('common.cost'), align: 'right', render: (e) => <span className="num tabular-nums text-[var(--brand-ink)]">{formatCurrency(e.cost_usd, locale)}</span>, sortValue: (e) => e.cost_usd },
  ]

  const exportQuery = new URLSearchParams()
  if (since) exportQuery.set('since', since)
  if (source !== 'all') exportQuery.set('source', source)
  if (project !== 'all') exportQuery.set('project', project)
  if (tag !== 'all') exportQuery.set('tag', tag)
  exportQuery.set('lang', locale)
  const exportSuffix = exportQuery.toString() ? `?${exportQuery.toString()}` : ''

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
 if (error || !data) return <div className="p-5 text-[12px] text-[var(--danger-ink)]">{t('app.statsFailed')}</div>

 const { summary, by_model, by_project, savings, daily, tps } = data
 const events = recentEvents

  return (
    <div className="space-y-5 p-5">
      {forecastData && (
        <Card className="p-4">
          <div className="flex items-center justify-between">
            <span className="text-[12px] font-semibold text-[var(--text-strong)]">{t('overview.forecast')}</span>
            {(forecastData.anomalies?.length ?? 0) > 0 && <Badge tone="danger">{t(`overview.anomaly.${new Intl.PluralRules(locale).select(forecastData.anomalies.length) === 'one' ? 'one' : 'other'}`, { count: forecastData.anomalies.length })}</Badge>}
          </div>
          <div className="mt-2 flex flex-wrap gap-2">
{(forecastData.forecast ?? []).map((f: ForecastPoint) => (
              <span key={f.day} className="rounded bg-[var(--bg-fill-secondary)] px-2 py-1 text-[11px] tabular-nums">{f.day.slice(5)}: {formatCurrency(f.cost_usd, locale)}</span>
            ))}
          </div>
          {(forecastData.anomalies?.length ?? 0) > 0 && (
<div className="mt-2 text-[11px] text-[var(--danger-ink)]">{t('overview.anomalyDetail')} {(forecastData.anomalies ?? []).map((a: Anomaly) => `${a.day.slice(5)} ${formatCurrency(a.cost_usd, locale)} > ${formatCurrency(a.threshold, locale)}`).join(', ')}</div>
          )}
        </Card>
      )}

      {/* KPIs — always 5, Saved shows $0.00 when absent */}
      <div className="flex justify-end gap-2">
        <a href={`/api/stats.csv${exportSuffix}`} className="rounded-md border border-[var(--border)] bg-[var(--bg-panel)] px-3 py-1.5 text-[11px] font-medium text-[var(--text-muted)] hover:text-[var(--text-strong)]">{t('overview.exportCsv')}</a>
        <a href={`/api/stats.svg${exportSuffix}`} download="odotoken-stats.svg" className="rounded-md border border-[var(--border)] bg-[var(--bg-panel)] px-3 py-1.5 text-[11px] font-medium text-[var(--text-muted)] hover:text-[var(--text-strong)]">{t('overview.snapshotSvg')}</a>
      </div>
      <div className="grid grid-cols-2 gap-4 lg:grid-cols-6">
        <Kpi label="Total Spent" value={formatCurrency(summary?.total_cost_usd ?? 0, locale)} sub="estimated USD" color="text-[var(--text-strong)]" />
        <Kpi label="Total Tokens" value={formatNumber(summary?.total_tokens ?? 0, locale)} sub="prompt + completion" color="text-[var(--text-strong)]" />
        <Kpi label="Cache Read" value={formatNumber(summary?.total_cache_read_tokens ?? 0, locale)} sub="cached tokens" color="text-[var(--text-strong)]" />
        <Kpi label="Throughput" value={formatTps(tps?.live_tps)} sub={`peak ${formatTps(tps?.peak_tps)} · avg ${formatTps(tps?.avg_tps)}`} color="text-[var(--chart-1)]" />
        <Kpi label="Total Requests" value={formatNumber(summary?.total_requests ?? 0, locale)} sub={`avg ${formatLatency(summary?.avg_latency_ms ?? 0)}`} color="text-[var(--text-strong)]" />
        <Kpi label="Saved vs Cloud" value={formatCurrency(savings?.saved_usd ?? 0, locale)} sub={savings ? `shadow ${formatCurrency(savings.shadow_usd, locale)}` : '—'} color="text-[var(--chart-1)]" />
      </div>

      {/* Consumo Diário + Charts row */}
      <div className="grid grid-cols-1 gap-5 lg:grid-cols-[minmax(280px,380px)_1fr_1fr]">
        <DailyConsumption daily={calendarData?.daily ?? []} />

        <Card>
          <CardHeader title={t('overview.dailyConsumption')} subtitle={t('overview.dailyConsumptionSub')} />
          <div className="h-64 px-1 pb-4">
            <ResponsiveContainer width="100%" height="100%">
              <BarChart data={daily ?? []} margin={{ left: 4, right: 16, top: 4, bottom: 0 }} barCategoryGap="30%">
                <CartesianGrid stroke="var(--border-soft)" strokeDasharray="3 3" vertical={false} />
                <XAxis dataKey="day" stroke="var(--text-soft)" fontSize={10} tickLine={false} axisLine={false} tickFormatter={(v: string) => v.slice(5)} />
                <YAxis yAxisId="left" width={42} tickMargin={6} stroke="var(--text-soft)" fontSize={10} tickLine={false} axisLine={false} tickFormatter={(v: number) => formatNumberCompact(v, locale)} />
                <YAxis yAxisId="right" width={52} tickMargin={6} orientation="right" stroke="var(--text-soft)" fontSize={10} tickLine={false} axisLine={false} tickFormatter={(v: number) => formatCurrency(v, locale)} />
                <Tooltip contentStyle={{ background: 'var(--bg-elevated)', border: '1px solid var(--border)', borderRadius: 8, fontSize: 12, color: 'var(--text-strong)' }} labelStyle={{ color: 'var(--text-soft)' }} />
                <Legend wrapperStyle={{ fontSize: 11, paddingTop: 8 }} />
                <Bar yAxisId="left" dataKey="total_tokens" name="Tokens" fill="var(--chart-2)" radius={[4, 4, 0, 0]} />
                <Line yAxisId="right" type="monotone" dataKey="cost_usd" name="Cost" stroke="var(--chart-1)" strokeWidth={2} dot={{ r: 2, fill: 'var(--chart-1)' }} activeDot={{ r: 4 }} connectNulls />
              </BarChart>
            </ResponsiveContainer>
          </div>
        </Card>

        <Card>
          <CardHeader title={t('overview.costByModel')} subtitle={`${formatCurrency(pieTotal, locale)} total`} />
          <div className="h-64 px-2 pb-2">
            {pieData.length === 1 ? (
              <div className="flex h-full flex-col items-center justify-center gap-1">
                <div className="text-[11px] uppercase tracking-wider text-[var(--text-soft)]">100% · {pieData[0].name}</div>
                <div className="num text-[22px] font-bold text-[var(--text-strong)]">{formatCurrency(pieData[0].value, locale)}</div>
              </div>
            ) : pieData.length > 0 ? (
              <ResponsiveContainer width="100%" height="100%">
                <PieChart>
                  <Pie
                    data={pieData}
                    dataKey="value"
                    nameKey="name"
                    innerRadius={52}
                    outerRadius={78}
                    paddingAngle={2}
                    labelLine={false}
                    label={({ name, percent }) => {
                      const pct = Math.round((percent ?? 0) * 100);
                      if (pct < 5) return '';
                      const labelName = name ?? 'unknown';
                      return (
                        <tspan x={0} dy="0em" fill="currentColor" stroke="var(--bg-panel)" strokeWidth="3" paintOrder="stroke">
                          {labelName.length > 12 ? labelName.slice(0, 10) + '…' : labelName} ({pct}%)
                        </tspan>
                      );
                    }}
                  >
{pieData.map((_, i) => <Cell key={i} fill={PIE_COLORS[i % PIE_COLORS.length]} stroke="var(--bg-panel)" strokeWidth={2} />)}
                  </Pie>
                  <Tooltip contentStyle={{ background: 'var(--bg-elevated)', border: '1px solid var(--border)', borderRadius: 8, fontSize: 12, color: 'var(--text-strong)' }} formatter={(v) => formatCurrency(v as number, locale)} />
                  <Legend wrapperStyle={{ fontSize: 11, paddingTop: 6 }} iconType="circle" verticalAlign="bottom" align="center" />
                </PieChart>
              </ResponsiveContainer>
            ) : (
              <div className="flex h-full items-center justify-center text-[12px] text-[var(--text-faint)]">{t('overview.noCostRecorded')}</div>
            )}
          </div>
        </Card>
      </div>

      {/* Top models + projects — mirrored columns */}
      <div className="grid grid-cols-1 gap-5 lg:grid-cols-2">
        <Card>
          <CardHeader title={t('overview.topModels')} subtitle={t('overview.byTokenVolume')} />
          <SortableTable
            rows={by_model ?? []}
            rowKey={(m: ModelRow) => m.model + m.provider}
            pageSize={8}
            columns={[
             { key: 'model', header: t('common.model'), render: (m: ModelRow) => <span className="font-medium text-[var(--text-strong)]">{m.model || '—'}</span>, sortValue: (m: ModelRow) => m.model },
             { key: 'requests', header: t('common.requests'), align: 'right', render: (m: ModelRow) => <span className="num tabular-nums">{formatNumber(m.requests, locale)}</span>, sortValue: (m: ModelRow) => m.requests },
             { key: 'tokens', header: t('common.tokens'), align: 'right', render: (m: ModelRow) => <span className="num tabular-nums font-medium" title={String(m.total_tokens)}>{formatNumber(m.total_tokens, locale)}</span>, sortValue: (m: ModelRow) => m.total_tokens },
             { key: 'cost', header: t('common.cost'), align: 'right', render: (m: ModelRow) => <span className="num tabular-nums text-[var(--brand-ink)]">{formatCurrency(m.cost_usd, locale)}</span>, sortValue: (m: ModelRow) => m.cost_usd },
            ]}
          />
        </Card>

        <Card>
          <CardHeader title={t('overview.projects')} subtitle={t('overview.costPerProjectTag')} />
          <SortableTable
            rows={by_project ?? ([] as ProjectRow[])}
            rowKey={(p: ProjectRow) => p.project_id}
            pageSize={8}
            columns={[
             { key: 'project', header: t('common.project'), render: (p: ProjectRow) => <Badge>{p.project_id}</Badge>, sortValue: (p: ProjectRow) => p.project_id },
             { key: 'requests', header: t('common.requests'), align: 'right', render: (p: ProjectRow) => <span className="num tabular-nums">{formatNumber(p.requests, locale)}</span>, sortValue: (p: ProjectRow) => p.requests },
             { key: 'tokens', header: t('common.tokens'), align: 'right', render: (p: ProjectRow) => <span className="num tabular-nums" title={String(p.total_tokens)}>{formatNumber(p.total_tokens, locale)}</span>, sortValue: (p: ProjectRow) => p.total_tokens },
             { key: 'cost', header: t('common.cost'), align: 'right', render: (p: ProjectRow) => <span className="num tabular-nums text-[var(--brand-ink)]">{formatCurrency(p.cost_usd, locale)}</span>, sortValue: (p: ProjectRow) => p.cost_usd },
             { key: 'shadow', header: t('overview.shadowCost'), align: 'right', render: (p: ProjectRow) => <span className="num tabular-nums text-[var(--text-soft)]">{formatCurrency(p.shadow_cost_usd, locale)}</span>, sortValue: (p: ProjectRow) => p.shadow_cost_usd },
            ]}
          />
        </Card>
      </div>

      {/* By Source */}
      {data.by_source && data.by_source.length > 0 && (
        <Card>
          <CardHeader title={t('overview.bySource')} subtitle={t('overview.proxyVsImported')} />
          <SortableTable
            rows={data.by_source}
            rowKey={(s: SourceRow) => s.source}
            columns={[
{ key: 'source', header: t('common.source'), render: (s: SourceRow) => <Badge tone={s.source === 'proxy' ? 'brand' : 'neutral'}>{s.source}</Badge>, sortValue: (s: SourceRow) => s.source },
{ key: 'requests', header: t('common.requests'), align: 'right', render: (s: SourceRow) => <span className="num tabular-nums">{formatNumber(s.requests, locale)}</span>, sortValue: (s: SourceRow) => s.requests },
{ key: 'tokens', header: t('common.tokens'), align: 'right', render: (s: SourceRow) => <span className="num tabular-nums">{formatNumber(s.total_tokens, locale)}</span>, sortValue: (s: SourceRow) => s.total_tokens },
{ key: 'cost', header: t('common.cost'), align: 'right', render: (s: SourceRow) => <span className="num tabular-nums text-[var(--brand-ink)]">{formatCurrency(s.cost_usd, locale)}</span>, sortValue: (s: SourceRow) => s.cost_usd },
            ]}
          />
        </Card>
      )}

      {/* In-flight model requests */}
      <Card>
        <CardHeader title={t('overview.liveRuns')} subtitle={t('overview.liveRunsSub')} right={<Badge tone="brand">{t('common.live')} · 2s</Badge>} />
        {!liveSessions || liveSessions.length === 0 ? (
          <div className="px-4 pb-4 text-[12px] text-[var(--text-faint)]">{t('overview.noInflight')}</div>
        ) : (
          <div className="overflow-x-auto px-2 pb-3">
            <table className="w-full text-start text-[12px]">
              <thead>
                <tr className="border-b border-[var(--border)] text-[11px] uppercase tracking-wider text-[var(--text-faint)]">
                  <th className="py-2 ps-2 pe-3 font-semibold">{t('events.app')}</th>
                  <th className="py-2 px-3 font-semibold">{t('common.provider')}</th>
                  <th className="py-2 px-3 font-semibold">{t('common.model')}</th>
                  <th className="py-2 px-3 text-end font-semibold">{t('common.tokens')}</th>
                  <th className="py-2 px-3 text-end font-semibold">{t('common.cost')}</th>
                  <th className="py-2 px-3 text-end font-semibold">{t('overview.elapsed')}</th>
                  <th className="py-2 ps-3 pe-2 text-end font-semibold">{t('overview.lastActivity')}</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-[var(--border)]">
               {liveSessions.map((s: LiveSession) => (
                  <tr key={s.cli + s.session_id} className="hover:bg-[var(--bg-fill)]">
                    <td className="py-2 ps-2 pe-3"><Badge tone="brand">{s.cli}</Badge></td>
                    <td className="py-2 px-3">{s.provider}</td>
                    <td className="py-2 px-3 font-medium text-[var(--text-strong)]">{s.model}</td>
                    <td className="py-2 px-3 text-end num tabular-nums">{formatNumber(s.tokens_input + s.tokens_output, locale)}</td>
                    <td className="py-2 px-3 text-end num tabular-nums text-[var(--brand-ink)]">{formatCurrency(s.cost_usd, locale)}</td>
                    <td className="py-2 px-3 text-end num tabular-nums">{formatLatency(s.elapsed_ms)}</td>
                    <td className="py-2 pe-2 ps-3 text-end text-[11px] text-[var(--text-soft)]">{s.last_updated ? formatTimestamp(s.last_updated, timezone, locale) : '—'}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </Card>

      {/* Recent events */}
      <Card>
        <CardHeader title={t('overview.recentRequests')} subtitle={t('overview.latest50')} right={<Badge tone="brand">{t('overview.refreshesEvery10s')}</Badge>} />
        <SortableTable rows={events} rowKey={(e: EventRow) => e.id} columns={eventColumns} pageSize={10} />
      </Card>
    </div>
  )
}