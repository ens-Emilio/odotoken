import { useQuery } from '@tanstack/react-query'
 import type { Stats, TpsData, Forecast, LiveSession, EventRow } from './types'

export interface GuardEvent {
  id: number
  timestamp: string
  status: string
  message: string
  action: string
  daily_cost_usd: number
  monthly_cost_usd: number
}

export interface GuardStatus {
  status: string
  allowed: boolean
  reason: string
  history?: GuardEvent[]
}

// The app shell needs only guard state; loading the full stats payload here
// doubles the database aggregation on every page.
export function useGuardStatus(enabled = true) {
  return useQuery<GuardStatus>({
    queryKey: ['guard-status'],
    queryFn: async () => {
      const res = await fetch('/api/guard/status')
      if (!res.ok) throw new Error(`guard status: ${res.status}`)
      return res.json()
    },
    enabled,
    staleTime: 10_000,
    refetchInterval: 10_000,
  })
}

export interface FilterOption {
  source?: string
  project_id?: string
  tag?: string
  requests: number
}

export interface FilterOptions {
  by_source: FilterOption[]
  by_project: FilterOption[]
  by_tag: FilterOption[]
}

export function useFilterOptions(project?: string, source?: string) {
  const params = new URLSearchParams()
  if (project && project !== 'all') params.set('project', project)
  if (source && source !== 'all') params.set('source', source)
  const qs = params.toString()
  return useQuery<FilterOptions>({
    queryKey: ['filter-options', project ?? 'all', source ?? 'all'],
    queryFn: async () => {
      const res = await fetch(`/api/filters${qs ? `?${qs}` : ''}`)
      if (!res.ok) throw new Error(`filters: ${res.status}`)
      return res.json()
    },
    staleTime: 30_000,
    refetchInterval: 30_000,
  })
}
export function useStats(since?: string, project?: string, source?: string, tag?: string, tz?: string) {
  const params = new URLSearchParams()
  if (since) params.set('since', since)
  if (project && project !== 'all') params.set('project', project)
  if (source && source !== 'all') params.set('source', source)
  if (tag && tag !== 'all') params.set('tag', tag)
  if (tz) params.set('tz', tz)
  const qs = params.toString()
  return useQuery<Stats>({
    queryKey: ['stats', since ?? 'all', project ?? 'all', source ?? 'all', tag ?? 'all', tz ?? 'UTC'],
    queryFn: async () => {
      const url = qs ? `/api/stats?${qs}` : '/api/stats'
      try {
        const controller = new AbortController()
        const timeoutId = setTimeout(() => controller.abort(), 30_000)
        const res = await fetch(url, { signal: controller.signal })
        clearTimeout(timeoutId)
        if (!res.ok) throw new Error(`stats: ${res.status}`)
        const data = await res.json()
        return data
      } catch (err) {
        console.error('useStats fetch error:', err)
        throw err
      }
    },
    staleTime: since ? 30_000 : 15_000,
    refetchInterval: since ? 30_000 : 15_000,
  })
}
export function useTps(since?: string, interval: string = '1h', project?: string, source?: string) {
  const params = new URLSearchParams()
  if (since) params.set('since', since)
  if (interval) params.set('interval', interval)
  if (project && project !== 'all') params.set('project', project)
  if (source && source !== 'all') params.set('source', source)
  const qs = params.toString()
  return useQuery<TpsData>({
    queryKey: ['tps', since ?? 'all', interval, project ?? 'all', source ?? 'all'],
    queryFn: async () => {
      const res = await fetch(`/api/tps?${qs}`)
      if (!res.ok) throw new Error(`tps: ${res.status}`)
      return res.json()
    },
    staleTime: 30_000,
    refetchInterval: 30_000,
  })
}
export function useForecast(tag?: string, tz?: string) {
  const params = new URLSearchParams()
  if (tag) params.set('tag', tag)
  if (tz) params.set('tz', tz)
  const qs = params.toString()
  return useQuery<Forecast>({
    queryKey: ['forecast', tag ?? 'all', tz ?? 'UTC'],
    queryFn: async () => {
      const res = await fetch(`/api/forecast${qs ? '?' + qs : ''}`)
      if (!res.ok) throw new Error(`forecast: ${res.status}`)
      return res.json()
    },
    staleTime: 30_000,
  })
}

export function useLiveSessions() {
  return useQuery<LiveSession[]>({
    queryKey: ['live-runs'],
    queryFn: async () => {
      const res = await fetch('/api/sessions/live')
      if (!res.ok) throw new Error(`live sessions: ${res.status}`)
      return res.json()
    },
    staleTime: 1_000,
    refetchInterval: 2_000,
 })
 }

 export function useEvents(since?: string, project?: string, source?: string, tag?: string, limit = 50) {
 const params = new URLSearchParams()
 if (since) params.set('since', since)
 if (project && project !== 'all') params.set('project', project)
 if (source && source !== 'all') params.set('source', source)
 if (tag && tag !== 'all') params.set('tag', tag)
 params.set('limit', String(limit))
 const qs = params.toString()
 return useQuery<EventRow[]>({
 queryKey: ['events', since ?? 'all', project ?? 'all', source ?? 'all', tag ?? 'all', limit],
 queryFn: async () => {
      const res = await fetch(`/api/events?${qs}`)
 if (!res.ok) throw new Error(`events: ${res.status}`)
 return res.json()
 },
 staleTime: 5_000,
 refetchInterval: 10_000,
 })
 }
