import { useState } from 'react'
import { usePrices, savePrice, deletePrice } from './api'
import { formatCurrency } from './format'
import { Card, CardHeader, Badge, Empty } from './ui/Card'
import { Button } from './ui/Button'
import { Dialog } from './ui/Dialog'
import { SortableTable, type Column } from './ui/SortableTable'
import { toast } from './ui/toast-store'
import type { PriceRow } from './types'
import { useLocale } from './i18n/useLocale'
import { useQueryClient } from '@tanstack/react-query'

const RATE_FIELDS = ['input', 'output', 'cache_read', 'cache_write'] as const

const RATE_LABELS: Record<typeof RATE_FIELDS[number], string> = {
  input: 'prices.columnInput',
  output: 'prices.columnOutput',
  cache_read: 'prices.columnCacheRead',
  cache_write: 'prices.columnCacheWrite',
}

/**
 * Price editing writes to prices.json through the API, which validates before
 * swapping the catalog the proxy prices with. Deleting only removes an override
 * — a built-in rate cannot be deleted, and the API says so.
 */
export default function PricesView() {
  const { t, locale } = useLocale()
  const queryClient = useQueryClient()
  const [query, setQuery] = useState('')
  const { data, isLoading, isError, refetch } = usePrices(query, 200)
  const [editing, setEditing] = useState<PriceRow | null>(null)
  const [rates, setRates] = useState<Record<string, string>>({})
  const [saving, setSaving] = useState(false)

  const openEditor = (row: PriceRow) => {
    setEditing(row)
    setRates({
      input: String(row.input),
      output: String(row.output),
      cache_read: String(row.cache_read),
      cache_write: String(row.cache_write),
    })
  }

  const save = async () => {
    if (!editing) return
    setSaving(true)
    try {
      await savePrice(editing.model, {
        input: Number(rates.input) || 0,
        output: Number(rates.output) || 0,
        cache_read: Number(rates.cache_read) || 0,
        cache_write: Number(rates.cache_write) || 0,
      })
      await queryClient.invalidateQueries({ queryKey: ['prices'] })
      toast('ok', t('prices.saved'))
      setEditing(null)
    } catch (err) {
      const code = err instanceof Error ? err.message : ''
      toast('error', code.startsWith('error.') ? t(code) : t('prices.saveFailed'))
    } finally {
      setSaving(false)
    }
  }

  const remove = async (row: PriceRow) => {
    try {
      await deletePrice(row.model)
      await queryClient.invalidateQueries({ queryKey: ['prices'] })
      toast('ok', t('prices.saved'))
    } catch {
      toast('error', t('prices.saveFailed'))
    }
  }

  const columns: Column<PriceRow>[] = [
    { key: 'model', header: t('common.model'), render: (r) => <span className="font-medium text-[var(--text-strong)]">{r.model}</span>, sortValue: (r) => r.model },
    { key: 'source', header: t('prices.source'), render: (r) => <Badge tone={r.explicit ? 'brand' : 'neutral'}>{r.explicit ? t('prices.explicit') : t('prices.fallback')}</Badge>, sortValue: (r) => String(r.explicit) },
    { key: 'input', header: t('prices.columnInput'), align: 'right', render: (r) => <span className="num tabular-nums">{formatCurrency(r.input, locale)}</span>, sortValue: (r) => r.input },
    { key: 'output', header: t('prices.columnOutput'), align: 'right', render: (r) => <span className="num tabular-nums">{formatCurrency(r.output, locale)}</span>, sortValue: (r) => r.output },
    { key: 'cache_read', header: t('overview.cacheRead'), align: 'right', render: (r) => <span className="num tabular-nums text-[var(--text-soft)]">{formatCurrency(r.cache_read, locale)}</span>, sortValue: (r) => r.cache_read },
    { key: 'cache_write', header: t('events.cacheWrite'), align: 'right', render: (r) => <span className="num tabular-nums text-[var(--text-soft)]">{formatCurrency(r.cache_write, locale)}</span>, sortValue: (r) => r.cache_write },
    {
      key: 'actions',
      header: '',
      align: 'right',
      render: (r) => (
        <span className="flex items-center justify-end gap-2">
          <button type="button" onClick={() => openEditor(r)} className="rounded border border-[var(--border)] px-2 py-0.5 text-[11px] text-[var(--text-strong)] hover:bg-[var(--bg-fill-secondary)]">{t('prices.edit')}</button>
          {r.explicit && (
            <button type="button" onClick={() => { void remove(r) }} title={t('prices.overrideHint')} className="rounded border border-[var(--border)] px-2 py-0.5 text-[11px] text-[var(--danger-ink)] hover:bg-[var(--danger-soft)]">{t('prices.delete')}</button>
          )}
        </span>
      ),
    },
  ]

  return (
    <div className="space-y-5 p-5">
      <Card>
        <CardHeader
          title={t('nav.prices')}
          subtitle={t('prices.subtitle')}
          right={<span className="num text-[11px] text-[var(--text-faint)]">{data?.path}</span>}
        />
        <div className="px-5 pb-4">
          <input
            type="search"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder={t('prices.search')}
            className="w-full max-w-sm rounded-md border border-[var(--border)] bg-[var(--bg)] px-2.5 py-1.5 text-[12px] focus:outline-none focus:ring-2 focus:ring-[var(--brand)]"
          />
        </div>
        {isLoading && <div className="mx-5 mb-5 h-40 animate-pulse rounded-[var(--radius)] bg-[var(--bg-fill-tertiary)]" />}
        {isError && <Empty label={t('prices.loadFailed')} hint={t('error.prices.unreadable')} onRetry={() => { void refetch() }} />}
        {data && data.prices.length === 0 && <Empty label={t('prices.empty')} hint={t('prices.search')} />}
        {data && data.prices.length > 0 && (
          <SortableTable rows={data.prices} rowKey={(r) => r.model} columns={columns} pageSize={25} searchable searchKeys={['model']} />
        )}
      </Card>

      <Dialog open={editing !== null} onClose={() => setEditing(null)} title={t('prices.editTitle', { model: editing?.model ?? '' })}>
        {editing && (
          <div className="space-y-3">
            <p className="text-[11px] text-[var(--text-soft)]">{t('prices.perMillion')}</p>
            <div className="grid grid-cols-2 gap-3">
              {RATE_FIELDS.map((field) => (
                <label key={field} className="flex flex-col gap-1">
                  <span className="text-[11px] text-[var(--text-soft)]">{t(RATE_LABELS[field])}</span>
                  <input
                    type="number"
                    min="0"
                    step="any"
                    value={rates[field]}
                    onChange={(e) => setRates({ ...rates, [field]: e.target.value })}
                    className="rounded-md border border-[var(--border)] bg-[var(--bg)] px-2 py-1.5 text-[12px] focus:outline-none focus:ring-2 focus:ring-[var(--brand)]"
                  />
                </label>
              ))}
            </div>
            <div className="flex justify-end gap-2">
              <Button onClick={() => setEditing(null)}>{t('common.cancel')}</Button>
              <Button variant="primary" onClick={save} disabled={saving}>{saving ? t('common.saving') : t('prices.save')}</Button>
            </div>
          </div>
        )}
      </Dialog>
    </div>
  )
}
