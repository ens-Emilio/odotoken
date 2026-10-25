export interface Summary {
  total_requests: number
  total_prompt_tokens: number
  total_completion_tokens: number
  total_tokens: number
  total_cache_read_tokens: number
  total_cache_write_tokens: number
  total_cost_usd: number
  avg_latency_ms: number
  avg_ttfb_ms: number
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
  price_missing_count?: number
}

export interface ClientRow {
  client_app: string
  requests: number
  total_tokens: number
  cost_usd: number
}

export interface EventRow {
  id: number
  timestamp: string
  provider: string
  model: string
  client_app: string
  status_code: number
  prompt_tokens: number
  total_tokens: number
  completion_tokens: number
  cache_read_tokens: number
  cache_write_tokens: number
  cost_usd: number
  latency_ms: number
  ttfb_ms: number
  streaming_ms: number
  tags: string
}

export interface DailyRow {
  day: string
  requests: number
  total_tokens: number
  cost_usd: number
}

export interface GuardInfo {
  limits: {
    daily_cost_limit_usd: number | null
    monthly_cost_limit_usd: number | null
    daily_tokens_limit: number | null
    monthly_tokens_limit: number | null
    warning_threshold_pct: number | null
    action_on_breach: string
    timezone: string | null
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

export interface ProjectRow {
  project_id: string
  requests: number
  total_tokens: number
  cost_usd: number
  shadow_cost_usd: number
}

export interface Savings {
  actual_usd: number
  shadow_usd: number
  saved_usd: number
}
export interface SourceRow {
  source: string
  requests: number
  total_tokens: number
  cost_usd: number
}

export interface TpsBucket {
  bucket: string
  requests: number
  total_tokens: number
  completion_tokens: number
  total_latency_ms: number
  avg_latency_ms: number
  avg_tps: number
  peak_tps: number
}
export interface TpsModelRow { model: string; provider: string; requests: number; completion_tokens: number; total_tokens: number; cost_usd: number; avg_latency_ms: number; tps: number }
export interface TpsProjectRow { project_id: string; requests: number; completion_tokens: number; total_tokens: number; cost_usd: number; avg_latency_ms: number; tps: number }
export interface TpsSummary { requests: number; completion_tokens: number; total_tokens: number; total_latency_ms: number; avg_latency_ms: number; avg_tps: number; peak_tps: number; live_tps: number }
export interface TpsData { series: TpsBucket[]; by_model: TpsModelRow[]; by_project: TpsProjectRow[]; summary: TpsSummary; live_tps: number; interval: string }
export interface TagRow { tag: string; requests: number; total_tokens: number; cost_usd: number }
export interface ForecastPoint { day: string; cost_usd: number }
export interface Anomaly { day: string; cost_usd: number; threshold: number }
export interface Forecast { forecast: ForecastPoint[]; anomalies: Anomaly[]; daily: DailyRow[] }
export interface Stats {
  summary: Summary
  by_model: ModelRow[]
  by_client: ClientRow[]
  by_project: ProjectRow[]
  by_source: SourceRow[]
  savings: Savings
  recent_events: EventRow[]
  daily: DailyRow[]
  tps: TpsSummary
  guard: GuardInfo
  by_tag: TagRow[]
  live_sessions: LiveSession[]
}

export interface LiveSession {
  run_id: string
  cli: string
  session_id: string
  provider: string
  model: string
  tokens_input: number
  tokens_output: number
  cost_usd: number
  project: string
  title: string
  started_at: string
  last_updated: string
  elapsed_ms: number
}