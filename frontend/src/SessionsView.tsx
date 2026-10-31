import { useMemo, useState } from 'react'
import { useSessionEvents, useSessions } from './api'
import { formatCurrency, formatNumber, formatTimestamp, sinceForFilter } from './format'
import { Card, CardHeader, Badge, Empty } from './ui/Card'
import { SortableTable, type Column } from './ui/SortableTable'
import { Dialog } from './ui/Dialog'
import type { SessionCost } from './types'
import { useTimezone } from './context/useTimezone'
import { useLocale } from './i18n/useLocale'

/**
 * Cost per conversation. The session key comes from the proxy's X-Odo-Session
 * header or, for imported transcripts, from the source file, so one row is one
 * conversation rather than one request.
 */
export default function SessionsView({ timeFilter, project, source, tag }: { timeFilter: string; project: string; source: string; tag: string }) {
  const { timezone } = useTimezone()
  const { t, locale } = useLocale()
  const since = useMemo(() => sinceForFilter(timeFilter, timezone), [timeFilter, timezone])
  const { data, isLoading, isError, refetch } = useSessions(since, project, source, tag, 200)
  const [selected, setSelected] = useState<SessionCost | null>(null)
  const detail = useSessionEvents(selected ? selected.session_id || selected.session_file || null : null)
  const exportHref = `/api/events.csv${since ? `?since=${encodeURIComponent(since)}` : ''}`

  if (isLoading) return <div className="p-5"><div className="h-64 animate-pulse rounded-[var(--radius)] bg-[var(--bg-fill-tertiary)]" /></div>
  if (isError || !data) return <div className="p-5"><Card><Empty label={t('sessions.loadFailed')} hint={t('error.sessions.unavailable')} onRetry={() => { void refetch() }} /></Card></div>
  if (data.length === 0) return <div className="p-5"><Card><Empty label={t('sessions.empty')} hint={t('models.changeRange')} /></Card></div>

  const columns: Column<SessionCost>[] = [
    { key: 'session', header: t('sessions.columnSession'), render: (s) => <span className="font-medium text-[var(--text-strong)]">{s.session_id || s.session_file || '—'}</span>, sortValue: (s) => s.session_id },
    { key: 'client', header: t('clients.clientApp'), render: (s) => <Badge>{s.client_app || '—'}</Badge>, sortValue: (s) => s.client_app },
    { key: 'project', header: t('common.project'), render: (s) => s.project_id || '—', sortValue: (s) => s.project_id },
    { key: 'models', header: t('common.model'), render: (s) => <span className="text-[var(--text-muted)]">{s.models || '—'}</span>, sortValue: (s) => s.models },
    { key: 'requests', header: t('common.requests'), align: 'right', render: (s) => <span className="num tabular-nums">{formatNumber(s.requests, locale)}</span>, sortValue: (s) => s.requests },
    { key: 'tokens', header: t('common.tokens'), align: 'right', render: (s) => <span className="num tabular-nums">{formatNumber(s.total_tokens, locale)}</span>, sortValue: (s) => s.total_tokens },
    { key: 'cost', header: t('common.cost'), align: 'right', render: (s) => <span className="num tabular-nums font-semibold text-[var(--brand-ink)]">{formatCurrency(s.cost_usd, locale)}</span>, sortValue: (s) => s.cost_usd },
    { key: 'window', header: t('sessions.columnWindow'), render: (s) => <span className="num text-[var(--text-soft)]">{formatTimestamp(s.started_at, timezone, locale)}</span>, sortValue: (s) => s.started_at },
  ]

  return (
    <div className="p-5">
      <Card>
        <CardHeader
          title={t('nav.sessions')}
          subtitle={t('sessions.subtitle')}
          right={<a href={exportHref} download="odotoken-events.csv" className="rounded-md border border-[var(--border)] bg-[var(--bg-panel)] px-3 py-1.5 text-[11px] font-medium text-[var(--text-muted)] hover:text-[var(--text-strong)]">{t('events.exportCsv')}</a>}
        />
        <SortableTable
          rows={data}
          rowKey={(s) => s.session_id || s.session_file || ''}
          columns={columns}
          pageSize={25}
          searchable
          searchKeys={['session_id', 'client_app', 'project_id', 'models']}
          onRowClick={setSelected}
          activeRowKey={selected?.session_id}
        />
      </Card>

      <Dialog open={selected !== null} onClose={() => setSelected(null)} title={t('sessions.detailTitle', { id: selected?.session_id || selected?.session_file || '' })}>
        {selected && (
          <div className="space-y-3">
            <div className="grid grid-cols-2 gap-2 text-[12px] sm:grid-cols-4">
              {[
                [t('common.cost'), formatCurrency(selected.cost_usd, locale)],
                [t('common.requests'), formatNumber(selected.requests, locale)],
                [t('common.tokens'), formatNumber(selected.total_tokens, locale)],
                [t('overview.cachedTokens'), formatNumber(selected.cache_read_tokens, locale)],
              ].map(([label, value]) => (
                <div key={label} className="rounded-md border border-[var(--border-soft)] bg-[var(--bg-fill-tertiary)] px-3 py-2">
                  <div className="text-[10px] uppercase tracking-wider text-[var(--text-soft)]">{label}</div>
                  <div className="num mt-0.5 font-medium text-[var(--text-strong)]">{value}</div>
                </div>
              ))}
            </div>
            <div>
              <div className="mb-1 text-[10px] uppercase tracking-wider text-[var(--text-soft)]">{t('common.model')}</div>
              {!detail.data?.events?.length ? (
                <div className="text-[11px] text-[var(--text-faint)]">{detail.isLoading ? t('common.loading') : t('sessions.noEvents')}</div>
              ) : (
                <div className="max-h-64 overflow-auto">
                  <table className="w-full text-[11px]">
                    <thead className="sticky top-0 bg-[var(--bg-elevated)]">
                      <tr className="border-b border-[var(--border)] text-[10px] uppercase tracking-wider text-[var(--text-faint)]">
                        <th scope="col" className="py-2 pe-3 text-start font-semibold">{t('common.model')}</th>
                        <th scope="col" className="py-2 pe-3 text-end font-semibold">{t('common.tokens')}</th>
                        <th scope="col" className="py-2 text-end font-semibold">{t('common.cost')}</th>
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-[var(--border)]">
                      {detail.data.events.map((e) => (
                        <tr key={e.id}>
                          <td className="py-1.5 pe-3 text-[var(--text-strong)]">{e.model || '—'}</td>
                          <td className="num py-1.5 pe-3 text-end tabular-nums">{formatNumber(e.total_tokens, locale)}</td>
                          <td className="num py-1.5 text-end tabular-nums">{formatCurrency(e.cost_usd, locale)}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              )}
            </div>
          </div>
        )}
      </Dialog>
    </div>
  )
}
