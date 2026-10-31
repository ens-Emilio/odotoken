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

export interface DailyDetail {
  day: string
  summary: Summary
  by_client: ClientRow[]
  by_source: SourceRow[]
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

export interface SessionCost {
  session_id: string
  session_file?: string
  client_app: string
  project_id: string
  models: string
  requests: number
  prompt_tokens: number
  completion_tokens: number
  total_tokens: number
  cache_read_tokens: number
  cache_write_tokens: number
  cost_usd: number
  started_at: string
  ended_at: string
}

export interface CacheSavings {
  requests: number
  cache_read_tokens: number
  cache_write_tokens: number
  actual_cost_usd: number
  read_saved_usd: number
  write_premium_usd: number
  net_saved_usd: number
  uncached_cost_usd: number
  discount_pct: number
  unpriced_models: number
  by_model: Array<{ model: string; provider: string; requests: number; cache_read_tokens: number; cache_write_tokens: number; net_saved_usd: number; discount_pct: number; priced: boolean }>
}

export interface SimulationRow {
  model: string
  input_rate_per_1m: number
  output_rate_per_1m: number
  cache_read_rate_per_1m: number
  cache_write_rate_per_1m: number
  per_request_usd: number
  total_usd: number
  delta_vs_baseline_usd: number
  priced: boolean
}

export interface PriceRow {
  model: string
  input: number
  output: number
  cache_read: number
  cache_write: number
  explicit: boolean
}

export interface AnomalyAttribution { key: string; cost_usd: number; baseline_usd: number; delta_usd: number; share_pct: number }
export interface AttributedAnomaly { day: string; cost_usd: number; threshold: number; z_score: number; by_model: AnomalyAttribution[]; by_project: AnomalyAttribution[] }