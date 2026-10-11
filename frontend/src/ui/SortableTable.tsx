import { useMemo, useState } from 'react'
import { ArrowDown, ArrowUp, ChevronsUpDown } from 'lucide-react'

export interface Column<T> {
  key: string
  header: string
  render: (row: T) => React.ReactNode
  sortValue?: (row: T) => number | string
  align?: 'left' | 'right'
  width?: string
}

export function SortableTable<T>({ rows, columns, pageSize = 25, rowKey, onRowClick, activeRowKey }: {
  rows: T[]
  columns: Column<T>[]
  pageSize?: number
  rowKey: (row: T) => string | number
  onRowClick?: (row: T) => void
  activeRowKey?: string | number
}) {
  const [sortKey, setSortKey] = useState<string | null>(null)
  const [sortDir, setSortDir] = useState<'asc' | 'desc'>('desc')
  const [page, setPage] = useState(0)

  const sorted = useMemo(() => {
    if (!sortKey) return rows
    const col = columns.find((c) => c.key === sortKey)
    if (!col?.sortValue) return rows
    const copy = [...rows]
    copy.sort((a, b) => {
      const av = col.sortValue!(a)
      const bv = col.sortValue!(b)
      if (typeof av === 'number' && typeof bv === 'number') return sortDir === 'asc' ? av - bv : bv - av
      return sortDir === 'asc' ? String(av).localeCompare(String(bv)) : String(bv).localeCompare(String(av)) * -1
    })
    return copy
  }, [rows, sortKey, sortDir, columns])

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
      <div className="overflow-x-auto">
        <table className="w-full text-left text-[12px]">
          <thead>
            <tr className="border-b border-[var(--border)] text-[var(--text-soft)]">
              {columns.map((col) => (
                <th
                  key={col.key}
                  className={`py-2.5 px-3 font-medium ${col.align === 'right' ? 'text-right' : ''} ${col.sortValue ? 'cursor-pointer select-none hover:text-[var(--text-strong)]' : ''} ${col.width ?? ''}`}
                  onClick={col.sortValue ? () => toggleSort(col.key) : undefined}
                >
                  <span className={`inline-flex items-center gap-1 ${col.align === 'right' ? 'flex-row-reverse' : ''}`}>
                    {col.header}
                    {col.sortValue && (
                      sortKey === col.key
                        ? (sortDir === 'asc' ? <ArrowUp className="h-3 w-3" /> : <ArrowDown className="h-3 w-3" />)
                        : <ChevronsUpDown className="h-3 w-3 opacity-30" />
                    )}
                  </span>
                </th>
              ))}
            </tr>
          </thead>
          <tbody className="divide-y divide-[var(--border-soft)] text-[var(--text)]">
            {pageRows.length === 0 ? (
              <tr><td colSpan={columns.length} className="py-6 text-center text-[var(--text-faint)]">No data</td></tr>
            ) : pageRows.map((row) => (
              <tr
                key={rowKey(row)}
                className={`hover:bg-[var(--bg-fill-tertiary)] ${onRowClick ? 'cursor-pointer' : ''} ${activeRowKey === rowKey(row) ? 'bg-[var(--bg-fill-tertiary)]' : ''}`}
                onClick={onRowClick ? () => onRowClick(row) : undefined}
              >
                {columns.map((col) => (
                  <td key={col.key} className={`py-2.5 px-3 ${col.align === 'right' ? 'text-right' : ''}`}>
                    {col.render(row)}
                  </td>
                ))}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      {sorted.length > pageSize && (
        <div className="flex items-center justify-end gap-2 border-t border-[var(--border-soft)] px-3 py-2 text-[11px] text-[var(--text-soft)]">
          <button
            className="rounded px-2 py-1 hover:bg-[var(--bg-fill-secondary)] disabled:opacity-30"
            disabled={safePage === 0}
            onClick={() => setPage(safePage - 1)}
          >
            Prev
          </button>
          <span className="num">{safePage + 1} / {pageCount}</span>
          <button
            className="rounded px-2 py-1 hover:bg-[var(--bg-fill-secondary)] disabled:opacity-30"
            disabled={safePage >= pageCount - 1}
            onClick={() => setPage(safePage + 1)}
          >
            Next
          </button>
        </div>
      )}
    </div>
  )
}