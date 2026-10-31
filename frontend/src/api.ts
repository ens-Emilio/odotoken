import { useQuery } from '@tanstack/react-query'
import type { Stats, TpsData, Forecast, LiveSession, EventRow, DailyDetail, SessionCost, CacheSavings, SimulationRow, PriceRow, AttributedAnomaly } from './types'

export interface GuardEvent {
  id: number
  timestamp: string
  status: string
  message: string
  message_code?: string
  message_params?: string
  action: string
  daily_cost_usd: number
  monthly_cost_usd: number
}

export interface GuardStatus {
  status: string
  allowed: boolean
  reason: string
  reason_code?: string
  reason_params?: Record<string, string>
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

 export function useDailyDetail(day: string | null, tz?: string, project?: string, source?: string, tag?: string) {
  const params = new URLSearchParams()
  if (day) params.set('day', day)
  if (tz) params.set('tz', tz)
  if (project && project !== 'all') params.set('project', project)
  if (source && source !== 'all') params.set('source', source)
  if (tag && tag !== 'all') params.set('tag', tag)
  const qs = params.toString()
  return useQuery<DailyDetail>({
    queryKey: ['daily-detail', day ?? 'none', tz ?? 'UTC', project ?? 'all', source ?? 'all', tag ?? 'all'],
    queryFn: async () => {
      const res = await fetch(`/api/stats/daily-detail?${qs}`)
      if (!res.ok) throw new Error(`daily detail: ${res.status}`)
      return res.json()
    },
    enabled: !!day,
    staleTime: 30_000,
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

// --- Analytics (features 12-17) ---------------------------------------------
// Each hook mirrors one /api route and keeps the filter vocabulary the shell
// already uses: 'all' is the absence of a filter, never a literal value.

function analyticsQuery(since?: string, project?: string, source?: string, tag?: string): string {
  const params = new URLSearchParams()
  if (since) params.set('since', since)
  if (project && project !== 'all') params.set('project', project)
  if (source && source !== 'all') params.set('source', source)
  if (tag && tag !== 'all') params.set('tag', tag)
  return params.toString()
}

export function useSessions(since?: string, project?: string, source?: string, tag?: string, limit = 100) {
  const params = new URLSearchParams(analyticsQuery(since, project, source, tag))
  params.set('limit', String(limit))
  return useQuery<SessionCost[]>({
    queryKey: ['sessions', since ?? 'all', project ?? 'all', source ?? 'all', tag ?? 'all', limit],
    queryFn: async () => {
      const res = await fetch(`/api/sessions?${params.toString()}`)
      if (!res.ok) throw new Error(`sessions: ${res.status}`)
      return res.json()
    },
    staleTime: 15_000,
    refetchInterval: 30_000,
  })
}

export function useSessionEvents(sessionKey: string | null) {
  return useQuery<{ session_id: string; events: EventRow[] }>({
    queryKey: ['session-events', sessionKey],
    queryFn: async () => {
      const res = await fetch(`/api/sessions/${encodeURIComponent(sessionKey ?? '')}`)
      if (!res.ok) throw new Error(`session: ${res.status}`)
      return res.json()
    },
    enabled: !!sessionKey,
    staleTime: 30_000,
  })
}

export function useCacheSavings(since?: string, project?: string, source?: string, tag?: string) {
  const qs = analyticsQuery(since, project, source, tag)
  return useQuery<CacheSavings>({
    queryKey: ['cache-savings', since ?? 'all', project ?? 'all', source ?? 'all', tag ?? 'all'],
    queryFn: async () => {
      const res = await fetch(`/api/cache-savings${qs ? `?${qs}` : ''}`)
      if (!res.ok) throw new Error(`cache savings: ${res.status}`)
      return res.json()
    },
    staleTime: 30_000,
  })
}

export interface SimulationRequest {
  requests: number
  prompt_tokens: number
  completion_tokens: number
  cache_read_tokens: number
  cache_write_tokens: number
  baseline?: string
  limit?: number
}

export interface SimulationResponse { baseline: string; rows: SimulationRow[] }

export function useSimulation(input: SimulationRequest | null) {
  return useQuery<SimulationResponse>({
    queryKey: ['simulate', input],
    queryFn: async () => {
      const res = await fetch('/api/simulate', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(input),
      })
      if (!res.ok) throw new Error(`simulate: ${res.status}`)
      return res.json()
    },
    enabled: input !== null,
    staleTime: 60_000,
  })
}

export function usePrices(query = '', limit = 200) {
  const params = new URLSearchParams()
  if (query) params.set('q', query)
  if (limit) params.set('limit', String(limit))
  return useQuery<{ path: string; prices: PriceRow[] }>({
    queryKey: ['prices', query, limit],
    queryFn: async () => {
      const res = await fetch(`/api/prices${params.toString() ? `?${params.toString()}` : ''}`)
      if (!res.ok) throw new Error(`prices: ${res.status}`)
      return res.json()
    },
    staleTime: 30_000,
  })
}

export async function savePrice(model: string, rates: Omit<PriceRow, 'model' | 'explicit'>): Promise<PriceRow> {
  const res = await fetch(`/api/prices/${encodeURIComponent(model)}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(rates),
  })
  if (!res.ok) {
    const body = await res.json().catch(() => null)
    throw new Error(body?.code ?? `save price: ${res.status}`)
  }
  return res.json()
}

export async function deletePrice(model: string): Promise<void> {
  const res = await fetch(`/api/prices/${encodeURIComponent(model)}`, { method: 'DELETE' })
  if (!res.ok && res.status !== 404) throw new Error(`delete price: ${res.status}`)
}

export function useAnomalies(project?: string, source?: string, tag?: string, tz?: string) {
  const params = new URLSearchParams(analyticsQuery(undefined, project, source, tag))
  if (tz) params.set('tz', tz)
  return useQuery<{ anomalies: AttributedAnomaly[]; generated_at: string }>({
    queryKey: ['anomalies', project ?? 'all', source ?? 'all', tag ?? 'all', tz ?? 'UTC'],
    queryFn: async () => {
      const res = await fetch(`/api/anomalies${params.toString() ? `?${params.toString()}` : ''}`)
      if (!res.ok) throw new Error(`anomalies: ${res.status}`)
      return res.json()
    },
    staleTime: 60_000,
  })
}

// --- Client keys (feature 20) ------------------------------------------------
// The listing only ever carries a masked identifier. The full secret exists
// once, in the PUT response, and is shown to the operator a single time.

export interface ClientKeyInput {
  key?: string
  client: string
  project: string
  daily_cost_limit_usd: number
  monthly_cost_limit_usd: number
  daily_tokens_limit: number
  monthly_tokens_limit: number
}

export interface ClientKeyRecord extends ClientKeyInput {
  key: string
  plaintext_key?: string
}

function normalizeClientKeys(payload: unknown): ClientKeyRecord[] {
  if (Array.isArray(payload)) return payload as ClientKeyRecord[]
  const keys = (payload as { keys?: unknown } | null)?.keys
  return Array.isArray(keys) ? (keys as ClientKeyRecord[]) : []
}

export function useClientKeys() {
  return useQuery<ClientKeyRecord[]>({
    queryKey: ['client-keys'],
    queryFn: async () => {
      const res = await fetch('/api/client-keys')
      if (!res.ok) throw new Error(`client keys: ${res.status}`)
      return normalizeClientKeys(await res.json())
    },
    staleTime: 30_000,
  })
}

async function apiErrorCode(res: Response, fallback: string): Promise<string> {
  const body = await res.json().catch(() => null)
  const code = (body as { code?: string } | null)?.code
  return code ?? fallback
}

export async function saveClientKey(input: ClientKeyInput): Promise<ClientKeyRecord> {
  const res = await fetch('/api/client-keys', {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(input),
  })
  if (!res.ok) throw new Error(await apiErrorCode(res, 'error.clientKeys.saveFailed'))
  const body = (await res.json()) as ClientKeyRecord
  return { ...body, key: body.key ?? input.key ?? '' }
}

export async function deleteClientKey(key: string): Promise<void> {
  const res = await fetch(`/api/client-keys/${encodeURIComponent(key)}`, { method: 'DELETE' })
  if (!res.ok && res.status !== 404) {
    throw new Error(await apiErrorCode(res, 'error.clientKeys.saveFailed'))
  }
}
