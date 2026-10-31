package config

import "encoding/json"

// Config is the root configuration structure.
type Config struct {
	TokenCounter    TokenCounterConfig        `json:"token_counter"`
	Sessions        SessionsConfig            `json:"sessions"`
	Limits          LimitsConfig              `json:"limits"`
	Alerts          AlertsConfig              `json:"alerts"`
	Guard           GuardConfig               `json:"guard"`
	Server          ServerConfig              `json:"server"`
	Providers       map[string]UpstreamConfig `json:"providers"`
	Storage         StorageConfig             `json:"storage"`
	ScheduledImport ScheduledImportConfig     `json:"scheduled_import"`
	Otel            OtelConfig                `json:"otel"`
}

// ServerConfig for HTTP server behavior.
type ServerConfig struct {
	BindHost      string                     `json:"bind_host"`
	APIKey        string                     `json:"api_key"`
	CORSAllowlist []string                   `json:"cors_allowlist"`
	ClientKeys    map[string]ClientKeyConfig `json:"client_keys,omitempty"`
}

// ClientKeyConfig is the identity and optional quota a client key grants.
type ClientKeyConfig struct {
	Client              string  `json:"client"`
	Project             string  `json:"project"`
	DailyCostLimitUSD   float64 `json:"daily_cost_limit_usd,omitempty"`
	MonthlyCostLimitUSD float64 `json:"monthly_cost_limit_usd,omitempty"`
	DailyTokenLimit     int     `json:"daily_tokens_limit,omitempty"`
	MonthlyTokenLimit   int     `json:"monthly_tokens_limit,omitempty"`
}

// OtelConfig exports usage as OTLP/HTTP JSON at Endpoint/v1/metrics.
type OtelConfig struct {
	Enabled         bool   `json:"enabled"`
	Endpoint        string `json:"endpoint"`
	IntervalSeconds int    `json:"interval_seconds"`
}

// UpstreamConfig holds the upstream URL for a provider and the ordered
// fallbacks used when it answers 429 or 5xx before the response starts.
// FallbackOnStatus narrows that trigger; empty means the transient defaults.
type UpstreamConfig struct {
	UpstreamURL      string   `json:"upstream_url"`
	Fallbacks        []string `json:"fallbacks,omitempty"`
	FallbackOnStatus []int    `json:"fallback_on_status,omitempty"`
}

// TokenCounterConfig controls token counting behavior.
type TokenCounterConfig struct {
	Enabled   bool                      `json:"enabled"`
	Providers map[string]ProviderConfig `json:"providers"`
	Heuristic HeuristicConfig           `json:"heuristic"`
}

// ProviderConfig for a single LLM provider.
type ProviderConfig struct {
	Enabled        bool    `json:"enabled"`
	APIKeyEnv      string  `json:"api_key_env"`
	BaseURL        string  `json:"base_url"`
	TimeoutSeconds float64 `json:"timeout_seconds"`
	MaxRetries     int     `json:"max_retries"`
	CountEndpoint  string  `json:"count_endpoint"`
}

// HeuristicConfig for fallback token counting.
type HeuristicConfig struct {
	CharsPerToken float64 `json:"chars_per_token"`
	WordsPerToken float64 `json:"words_per_token"`
}

// SessionsConfig for session tracking.
type SessionsConfig struct {
	Enabled bool              `json:"enabled"`
	Paths   map[string]string `json:"paths"`
}

// LimitsConfig holds the request limiter. The legacy budget fields that used
// to live here are read by LegacyLimitsConfig and folded into Guard.
type LimitsConfig struct {
	RateLimit RateLimitConfig `json:"rate_limit"`
}

// RateLimitConfig configures the optional request limiter. Projects and
// Clients hold overrides keyed by X-Odo-Project and X-Odo-Client; an override
// inherits the top-level rps/burst when its own value is zero.
type RateLimitConfig struct {
	Enabled   bool                       `json:"enabled"`
	RPS       float64                    `json:"rps"`
	BurstSize int                        `json:"burst_size"`
	Projects  map[string]RateLimitConfig `json:"projects,omitempty"`
	Clients   map[string]RateLimitConfig `json:"clients,omitempty"`
}

// AlertsConfig for notification channels.
type AlertsConfig struct {
	WebhookURL string         `json:"webhook_url"`
	SlackURL   string         `json:"slack_url"`
	DiscordURL string         `json:"discord_url"`
	Telegram   TelegramConfig `json:"telegram"`
	Email      EmailConfig    `json:"email"`
	Retry      AlertRetry     `json:"retry"`
}

// AlertRetry controls the durable alert outbox.
type AlertRetry struct {
	MaxAttempts      int `json:"max_attempts"`
	BaseDelaySeconds int `json:"base_delay_seconds"`
	MaxDelaySeconds  int `json:"max_delay_seconds"`
}

// TelegramConfig for Telegram alerts.
type TelegramConfig struct {
	BotTokenEnv string `json:"bot_token_env"`
	BotToken    string `json:"bot_token"`
	ChatID      string `json:"chat_id"`
}

// EmailConfig for email alerts.
type EmailConfig struct {
	SMTPHost string `json:"smtp_host"`
	SMTPPort int    `json:"smtp_port"`
	Username string `json:"username"`
	Password string `json:"password"`
	FromAddr string `json:"from_addr"`
	ToAddr   string `json:"to_addr"`
}

