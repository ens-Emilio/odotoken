import { useQuery } from '@tanstack/react-query'
 import type { Stats, TpsData, Forecast, LiveSession, EventRow } from './types'

export function useStats(since?: string, project?: string, source?: string, tag?: string) {
  const params = new URLSearchParams()
  if (since) params.set('since', since)
  if (project && project !== 'all') params.set('project', project)
  if (source && source !== 'all') params.set('source', source)
  if (tag && tag !== 'all') params.set('tag', tag)
  const qs = params.toString()
  return useQuery<Stats>({
    queryKey: ['stats', since ?? 'all', project ?? 'all', source ?? 'all', tag ?? 'all'],
    queryFn: async () => {
      const url = qs ? `/api/stats?${qs}` : '/api/stats'
      console.log('useStats: fetching', url)
      try {
        const controller = new AbortController()
        const timeoutId = setTimeout(() => controller.abort(), 5000)
        const res = await fetch(url, { signal: controller.signal })
        clearTimeout(timeoutId)
        if (!res.ok) throw new Error(`stats: ${res.status}`)
        const data = await res.json()
        console.log('useStats: success', data.summary?.total_requests)
        return data
      } catch (err) {
        console.error('useStats fetch error:', err)
        throw err
      }
    },
    staleTime: 10_000,
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
    staleTime: 5_000,
    refetchInterval: 5_000,
  })
}
export function useForecast(tag?: string) {
  const qs = tag ? `?tag=${encodeURIComponent(tag)}` : ''
  return useQuery<Forecast>({
    queryKey: ['forecast', tag ?? 'all'],
    queryFn: async () => {
      const res = await fetch(`/api/forecast${qs}`)
      if (!res.ok) throw new Error(`forecast: ${res.status}`)
      return res.json()
    },
    staleTime: 30_000,
  })
}

export function useLiveSessions() {
  return useQuery<LiveSession[]>({
    queryKey: ['live-sessions'],
    queryFn: async () => {
      const res = await fetch('/api/sessions/live')
      if (!res.ok) throw new Error(`live sessions: ${res.status}`)
      return res.json()
    },
    staleTime: 5_000,
    refetchInterval: 10_000,
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
