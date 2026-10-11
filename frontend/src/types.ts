export interface Summary {
  total_requests: number
  total_prompt_tokens: number
  total_completion_tokens: number
  total_tokens: number
  total_cache_read_tokens: number
  total_cache_write_tokens: number
  total_cost_usd: number
  avg_latency_ms: number
}

export interface ModelRow {
  model: string
  provider: string
  requests: number
  prompt_tokens: number
  completion_tokens: number
  total_tokens: number
  cache_read_tokens: number
  cache_write_tokens: number
  cost_usd: number
}

export interface ClientRow {
  client_app: string
  requests: number
  prompt_tokens: number
  completion_tokens: number
  total_tokens: number
  cost_usd: number
}

export interface EventRow {
  id: number
  timestamp: string
  provider: string
  model: string
  client_app: string
  prompt_tokens: number
  completion_tokens: number
  total_tokens: number
  cache_read_tokens: number
  cache_write_tokens: number
  cost_usd: number
  latency_ms: number
  status_code: number
}

export interface DailyRow {
  day: string
  request_count: number
  total_tokens: number
  cost_usd: number
}

export interface GuardInfo {
  limits: {
    daily_cost_limit_usd: number | null
    monthly_cost_limit_usd: number | null
    daily_tokens_limit: number | null
    monthly_tokens_limit: number | null
    action_on_breach: string
  }
  current?: {
    daily_cost_usd: number
    monthly_cost_usd: number
    daily_tokens: number
    monthly_tokens: number
  }
  status: string
  allowed: boolean
  reason: string
}

export interface Stats {
  summary: Summary
  by_model: ModelRow[]
  by_client: ClientRow[]
  recent_events: EventRow[]
  daily: DailyRow[]
  guard: GuardInfo
}