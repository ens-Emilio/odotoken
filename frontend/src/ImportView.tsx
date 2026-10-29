import { useCallback, useEffect, useRef, useState } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Card, CardHeader, Badge, Empty } from './ui/Card'
import { Button } from './ui/Button'
import { formatCurrencyExact, formatNumber, formatTimestamp } from './format'
import { useTimezone } from './context/useTimezone'
import { toast } from './ui/toast-store'
import { useLocale } from './i18n/useLocale'

interface Source {
  cli: string
  available: boolean
  path: string
  default_path: string
  override?: string
  path_exists: boolean
  session_files: number
  first_modified?: string
  last_modified?: string
  last_imported?: string
  last_inserted?: number
}

interface CLIReport {
  cli: string
  scanned: number
  inserted: number
  duplicates: number
  tokens: number
  cost_usd: number
  priced_by_catalog: number
  priced_by_provider: number
  unpriced: number
  errors: number
  error?: string
}

interface Report {
  started_at: string
  finished_at: string
  dry_run: boolean
  since?: string
  clis: CLIReport[]
  inserted: number
  duplicates: number
  tokens: number
  cost_usd: number
  unpriced: number
  errors: number
  status: 'done' | 'partial' | 'cancelled'
}

interface Progress {
  cli: string
  scanned: number
  inserted: number
  index: number
  total: number
}

interface StatusResponse {
  job_id: string
  done: boolean
  progress: Progress
  report: Report
  error?: string
}

interface RunRecord {
  id: string
  started_at: string
  finished_at?: string
  status: string
  dry_run: boolean
  clis: string
  inserted: number
  duplicates: number
  cost_usd: number
  error?: string
}

// Coverage windows are chosen, not typed: the honest question is "how far back",
// and an RFC3339 text box answers it only for people who already know the format.
const RANGES: { value: string; labelKey: string; days: number }[] = [
  { value: '7d', labelKey: 'import.last7', days: 7 },
  { value: '30d', labelKey: 'import.last30', days: 30 },
  { value: '90d', labelKey: 'import.last90', days: 90 },
  { value: 'all', labelKey: 'import.everything', days: 0 },
]

function sinceFor(value: string): string {
  const range = RANGES.find((r) => r.value === value)
  if (!range || range.days === 0) return ''
  const d = new Date()
  d.setUTCDate(d.getUTCDate() - range.days)
  return d.toISOString().replace(/\.\d{3}Z$/, 'Z')
}