// GuardConfig for budget enforcement. Projects overrides individual fields
// for one project; anything left nil falls back to the global value.
type GuardConfig struct {
	DailyCostLimitUSD   float64                       `json:"daily_cost_limit_usd"`
	MonthlyCostLimitUSD float64                       `json:"monthly_cost_limit_usd"`
	DailyTokenLimit     int                           `json:"daily_tokens_limit"`
	MonthlyTokenLimit   int                           `json:"monthly_tokens_limit"`
	WarningThresholdPct float64                       `json:"warning_threshold_pct"`
	ActionOnBreach      string                        `json:"action_on_breach"`
	Timezone            string                        `json:"timezone"`
	Projects            map[string]ProjectGuardConfig `json:"projects,omitempty"`
}

// ProjectGuardConfig is a partial override: nil means "inherit global".
type ProjectGuardConfig struct {
	DailyCostLimitUSD   *float64 `json:"daily_cost_limit_usd,omitempty"`
	MonthlyCostLimitUSD *float64 `json:"monthly_cost_limit_usd,omitempty"`
	DailyTokenLimit     *int     `json:"daily_tokens_limit,omitempty"`
	MonthlyTokenLimit   *int     `json:"monthly_tokens_limit,omitempty"`
	WarningThresholdPct *float64 `json:"warning_threshold_pct,omitempty"`
	ActionOnBreach      *string  `json:"action_on_breach,omitempty"`
	Timezone            *string  `json:"timezone,omitempty"`
}

// ForProject returns the effective limits for one project.
func (g GuardConfig) ForProject(projectID string) GuardConfig {
	override, ok := g.Projects[projectID]
	if !ok {
		return g
	}
	effective := g
	if override.DailyCostLimitUSD != nil {
		effective.DailyCostLimitUSD = *override.DailyCostLimitUSD
	}
	if override.MonthlyCostLimitUSD != nil {
		effective.MonthlyCostLimitUSD = *override.MonthlyCostLimitUSD
	}
	if override.DailyTokenLimit != nil {
		effective.DailyTokenLimit = *override.DailyTokenLimit
	}
	if override.MonthlyTokenLimit != nil {
		effective.MonthlyTokenLimit = *override.MonthlyTokenLimit
	}
	if override.WarningThresholdPct != nil {
		effective.WarningThresholdPct = *override.WarningThresholdPct
	}
	if override.ActionOnBreach != nil {
		effective.ActionOnBreach = *override.ActionOnBreach
	}
	if override.Timezone != nil {
		effective.Timezone = *override.Timezone
	}
	return effective
}

// StorageConfig bounds local history growth.
type StorageConfig struct {
	TokenEventsRetentionDays int  `json:"token_events_retention_days"`
	VacuumAfterRetention     bool `json:"vacuum_after_retention"`
}

// ScheduledImportConfig runs the shared importer on a fixed interval while
// the daemon is up.
type ScheduledImportConfig struct {
	Enabled  bool     `json:"enabled"`
	Interval string   `json:"interval"`
	CLIs     []string `json:"clis,omitempty"`
}

// UnmarshalJSON accepts the legacy top-level `limits` block as well as the
// current `guard` block. The dashboard documents both spellings, and reading
// only one of them silently disables limits for existing configs.
func (c *Config) UnmarshalJSON(data []byte) error {
	type plain Config
	current := plain(*c)
	if err := json.Unmarshal(data, &current); err != nil {
		return err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	var legacy struct {
		Limits *LegacyLimitsConfig `json:"limits"`
	}
	if err := json.Unmarshal(data, &legacy); err != nil {
		return err
	}
	*c = Config(current)
	_, hasModernGuard := raw["guard"]
	if legacy.Limits != nil && !hasModernGuard {
		c.Guard = GuardConfig{
			DailyCostLimitUSD:   legacy.Limits.DailyCostLimitUSD,
			MonthlyCostLimitUSD: legacy.Limits.MonthlyCostLimitUSD,
			DailyTokenLimit:     legacy.Limits.DailyTokenLimit,
			MonthlyTokenLimit:   legacy.Limits.MonthlyTokenLimit,
			WarningThresholdPct: legacy.Limits.WarningThresholdPct,
			ActionOnBreach:      legacy.Limits.ActionOnBreach,
		}
	}
	if legacy.Limits != nil && !c.Limits.RateLimit.Enabled && c.Limits.RateLimit.RPS == 0 {
		c.Limits.RateLimit = legacy.Limits.RateLimit
	}
	if c.Guard.ActionOnBreach == "" {
		c.Guard.ActionOnBreach = "block"
	}
	return nil
}

// LegacyLimitsConfig is the old `limits` block. It is read only to migrate
// existing configs into Guard; new code should use `guard`.
type LegacyLimitsConfig struct {
	DailyCostLimitUSD   float64         `json:"daily_cost_limit_usd"`
	MonthlyCostLimitUSD float64         `json:"monthly_cost_limit_usd"`
	DailyTokenLimit     int             `json:"daily_tokens_limit"`
	MonthlyTokenLimit   int             `json:"monthly_tokens_limit"`
	WarningThresholdPct float64         `json:"warning_threshold_pct"`
	ActionOnBreach      string          `json:"action_on_breach"`
	RateLimit           RateLimitConfig `json:"rate_limit"`
}
