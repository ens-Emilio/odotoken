import { useDailyDetail } from '../api'
import { formatCurrency, formatNumber, formatLatency } from '../format'
import { Card, CardHeader, Badge } from './Card'
import { SortableTable, type Column } from './SortableTable'
import type { ClientRow, SourceRow } from '../types'

interface Props {
  day: string
  tz: string
  project: string
  source: string
  tag: string
  onClose: () => void
}

function Kpi({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded border border-[var(--border)] bg-[var(--bg-fill)] px-2 py-1.5">
      <div className="text-[9px] font-semibold uppercase tracking-wider text-[var(--text-soft)]">{label}</div>
      <div className="num mt-0.5 text-sm font-bold tabular-nums text-[var(--text-strong)]">{value}</div>
    </div>
  )
}

const clientColumns: Column<ClientRow>[] = [
  { key: 'client_app', header: 'Cliente / IDE', align: 'left', render: (r) => <Badge tone="brand">{r.client_app || 'unknown'}</Badge> },
  { key: 'requests', header: 'Requisições', align: 'right', render: (r) => formatNumber(r.requests) },
  { key: 'total_tokens', header: 'Tokens', align: 'right', render: (r) => formatNumber(r.total_tokens) },
  { key: 'cost_usd', header: 'Custo', align: 'right', render: (r) => <span className="text-[var(--brand-ink)]">{formatCurrency(r.cost_usd)}</span> },
]

const sourceColumns: Column<SourceRow>[] = [
  { key: 'source', header: 'Fonte', align: 'left', render: (r) => <span className="font-medium text-[var(--text-strong)]">{r.source}</span> },
  { key: 'requests', header: 'Requisições', align: 'right', render: (r) => formatNumber(r.requests) },
  { key: 'total_tokens', header: 'Tokens', align: 'right', render: (r) => formatNumber(r.total_tokens) },
  { key: 'cost_usd', header: 'Custo', align: 'right', render: (r) => <span className="text-[var(--brand-ink)]">{formatCurrency(r.cost_usd)}</span> },
]

export default function DayDetailPanel({ day, tz, project, source, tag, onClose }: Props) {
  const { data, isLoading, error } = useDailyDetail(day, tz, project, source, tag)

  return (
    <Card>
      <CardHeader
        title={`Detalhes de ${day}`}
        subtitle="consumo por cliente e fonte"
        right={
          <button
            onClick={onClose}
            className="rounded px-2 py-1 text-xs text-[var(--text-soft)] hover:bg-[var(--bg-fill)] hover:text-[var(--text-strong)]"
          >
            ✕ Fechar
          </button>
        }
      />
      <div className="space-y-2 px-3 pb-2">
        {isLoading && (
          <div className="space-y-3">
            <div className="grid grid-cols-2 gap-1.5 sm:grid-cols-4">
              {[1, 2, 3, 4].map((i) => (
                <div key={i} className="h-8 animate-pulse rounded bg-[var(--bg-fill-secondary)]" />
              ))}
            </div>
            <div className="h-16 animate-pulse rounded bg-[var(--bg-fill-secondary)]" />
          </div>
        )}

        {error && (
          <div className="rounded border border-red-500/30 bg-red-500/10 p-2 text-xs text-red-400">
            Erro ao carregar detalhes: {(error as Error).message}
          </div>
        )}

        {data && !isLoading && (
          <>
            <div className="grid grid-cols-2 gap-1.5 sm:grid-cols-4">
              <Kpi label="Custo Total" value={formatCurrency(data.summary.total_cost_usd)} />
              <Kpi label="Tokens Totais" value={formatNumber(data.summary.total_tokens)} />
              <Kpi label="Requisições" value={formatNumber(data.summary.total_requests)} />
              <Kpi label="Latência Média" value={formatLatency(data.summary.avg_latency_ms)} />
            </div>

            <div className="space-y-0.5">
              <h3 className="text-[10px] font-semibold uppercase tracking-wider text-[var(--text-soft)]">Por Cliente / IDE</h3>
              {data.by_client.length > 0 ? (
                <SortableTable rows={data.by_client} rowKey={(r) => r.client_app} columns={clientColumns} pageSize={10} />
              ) : (
                <div className="py-2 text-center text-xs text-[var(--text-soft)]">Nenhum registro de cliente neste dia.</div>
              )}
            </div>

            <div className="space-y-0.5">
              <h3 className="text-[10px] font-semibold uppercase tracking-wider text-[var(--text-soft)]">Por Fonte</h3>
              {data.by_source.length > 0 ? (
                <SortableTable rows={data.by_source} rowKey={(r) => r.source} columns={sourceColumns} pageSize={10} />
              ) : (
                <div className="py-2 text-center text-xs text-[var(--text-soft)]">Nenhum registro de fonte neste dia.</div>
              )}
            </div>
          </>
        )}
      </div>
    </Card>
  )
}