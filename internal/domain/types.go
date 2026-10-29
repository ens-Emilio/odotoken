package domain

import "time"

// TokenEvent represents a single token consumption record.
type TokenEvent struct {
	ID               int64     `json:"id,omitempty"`
	Timestamp        time.Time `json:"timestamp"`
	Provider         string    `json:"provider"`
	Model            string    `json:"model"`
	ClientApp        string    `json:"client_app"`
	PromptTokens     int       `json:"prompt_tokens"`
	CompletionTokens int       `json:"completion_tokens"`
	TotalTokens      int       `json:"total_tokens"`
	CacheReadTokens  int       `json:"cache_read_tokens"`
	CacheWriteTokens int       `json:"cache_write_tokens"`
	CostUSD          float64   `json:"cost_usd"`
	ShadowCost       float64   `json:"shadow_cost"`
	LatencyMs        float64   `json:"latency_ms"`
	TTFBMs           float64   `json:"ttfb_ms"`
	StreamingMs      float64   `json:"streaming_ms"`
	StatusCode       int       `json:"status_code"`
	ProjectID        string    `json:"project_id"`
	Tags             string    `json:"tags"`
	Metadata         string    `json:"metadata"`
	Source           string    `json:"source"`
	SessionID        string    `json:"session_id"`
	SessionFile      string    `json:"session_file"`
	EntryID          string    `json:"entry_id"`
}

// UsageResult holds parsed token usage from a response or stream. Estimated
// marks values inferred from response size when the provider omitted usage.
type UsageResult struct {
	Provider         string `json:"provider"`
	Model            string `json:"model"`
	PromptTokens     int    `json:"prompt_tokens"`
	CompletionTokens int    `json:"completion_tokens"`
	TotalTokens      int    `json:"total_tokens"`
	CacheReadTokens  int    `json:"cache_read_tokens"`
	CacheWriteTokens int    `json:"cache_write_tokens"`
	Estimated        bool   `json:"estimated,omitempty"`
}

// RawUsage represents a raw usage record from an importer.
type RawUsage struct {
	CLI              string  `json:"cli"`
	SessionID        string  `json:"session_id"`
	SessionFile      string  `json:"session_file"`
	EntryID          string  `json:"entry_id"`
	Timestamp        string  `json:"timestamp"`
	Provider         string  `json:"provider"`
	Model            string  `json:"model"`
	PromptTokens     int     `json:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens"`
	CacheReadTokens  int     `json:"cache_read_tokens"`
	CacheWriteTokens int     `json:"cache_write_tokens"`
	Project          string  `json:"project"`
	RawCost          float64 `json:"raw_cost,omitempty"`
}

// ScanResult holds the result of scanning for importable sessions.
type ScanResult struct {
	CLI          string `json:"cli"`
	Available    bool   `json:"available"`
	SessionFiles int    `json:"session_files"`
	TotalEntries int    `json:"total_entries"`
	Path         string `json:"path"`
	// FirstModified and LastModified bound what the files on disk can cover.
	// Session files carry no reliable global order, so mtime is the only
	// coverage signal available without parsing every file.
	FirstModified time.Time `json:"first_modified,omitempty"`
	LastModified  time.Time `json:"last_modified,omitempty"`
}

// GuardCheckResult holds the result of a budget guard check.
type GuardCheckResult struct {
	Allowed bool   `json:"allowed"`
	Status  string `json:"status"` // "ok", "warning", "breached"
	// Message is the server's own English prose, kept for logs and alert
	// payloads. Clients render ReasonCode/ReasonParams instead: the server does
	// not choose the reader's language.
	Message             string            `json:"message"`
	ReasonCode          string            `json:"reason_code,omitempty"`
	ReasonParams        map[string]string `json:"reason_params,omitempty"`
	CurrentDailyCost    float64           `json:"current_daily_cost"`
	DailyLimit          float64           `json:"daily_limit"`
	CurrentMonthlyCost  float64           `json:"current_monthly_cost"`
	MonthlyLimit        float64           `json:"monthly_limit"`
	CurrentDailyTokens  int               `json:"current_daily_tokens"`
	CurrentMonthTokens  int               `json:"current_monthly_tokens"`
	DailyTokensLimit    int               `json:"daily_tokens_limit"`
	MonthlyTokensLimit  int               `json:"monthly_tokens_limit"`
	WarningThresholdPct float64           `json:"warning_threshold_pct"`
	ActionOnBreach      string            `json:"action_on_breach"`
}
