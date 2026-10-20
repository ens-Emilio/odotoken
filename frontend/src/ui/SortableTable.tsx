import { useEffect, useMemo, useState } from 'react'
import { ArrowDown, ArrowUp, ChevronsUpDown } from 'lucide-react'

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
  const [filter, setFilter] = useState('')
  const filtered = useMemo(() => {
    if (!filter || !searchKeys?.length) return rows
    const q = filter.toLowerCase()
    return rows.filter((r) => searchKeys.some((k) => String(r[k] ?? '').toLowerCase().includes(q)))
  }, [rows, filter, searchKeys])
  const [sortKey, setSortKey] = useState<string | null>(null)
  const [sortDir, setSortDir] = useState<'asc' | 'desc'>('desc')
  const [page, setPage] = useState(0)
  useEffect(() => {
    const maxPage = Math.max(0, Math.ceil(filtered.length / pageSize) - 1)
    if (page > maxPage) setPage(0)
  }, [filtered.length, pageSize, page])
  useEffect(() => { setPage(0) }, [sortKey, sortDir, filter])

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
  const safePage = Math.min(page, pageCount - 1)
  const pageRows = sorted.slice(safePage * pageSize, (safePage + 1) * pageSize)

  const toggleSort = (key: string) => {
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
          <input value={filter} onChange={(e) => setFilter(e.target.value)} placeholder="Filter…" aria-label="Filter table" className="w-full max-w-xs rounded-md border border-[var(--border)] bg-[var(--bg)] px-2.5 py-1.5 text-[12px] placeholder:text-[var(--text-faint)] focus:outline-none focus:ring-2 focus:ring-[var(--brand)]" />
          {filter && <button onClick={() => setFilter('')} className="text-[11px] text-[var(--text-soft)] hover:text-[var(--text-strong)]">Clear</button>}
          <span className="ml-auto text-[11px] text-[var(--text-faint)]">{filtered.length !== rows.length ? `${filtered.length}/${rows.length} rows` : `${rows.length} rows`}</span>
        </div>
      ) : null}
      <div className="table-scroll overflow-x-auto">
        <table className="w-full text-left text-[12px]">
          <thead>
            <tr className="border-b border-[var(--border)]">
              {columns.map((col) => (
                <th
                  key={col.key}
                  className={`py-2.5 px-3 text-[11px] font-medium uppercase tracking-wider ${col.align === 'right' ? 'text-right' : 'text-left'} ${col.sortValue ? 'cursor-pointer select-none' : ''} ${sortKey === col.key ? 'text-[var(--text-strong)]' : 'text-[var(--text-soft)] hover:text-[var(--text-strong)]'} ${col.width ?? ''}`}
                  onClick={col.sortValue ? () => toggleSort(col.key) : undefined}
                >
                  <span className={`inline-flex items-center gap-1 ${col.align === 'right' ? 'flex-row-reverse' : ''}`}>
                    {col.header}
                    {col.sortValue && (
                      sortKey === col.key
                        ? (sortDir === 'asc' ? <ArrowUp className="h-3 w-3" /> : <ArrowDown className="h-3 w-3" />)
                        : <ChevronsUpDown className="h-3 w-3 opacity-40" />
                    )}
                  </span>
                </th>
              ))}
            </tr>
          </thead>
          <tbody className="divide-y divide-[var(--border-soft)] text-[var(--text)]">
            {pageRows.length === 0 ? (
              <tr><td colSpan={columns.length} className="py-8 text-center text-[12px] text-[var(--text-soft)]">No data in this period — try a wider time range</td></tr>
            ) : pageRows.map((row) => (
              <tr
                key={rowKey(row)}
                className={`hover:bg-[var(--bg-fill-tertiary)] ${onRowClick ? 'cursor-pointer' : ''} ${activeRowKey === rowKey(row) ? 'bg-[var(--bg-fill-tertiary)]' : ''}`}
                onClick={onRowClick ? () => onRowClick(row) : undefined}
              >
                {columns.map((col) => (
                  <td key={col.key} className={`py-3 px-3 ${col.align === 'right' ? 'text-right tabular-nums' : ''}`}>
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
          <span className="num">{sorted.length} rows · page {safePage + 1} of {pageCount}</span>
          <div className="flex items-center gap-1">
            <button className="flex h-7 w-7 items-center justify-center rounded-md border border-[var(--border)] hover:bg-[var(--bg-fill-secondary)] disabled:opacity-30" disabled={safePage === 0} onClick={() => setPage(safePage - 1)} aria-label="Previous page">‹</button>
            <span className="num px-1">{safePage + 1} / {pageCount}</span>
            <button className="flex h-7 w-7 items-center justify-center rounded-md border border-[var(--border)] hover:bg-[var(--bg-fill-secondary)] disabled:opacity-30" disabled={safePage >= pageCount - 1} onClick={() => setPage(safePage + 1)} aria-label="Next page">›</button>
          </div>
        </div>
      )}
    </div>
  )
}