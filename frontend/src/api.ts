import { useQuery } from '@tanstack/react-query'
import type { Stats } from './types'

export function useStats(since?: string) {
  return useQuery<Stats>({
    queryKey: ['stats', since ?? 'all'],
    queryFn: async () => {
      const url = since ? `/api/stats?since=${encodeURIComponent(since)}` : '/api/stats'
      const res = await fetch(url)
      if (!res.ok) throw new Error(`stats: ${res.status}`)
      return res.json()
    },
  })
}