import { useAnomalies } from './api'
import { formatCurrency } from './format'
import { Card, CardHeader, Badge, Empty } from './ui/Card'
import { SortableTable, type Column } from './ui/SortableTable'
import { useTimezone } from './context/useTimezone'
import { useLocale } from './i18n/useLocale'
import type { AttributedAnomaly, AnomalyAttribution } from './types'

/**
 * An anomalous day is a fact; the useful part is the attribution. Each row
 * names the models and projects whose spend moved away from their own baseline,
 * ordered by dollars added, so the reader sees what to look at first.
 */
export default function AnomaliesView({ project, source, tag }: { project: string; source: string; tag: string }) {
  const { timezone } = useTimezone()
  const { t, locale } = useLocale()
  const { data, isLoading, isError, refetch } = useAnomalies(project, source, tag, timezone)

  if (isLoading) return <div className="p-5"><div className="h-64 animate-pulse rounded-[var(--radius)] bg-[var(--bg-fill-tertiary)]" /></div>
  if (isError || !data) return <div className="p-5"><Card><Empty label={t('anomalies.loadFailed')} hint={t('error.stats.unavailable')} onRetry={() => { void refetch() }} /></Card></div>
  if (data.anomalies.length === 0) return <div className="p-5"><Card><Empty label={t('anomalies.empty')} hint={t('anomalies.explanation')} /></Card></div>

  const columns: Column<AttributedAnomaly>[] = [
    { key: 'day', header: t('anomalies.columnDay'), render: (a) => <span className="num font-medium text-[var(--text-strong)]">{a.day}</span>, sortValue: (a) => a.day },
    { key: 'cost', header: t('anomalies.columnCost'), align: 'right', render: (a) => <span className="num tabular-nums font-semibold text-[var(--danger-ink)]">{formatCurrency(a.cost_usd, locale)}</span>, sortValue: (a) => a.cost_usd },
    { key: 'threshold', header: t('anomalies.columnThreshold'), align: 'right', render: (a) => <span className="num tabular-nums text-[var(--text-soft)]">{formatCurrency(a.threshold, locale)}</span>, sortValue: (a) => a.threshold },
    { key: 'z', header: t('anomalies.columnZ'), align: 'right', render: (a) => <Badge tone="danger">{a.z_score.toFixed(2)}σ</Badge>, sortValue: (a) => a.z_score },
  ]

  return (
    <div className="space-y-5 p-5">
      <Card>
        <CardHeader title={t('nav.anomalies')} subtitle={t('anomalies.subtitle')} right={<span className="num text-[11px] text-[var(--text-faint)]">{data.generated_at}</span>} />
        <SortableTable rows={data.anomalies} rowKey={(a) => a.day} columns={columns} pageSize={25} />
      </Card>
      {data.anomalies.map((a) => (
        <Card key={a.day}>
          <CardHeader title={t('anomalies.detailTitle', { day: a.day })} subtitle={t('anomalies.explanation')} />
          <div className="grid grid-cols-1 gap-5 px-5 pb-5 lg:grid-cols-2">
            <AttributionList title={t('anomalies.byModel')} rows={a.by_model} />
            <AttributionList title={t('anomalies.byProject')} rows={a.by_project} />
          </div>
        </Card>
      ))}
    </div>
  )
}

function AttributionList({ title, rows }: { title: string; rows: AnomalyAttribution[] }) {
  const { t, locale } = useLocale()
  if (rows.length === 0) return <div><div className="mb-1 text-[10px] uppercase tracking-wider text-[var(--text-faint)]">{title}</div><div className="text-[11px] text-[var(--text-faint)]">{t('common.noRecords')}</div></div>
  return (
    <div>
      <div className="mb-1.5 text-[10px] uppercase tracking-wider text-[var(--text-faint)]">{title}</div>
      <div className="space-y-1.5">
        {rows.map((r) => (
          <div key={r.key} className="flex items-center justify-between gap-3 text-[12px]">
            <span className="min-w-0 truncate text-[var(--text-strong)]">{r.key || '—'}</span>
            <span className="flex shrink-0 items-center gap-3 tabular-nums">
              <span className="num text-[var(--text-soft)]">{r.share_pct.toFixed(1)}%</span>
              <span className={`num ${r.delta_usd > 0 ? 'text-[var(--danger-ink)]' : 'text-[var(--brand-ink)]'}`}>
                {r.delta_usd > 0 ? '+' : ''}{formatCurrency(r.delta_usd, locale)}
              </span>
              <span className="num w-20 text-end text-[var(--text-strong)]">{formatCurrency(r.cost_usd, locale)}</span>
            </span>
          </div>
        ))}
      </div>
    </div>
  )
}
