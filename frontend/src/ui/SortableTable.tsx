import { useMemo, useState } from 'react'
import { ArrowDown, ArrowUp, ChevronsUpDown } from 'lucide-react'
import { useLocale } from '../i18n/useLocale'

export interface Column<T> {
  key: string
  header: string
  render: (row: T) => React.ReactNode
  sortValue?: (row: T) => number | string
  align?: 'left' | 'right'
  width?: string
}

export function SortableTable<T>({ rows, columns, pageSize = 25, rowKey, onRowClick, activeRowKey, searchable, searchKeys }: {
  rows: T[]
  columns: Column<T>[]
  pageSize?: number
  rowKey: (row: T) => string | number
  onRowClick?: (row: T) => void
  activeRowKey?: string | number
  searchable?: boolean
  searchKeys?: (keyof T)[]
}) {
  const { t, locale } = useLocale()
  const [filter, setFilter] = useState('')
  const filtered = useMemo(() => {
    if (!filter || !searchKeys?.length) return rows
    const q = filter.toLowerCase()
    return rows.filter((r) => searchKeys.some((k) => String(r[k] ?? '').toLowerCase().includes(q)))
  }, [rows, filter, searchKeys])
  const [sortKey, setSortKey] = useState<string | null>(null)
  const [sortDir, setSortDir] = useState<'asc' | 'desc'>('desc')
  const [page, setPage] = useState(0)

  const sorted = useMemo(() => {
    if (!sortKey) return filtered
    const col = columns.find((c) => c.key === sortKey)
    if (!col?.sortValue) return filtered
    const copy = [...filtered]
    copy.sort((a, b) => {
      const av = col.sortValue!(a)
      const bv = col.sortValue!(b)
      if (typeof av === 'number' && typeof bv === 'number') return sortDir === 'asc' ? av - bv : bv - av
      return sortDir === 'asc' ? String(av).localeCompare(String(bv)) : String(bv).localeCompare(String(av))
    })
    return copy
  }, [filtered, sortKey, sortDir, columns])

  const pageCount = Math.max(1, Math.ceil(sorted.length / pageSize))
  const safePage = page >= pageCount ? 0 : page
  const pageRows = sorted.slice(safePage * pageSize, (safePage + 1) * pageSize)

  const onFilterChange = (value: string) => { setFilter(value); setPage(0) }

  const plural = (count: number, key: 'table.rows' | 'table.filtered'): string => {
    const rule = new Intl.PluralRules(locale).select(count)
    return t(`${key}.${rule === 'one' ? 'one' : 'other'}`, { count })
  }

  const toggleSort = (key: string) => {
    setPage(0)
    if (sortKey === key) {
      setSortDir((d) => (d === 'asc' ? 'desc' : 'asc'))
    } else {
      setSortKey(key)
      setSortDir('desc')
    }
  }

  return (
    <div>
      {searchable && searchKeys?.length ? (
        <div className="flex items-center gap-2 px-3 py-2 border-b border-[var(--border-soft)]">
          <input value={filter} onChange={(e) => onFilterChange(e.target.value)} placeholder={t('common.filter')} aria-label={t('common.filterTable')} className="w-full max-w-xs rounded-md border border-[var(--border)] bg-[var(--bg)] px-2.5 py-1.5 text-[12px] placeholder:text-[var(--text-faint)] focus:outline-none focus:ring-2 focus:ring-[var(--brand)]" />
          {filter && <button onClick={() => onFilterChange('')} className="text-[11px] text-[var(--text-soft)] hover:text-[var(--text-strong)]">{t('common.clear')}</button>}
          <span className="ms-auto text-[11px] text-[var(--text-faint)]">{filtered.length !== rows.length ? plural(filtered.length, 'table.filtered') : plural(rows.length, 'table.rows')}</span>
        </div>
      ) : null}
      <div className="table-scroll overflow-x-auto">
        <table className="w-full text-start text-[12px]">
          <thead>
            <tr className="border-b border-[var(--border)]">
              {columns.map((col) => (
                <th
                  key={col.key}
                  scope="col"
                  aria-sort={sortKey === col.key ? (sortDir === 'asc' ? 'ascending' : 'descending') : 'none'}
                  tabIndex={col.sortValue ? 0 : undefined}
                  role={col.sortValue ? 'button' : undefined}
                  onKeyDown={col.sortValue ? (event) => {
                    if (event.key === 'Enter' || event.key === ' ') {
                      event.preventDefault()
                      toggleSort(col.key)
                    }
                  } : undefined}
                  className={`py-2.5 px-3 text-[11px] font-medium uppercase tracking-wider ${col.align === 'right' ? 'text-end' : 'text-start'} ${col.sortValue ? 'cursor-pointer select-none focus:outline-none focus:ring-2 focus:ring-[var(--brand)]' : ''} ${sortKey === col.key ? 'text-[var(--text-strong)]' : 'text-[var(--text-soft)] hover:text-[var(--text-strong)]'} ${col.width ?? ''}`}
                  onClick={col.sortValue ? () => toggleSort(col.key) : undefined}
                >
                  <span className={`inline-flex items-center gap-1 ${col.align === 'right' ? 'flex-row-reverse' : ''}`}>
                    {col.header}
                    {col.sortValue && (
                      sortKey === col.key
                        ? (sortDir === 'asc' ? <ArrowUp aria-hidden className="h-3 w-3" /> : <ArrowDown aria-hidden className="h-3 w-3" />)
                        : <ChevronsUpDown aria-hidden className="h-3 w-3 opacity-40" />
                    )}
                  </span>
                </th>
              ))}
            </tr>
          </thead>
          <tbody className="divide-y divide-[var(--border-soft)] text-[var(--text)]">
            {pageRows.length === 0 ? (
              <tr><td colSpan={columns.length} className="py-8 text-center text-[12px] text-[var(--text-soft)]">{t('common.noRecords')}</td></tr>
            ) : pageRows.map((row) => (
              <tr
                key={rowKey(row)}
                tabIndex={onRowClick ? 0 : undefined}
                onKeyDown={onRowClick ? (event) => {
                  if (event.key === 'Enter' || event.key === ' ') {
                    event.preventDefault()
                    onRowClick(row)
                  }
                } : undefined}
                className={`hover:bg-[var(--bg-fill-tertiary)] ${onRowClick ? 'cursor-pointer focus:outline-none focus:ring-2 focus:ring-inset focus:ring-[var(--brand)]' : ''} ${activeRowKey === rowKey(row) ? 'bg-[var(--bg-fill-tertiary)]' : ''}`}
                onClick={onRowClick ? () => onRowClick(row) : undefined}
              >
                {columns.map((col) => (
                  <td key={col.key} className={`py-3 px-3 ${col.align === 'right' ? 'text-end tabular-nums' : ''}`}>
                    {col.render(row)}
                  </td>
                ))}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      {sorted.length > pageSize && (
        <div className="flex items-center justify-between border-t border-[var(--border-soft)] px-3 py-2 text-[11px] text-[var(--text-soft)]">
          <span className="num">{plural(sorted.length, 'table.rows')}</span>
          <div className="flex items-center gap-1">
            <button className="flex h-7 w-7 items-center justify-center rounded-md border border-[var(--border)] hover:bg-[var(--bg-fill-secondary)] disabled:opacity-30" disabled={safePage === 0} onClick={() => setPage(safePage - 1)} aria-label={t('common.previousPage')}><span className="rtl:rotate-180">‹</span></button>
            <span className="num px-1">{safePage + 1} / {pageCount}</span>
            <button className="flex h-7 w-7 items-center justify-center rounded-md border border-[var(--border)] hover:bg-[var(--bg-fill-secondary)] disabled:opacity-30" disabled={safePage >= pageCount - 1} onClick={() => setPage(safePage + 1)} aria-label={t('common.nextPage')}><span className="rtl:rotate-180">›</span></button>
          </div>
        </div>
      )}
    </div>
  )
}