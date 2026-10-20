import { useQuery } from '@tanstack/react-query'
import type { Stats } from './types'

export function useStats(since?: string, project?: string, source?: string) {
  const params = new URLSearchParams()
  if (since) params.set('since', since)
  if (project && project !== 'all') params.set('project', project)
  if (source && source !== 'all') params.set('source', source)
  const qs = params.toString()
  return useQuery<Stats>({
    queryKey: ['stats', since ?? 'all', project ?? 'all', source ?? 'all'],
    queryFn: async () => {
      const url = qs ? `/api/stats?${qs}` : '/api/stats'
      const res = await fetch(url)
      if (!res.ok) throw new Error(`stats: ${res.status}`)
      return res.json()
    },
    staleTime: 10_000,
    refetchInterval: 10_000,
  })
}