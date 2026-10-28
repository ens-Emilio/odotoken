import { useMemo } from 'react'
import type { DailyRow } from '../types'
import { Card, CardHeader } from './Card'
import { formatCurrency, formatNumberCompact } from '../format'

interface Props {
  daily: DailyRow[]
  selectedDay: string | null
  onSelectDay: (day: string) => void
}

const WEEKDAYS = ['S', 'T', 'Q', 'Q', 'S', 'S', 'D']

function getIntensity(cost: number, maxCost: number): number {
  if (maxCost <= 0 || cost <= 0) return 0
  const ratio = cost / maxCost
  if (ratio < 0.25) return 1
  if (ratio < 0.5) return 2
  if (ratio < 0.75) return 3
  return 4
}

export default function ConsumptionCalendar({ daily, selectedDay, onSelectDay }: Props) {
  const { grid, maxCost } = useMemo(() => {
    if (!daily || daily.length === 0) return { grid: [], maxCost: 0 }

    const sorted = [...daily].sort((a, b) => a.day.localeCompare(b.day))
    const firstDate = new Date(sorted[0].day + 'T00:00:00')
    const lastDate = new Date(sorted[sorted.length - 1].day + 'T00:00:00')

    const startDate = new Date(firstDate)
    startDate.setDate(startDate.getDate() - startDate.getDay())

    const endDate = new Date(lastDate)
    endDate.setDate(endDate.getDate() + (6 - endDate.getDay()))

    const dayMap = new Map<string, DailyRow>()
    for (const row of sorted) {
      dayMap.set(row.day, row)
    }

    const cells: Array<{ day: string; row?: DailyRow; inRange: boolean }> = []
    const cursor = new Date(startDate)
    let max = 0
    while (cursor <= endDate) {
      const key = cursor.toISOString().slice(0, 10)
      const row = dayMap.get(key)
      if (row && row.cost_usd > max) max = row.cost_usd
      cells.push({
        day: key,
        row,
        inRange: cursor >= firstDate && cursor <= lastDate,
      })
      cursor.setDate(cursor.getDate() + 1)
    }

    return { grid: cells, maxCost: max }
  }, [daily])

  if (!daily || daily.length === 0) return null

  return (
    <Card className="py-2">
      <CardHeader title="Consumo Diário" subtitle="clique em um dia para ver detalhes" />
      <div className="px-3 pb-2">
        {/* Weekday headers */}
        <div className="mb-1.5 grid grid-cols-7 gap-1 text-center text-[9px] font-semibold uppercase tracking-wider text-[var(--text-faint)]">
          {WEEKDAYS.map((d, i) => <div key={i} className="py-0.5">{d}</div>)}
        </div>

        {/* Calendar grid */}
        <div className="grid grid-cols-7 gap-1">
          {grid.map((cell) => {
            const intensity = cell.row ? getIntensity(cell.row.cost_usd, maxCost) : 0
            const isSelected = cell.day === selectedDay
            const isEmpty = !cell.inRange || !cell.row
            const dayNum = cell.inRange ? new Date(cell.day + 'T00:00:00').getDate() : ''

            let bgClass = 'bg-transparent'
            if (!isEmpty) {
              if (intensity === 0) bgClass = 'bg-[var(--bg-fill)]'
              else if (intensity === 1) bgClass = 'bg-emerald-950/40'
              else if (intensity === 2) bgClass = 'bg-emerald-900/50'
              else if (intensity === 3) bgClass = 'bg-emerald-700/60'
              else bgClass = 'bg-emerald-600/80'
            }

            return (
              <button
                key={cell.day}
                disabled={isEmpty}
                onClick={() => onSelectDay(cell.day)}
                title={cell.row ? `${cell.day}: ${formatCurrency(cell.row.cost_usd)} · ${formatNumberCompact(cell.row.total_tokens)} tokens` : cell.day}
                className={`
                  flex h-9 items-center justify-center rounded text-[11px] font-medium tabular-nums transition-colors
                  ${bgClass}
                  ${isSelected ? 'ring-2 ring-[var(--brand)] ring-offset-1 ring-offset-[var(--bg-card)]' : ''}
                  ${!isEmpty ? 'cursor-pointer hover:brightness-110' : 'cursor-default'}
                  ${cell.row?.cost_usd ? 'text-white/90' : isEmpty ? 'text-transparent' : 'text-[var(--text-soft)]'}
                `}
              >
                {dayNum}
              </button>
            )
          })}
        </div>

        {/* Legend */}
        <div className="mt-2 flex items-center justify-end gap-1.5 text-[9px] text-[var(--text-faint)]">
          <span>Menos</span>
          <div className="h-2.5 w-2.5 rounded-sm bg-[var(--bg-fill)]" />
          <div className="h-2.5 w-2.5 rounded-sm bg-emerald-950/40" />
          <div className="h-2.5 w-2.5 rounded-sm bg-emerald-900/50" />
          <div className="h-2.5 w-2.5 rounded-sm bg-emerald-700/60" />
          <div className="h-2.5 w-2.5 rounded-sm bg-emerald-600/80" />
          <span>Mais</span>
        </div>
      </div>
    </Card>
  )
}