export default function ImportView() {
  const { timezone } = useTimezone()
  const { t, locale } = useLocale()
  const queryClient = useQueryClient()
  const [range, setRange] = useState('30d')
  const [query, setQuery] = useState('')
  const [selected, setSelected] = useState<Set<string>>(new Set())
  const [paths, setPaths] = useState<Record<string, string>>({})
  const [savingPath, setSavingPath] = useState<string | null>(null)
  const [jobId, setJobId] = useState<string | null>(null)
  const [status, setStatus] = useState<StatusResponse | null>(null)
  const [preview, setPreview] = useState<Report | null>(null)
  const [busy, setBusy] = useState(false)
  const pollRef = useRef<number | null>(null)
  const since = sinceFor(range)

  const { data, isLoading, isError, error, refetch } = useQuery<{ sources: Source[] }>({
    queryKey: ['import-sources', since],
    queryFn: async () => {
      const res = await fetch(`/api/import/sources${since ? `?since=${encodeURIComponent(since)}` : ''}`)
      if (!res.ok) throw new Error(`sources: ${res.status}`)
      return res.json()
    },
    staleTime: 15_000,
    retry: false,
  })

  const history = useQuery<{ runs: RunRecord[] }>({
    queryKey: ['import-history'],
    queryFn: async () => {
      const res = await fetch('/api/import/history?limit=8')
      if (!res.ok) throw new Error(`history: ${res.status}`)
      return res.json()
    },
    staleTime: 10_000,
  })

  const sources = data?.sources ?? []
  const filteredSources = sources.filter((source) => {
    const term = query.trim().toLowerCase()
    return !term || `${source.cli} ${source.path}`.toLowerCase().includes(term)
  })
  const ready = sources.filter((s) => s.available && s.session_files > 0)
  // Default the selection to everything importable, once, so the common case is
  // one click. Re-seeding on every refetch would undo the user's deselection.
  const seeded = useRef(false)
  useEffect(() => {
    if (seeded.current || ready.length === 0) return
    seeded.current = true
    setSelected(new Set(ready.map((s) => s.cli)))
  }, [ready])

  const stopPolling = useCallback(() => {
    if (pollRef.current !== null) {
      window.clearInterval(pollRef.current)
      pollRef.current = null
    }
  }, [])

  useEffect(() => stopPolling, [stopPolling])

  const startPolling = useCallback(
    (id: string) => {
      stopPolling()
      pollRef.current = window.setInterval(async () => {
        try {
          const res = await fetch(`/api/import/status?job_id=${encodeURIComponent(id)}`)
          if (!res.ok) throw new Error(String(res.status))
          const body: StatusResponse = await res.json()
          setStatus(body)
          if (body.done) {
            stopPolling()
            setBusy(false)
            void refetch()
            void queryClient.invalidateQueries({ queryKey: ['import-history'] })
          }
        } catch {
          stopPolling()
          setBusy(false)
          toast('error', t('import.lostTrack'))
        }
      }, 400)
    },
    [queryClient, refetch, stopPolling, t],
  )

  async function startImport(dryRun: boolean) {
    setBusy(true)
    setStatus(null)
    setPreview(null)
    try {
      const res = await fetch('/api/import/run', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ clis: [...selected], since, dry_run: dryRun }),
      })
      const body = await res.json()
      if (!res.ok) throw new Error(body.code ? t(body.code, body.params) : body.error ?? `run: ${res.status}`)
      setJobId(body.job_id)
      startPolling(body.job_id)
    } catch (err) {
      setBusy(false)
      toast('error', err instanceof Error ? err.message : t('import.startFailed'))
    }
  }

  async function cancel() {
    if (!jobId) return
    await fetch(`/api/import/cancel?job_id=${encodeURIComponent(jobId)}`, { method: 'POST' })
  }

  // Saving a path writes config.json, so it stays explicit: Enter or Save
  // applies it, and the list is refetched so the row shows the real root.
  async function savePath(cli: string, value: string) {
    const current = sources.find((s) => s.cli === cli)
    const next = value.trim()
    const clearDraft = () =>
      setPaths((prev) => {
        const rest = { ...prev }
        delete rest[cli]
        return rest
      })
    if (next === (current?.override ?? '')) {
      clearDraft()
      return
    }
    setSavingPath(cli)
    try {
      const res = await fetch('/api/import/path', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ cli, path: next }),
      })
      const body = await res.json()
      if (!res.ok) throw new Error(body.code ? t(body.code, body.params) : body.error ?? `save: ${res.status}`)
      clearDraft()
      await refetch()
      toast(
        body.path_exists ? 'ok' : 'error',
        body.path_exists ? t('import.nowReads', { cli, path: body.path }) : t('import.savedMissing', { path: body.path }),
      )
    } catch (err) {
      toast('error', err instanceof Error ? err.message : t('import.pathSaveFailed'))
    } finally {
      setSavingPath(null)
    }
  }

  // A dry run is a preview: keep its report beside the controls instead of
  // replacing the live-progress area, which belongs to the real run.
  useEffect(() => {
    if (status?.done && status.report?.dry_run) setPreview(status.report)
  }, [status])

  const toggle = (cli: string) => {
    setSelected((prev) => {
      const next = new Set(prev)
      if (next.has(cli)) next.delete(cli)
      else next.add(cli)
      return next
    })
  }

  const live = status && !status.done ? status.progress : null
  const result = status?.done && !status.report?.dry_run ? status.report : null
  const nothingSelected = selected.size === 0

  return (
    <div className="space-y-5 p-5">
      <Card>
        <CardHeader
          title={t('import.sources')}
          subtitle={since ? t('import.countingWindow') : t('import.everySession')}
          right={
            <Button onClick={() => void refetch()}>{t('common.rescan')}</Button>
          }
        />
        <div className="flex flex-wrap items-end justify-between gap-3 px-5 pt-1">
          <label className="min-w-48 flex-1 text-[11px] text-[var(--text-muted)]">
            {t('import.searchCli')}
            <input
              type="search"
              value={query}
              onChange={(event) => setQuery(event.target.value)}
              placeholder={t('import.searchPlaceholder')}
              className="mt-1 block h-8 w-full rounded-md border border-[var(--border)] bg-[var(--bg)] px-2.5 text-[12px] text-[var(--text-strong)] outline-none focus:ring-2 focus:ring-[var(--brand)]"
            />
          </label>
          <span className="pb-1.5 text-[11px] text-[var(--text-faint)]">
            {filteredSources.length} of {sources.length}
          </span>
        </div>
        <div className="px-5 pb-5 pt-3">
          {isError ? (
            <Empty
              label={t('import.reachFailed')}
              hint={`${error instanceof Error ? error.message : t('import.requestFailed')} — ${t('error.ui.unknown')}`}
              onRetry={() => void refetch()}
            />
          ) : isLoading ? (
            <div className="h-32 animate-pulse rounded-[var(--radius)] bg-[var(--bg-fill-tertiary)]" />
          ) : sources.length === 0 ? (
            <Empty label={t('import.noSessionData')} hint={t('import.everySession')} onRetry={() => void refetch()} />
          ) : filteredSources.length === 0 ? (
            <Empty label={t('import.noSourceMatch')} hint={t('common.clear')} />
          ) : (
            <ul className="divide-y divide-[var(--border)]">
              {filteredSources.map((s) => {
                const usable = s.available && s.session_files > 0
                return (
                  <li key={s.cli} className="flex items-start gap-3 py-3">
                    <input
                      id={`cli-${s.cli}`}
                      type="checkbox"
                      checked={selected.has(s.cli)}
                      onChange={() => toggle(s.cli)}
                      disabled={!usable}
                      className="mt-0.5 h-3.5 w-3.5 shrink-0 accent-[var(--brand)] disabled:opacity-40"
                    />
                    <div className="min-w-0 flex-1">
                      <label htmlFor={`cli-${s.cli}`} className="block cursor-pointer">
                        <div className="flex flex-wrap items-center gap-2">
                          <span className="text-[12px] font-medium text-[var(--text-strong)]">{s.cli}</span>
                          {!s.available && <Badge>{t('import.notDetected')}</Badge>}
                          {s.available && s.session_files === 0 && <Badge tone="warn">{t('import.noSessionsInRange')}</Badge>}
                          {s.override && <Badge tone="brand">{t('import.customPath')}</Badge>}
                          {s.last_imported && (
                            <span className="text-[11px] text-[var(--text-faint)]">
                              {t('import.lastImport', { time: formatTimestamp(s.last_imported, timezone, locale) })}
                              {typeof s.last_inserted === 'number' ? ` · ${formatNumber(s.last_inserted, locale)} ${t('common.new')}` : ''}
                            </span>
                          )}
                        </div>
                        <div className="mt-0.5 truncate font-mono text-[11px] text-[var(--text-faint)]" title={s.path}>
                          {s.path}
                        </div>
                        {s.session_files > 0 && (
                          <div className="mt-0.5 text-[11px] text-[var(--text-soft)]">
                            {formatNumber(s.session_files, locale)} {t('import.allDetected')}
                            {s.first_modified && s.last_modified
                              ? ` · ${formatTimestamp(s.first_modified, timezone, locale)} → ${formatTimestamp(s.last_modified, timezone, locale)}`
                              : ''}
                          </div>
                        )}
                      </label>
                      <div className="mt-2 flex flex-wrap items-center gap-2">
                        <input
                          aria-label={t('import.dataDirectory', { cli: s.cli })}
                          value={paths[s.cli] ?? s.override ?? ''}
                          placeholder={s.default_path}
                          onChange={(event) => setPaths((prev) => ({ ...prev, [s.cli]: event.target.value }))}
                          onKeyDown={(event) => {
                            if (event.key === 'Enter') void savePath(s.cli, event.currentTarget.value)
                          }}
                          className="min-w-0 flex-1 rounded-md border border-[var(--border)] bg-[var(--bg)] px-2 py-1 font-mono text-[11px] text-[var(--text-strong)] outline-none focus:ring-2 focus:ring-[var(--brand)]"
                        />
                        <Button onClick={() => void savePath(s.cli, paths[s.cli] ?? s.override ?? '')} disabled={savingPath === s.cli}>
                          {savingPath === s.cli ? t('common.saving') : t('import.savePath')}
                        </Button>
                        {s.override && (
                          <Button onClick={() => void savePath(s.cli, '')} disabled={savingPath === s.cli}>
                            {t('import.useDefault')}
                          </Button>
                        )}
                      </div>
                    </div>
                  </li>
                )
              })}
            </ul>
          )}
        </div>
      </Card>

      <Card>
        <CardHeader title={t('import.title')} subtitle={t('import.titleSub')} />
        <div className="space-y-4 px-5 pb-5">
          <div className="flex flex-wrap items-center gap-2">
            <span className="text-[11px] text-[var(--text-muted)]">{t('import.range')}</span>
            {RANGES.map((r) => (
              <button
                key={r.value}
                type="button"
                onClick={() => setRange(r.value)}
                aria-pressed={range === r.value}
                className={`rounded-md border px-2.5 py-1 text-[11px] font-medium transition-colors focus:outline-none focus:ring-2 focus:ring-[var(--brand)] ${
                  range === r.value
                    ? 'border-transparent bg-[var(--bg-fill-secondary)] text-[var(--text-strong)]'
                    : 'border-[var(--border)] text-[var(--text-muted)] hover:text-[var(--text-strong)]'
                }`}
              >
                {t(r.labelKey)}
              </button>
            ))}
          </div>

          <div className="flex flex-wrap items-center gap-2">
            <Button onClick={() => void startImport(true)} disabled={busy || nothingSelected} variant="secondary">
              Preview
            </Button>
            <Button onClick={() => void startImport(false)} disabled={busy || nothingSelected} variant="primary">
              Import {selected.size > 0 ? `${selected.size} ${selected.size === 1 ? 'source' : 'sources'}` : ''}
            </Button>
            {busy && (
              <Button variant="secondary" onClick={() => void cancel()}>{t('common.cancel')}</Button>
            )}
            {nothingSelected && <span className="text-[11px] text-[var(--text-faint)]">{t('import.selectAtLeastOne')}</span>}
          </div>

          {live && (
            <div className="text-[11px] text-[var(--text-muted)]">
              {live.cli} · {t('import.step', { index: live.index, total: live.total })} · {t('import.scanned', { count: formatNumber(live.scanned, locale) })} · {formatNumber(live.inserted, locale)} {t('common.new')}
            </div>
          )}
          {/* A cancelled run is a choice, not a failure: the report says so. */}
          {status?.error && status.report?.status !== 'cancelled' && (
            <div className="text-[11px] text-[var(--danger-ink)]">{status.error}</div>
          )}
        </div>
      </Card>

      {preview && (
        <Card>
          <CardHeader title={t('import.previewTitle')} subtitle={t('import.previewSub')} right={<Badge tone="brand">{t('common.dryRun')}</Badge>} />
          <div className="px-5 pb-5">
            <ReportTable report={preview} />
          </div>
        </Card>
      )}

      {result && (
        <Card>
          <CardHeader title={result.status === 'cancelled' ? t('import.cancelled') : t('import.imported')} subtitle={formatTimestamp(result.finished_at, timezone)} right={<Badge tone={result.status === 'done' ? 'brand' : 'warn'}>{result.status}</Badge>} />
          <div className="px-5 pb-5">
            <ReportTable report={result} />
          </div>
        </Card>
      )}

      <Card>
        <CardHeader title={t('import.pastImports')} subtitle={t('import.newestFirst')} />
        <div className="px-5 pb-5">
          {!history.data || history.data.runs.length === 0 ? (
            <Empty label={t('import.noImports')} hint={t('import.previewSub')} />
          ) : (
            <table className="w-full text-[11px]">
              <thead>
                <tr className="text-start text-[var(--text-faint)]">
                  <th className="pb-1 font-medium">{t('import.started')}</th>
                  <th className="pb-1 font-medium">{t('common.sources')}</th>
                  <th className="pb-1 text-end font-medium">{t('common.new')}</th>
                  <th className="pb-1 text-end font-medium">{t('common.duplicates')}</th>
                  <th className="pb-1 text-end font-medium">{t('common.cost')}</th>
                  <th className="pb-1 text-end font-medium">{t('common.status')}</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-[var(--border)]">
                {history.data.runs.map((run) => (
                  <tr key={run.id} className="text-[var(--text-muted)]">
                    <td className="py-1.5 text-[var(--text-strong)]">{formatTimestamp(run.started_at, timezone)}</td>
                    <td className="py-1.5">{run.clis || t('import.allDetected')}</td>
                    <td className="num py-1.5 text-end">{formatNumber(run.inserted, locale)}</td>
                    <td className="num py-1.5 text-end">{formatNumber(run.duplicates, locale)}</td>
                    <td className="num py-1.5 text-end">{formatCurrencyExact(run.cost_usd, locale)}</td>
                    <td className="py-1.5 text-end">{run.dry_run ? t('common.dryRun') : run.status}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      </Card>
    </div>
  )
}

function ReportTable({ report }: { report: Report }) {
  const { t, locale } = useLocale()
  return (
    <>
      <table className="w-full text-[11px]">
        <thead>
          <tr className="text-start text-[var(--text-faint)]">
            <th className="pb-1 font-medium">{t('common.source')}</th>
            <th className="pb-1 text-end font-medium">{t('import.reportScanned')}</th>
            <th className="pb-1 text-end font-medium">{t('common.new')}</th>
            <th className="pb-1 text-end font-medium">{t('common.duplicates')}</th>
            <th className="pb-1 text-end font-medium">{t('common.tokens')}</th>
            <th className="pb-1 text-end font-medium">{t('common.cost')}</th>
            <th className="pb-1 text-end font-medium">{t('import.reportUnpriced')}</th>
          </tr>
        </thead>
        <tbody className="divide-y divide-[var(--border)]">
          {report.clis.map((c) => (
            <tr key={c.cli} className="text-[var(--text-muted)]">
              <td className="py-1.5 text-[var(--text-strong)]">
                {c.cli}
                {c.error && <span className="ms-2 text-[var(--warn-ink)]">{c.error}</span>}
              </td>
              <td className="num py-1.5 text-end">{formatNumber(c.scanned, locale)}</td>
              <td className="num py-1.5 text-end">{formatNumber(c.inserted, locale)}</td>
              <td className="num py-1.5 text-end">{formatNumber(c.duplicates, locale)}</td>
              <td className="num py-1.5 text-end">{formatNumber(c.tokens, locale)}</td>
              <td className="num py-1.5 text-end">{formatCurrencyExact(c.cost_usd, locale)}</td>
              <td className="num py-1.5 text-end">{c.unpriced > 0 ? formatNumber(c.unpriced, locale) : '—'}</td>
            </tr>
          ))}
        </tbody>
      </table>
      <div className="mt-3 flex flex-wrap items-baseline gap-x-4 gap-y-1 text-[11px] text-[var(--text-muted)]">
        <span>
          <span className="num font-medium text-[var(--text-strong)]">{formatNumber(report.inserted, locale)}</span> {t('common.new')}
        </span>
        <span>
          <span className="num font-medium text-[var(--text-strong)]">{formatNumber(report.duplicates, locale)}</span> {t('import.alreadyPresent')}
        </span>
        <span>
          <span className="num font-medium text-[var(--text-strong)]">{formatNumber(report.tokens, locale)}</span> {t('common.tokens')}
        </span>
        <span>
          <span className="num font-medium text-[var(--text-strong)]">{formatCurrencyExact(report.cost_usd, locale)}</span> {t('common.cost')}
        </span>
        {report.unpriced > 0 && (
          <span className="text-[var(--warn-ink)]">
            {formatNumber(report.unpriced)} {t('import.noPrice', { count: formatNumber(report.unpriced, locale) })}
          </span>
        )}
      </div>
    </>
  )
}
