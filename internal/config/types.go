package config

import "encoding/json"

// Config is the root configuration structure.
type Config struct {
	TokenCounter TokenCounterConfig        `json:"token_counter"`
	Sessions     SessionsConfig            `json:"sessions"`
	Cache        CacheConfig               `json:"cache"`
	Limits       LimitsConfig              `json:"limits"`
	Alerts       AlertsConfig              `json:"alerts"`
	Guard        GuardConfig               `json:"guard"`
	Server       ServerConfig              `json:"server"`
	Providers    map[string]UpstreamConfig `json:"providers"`
}

// ServerConfig for HTTP server behavior.
type ServerConfig struct {
	BindHost      string   `json:"bind_host"`
	APIKey        string   `json:"api_key"`
	CORSAllowlist []string `json:"cors_allowlist"`
}

// UpstreamConfig holds the upstream URL for a provider.
type UpstreamConfig struct {
	UpstreamURL string `json:"upstream_url"`
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

// CacheConfig for semantic caching.
type CacheConfig struct {
	Semantic SemanticCacheConfig `json:"semantic"`
}

// SemanticCacheConfig for semantic cache settings.
type SemanticCacheConfig struct {
	Enabled bool `json:"enabled"`
	TTL     int  `json:"ttl"`
}

// LimitsConfig is kept for backward compatibility. UnmarshalJSON folds its
// budget fields into Guard; new code should use the `guard` block.
type LimitsConfig struct {
	DailyCostLimitUSD   float64         `json:"daily_cost_limit_usd"`
	MonthlyCostLimitUSD float64         `json:"monthly_cost_limit_usd"`
	DailyTokenLimit     int             `json:"daily_tokens_limit"`
	MonthlyTokenLimit   int             `json:"monthly_tokens_limit"`
	WarningThresholdPct float64         `json:"warning_threshold_pct"`
	RateLimit           RateLimitConfig `json:"rate_limit"`
}

// RateLimitConfig for rate limiter settings.
type RateLimitConfig struct {
	Enabled   bool    `json:"enabled"`
	RPS       float64 `json:"rps"`
	BurstSize int     `json:"burst_size"`
}

// AlertsConfig for notification channels.
type AlertsConfig struct {
	SlackURL   string         `json:"slack_url"`
	DiscordURL string         `json:"discord_url"`
	Telegram   TelegramConfig `json:"telegram"`
	Email      EmailConfig    `json:"email"`
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

// GuardConfig for budget enforcement.
type GuardConfig struct {
	DailyCostLimitUSD   float64 `json:"daily_cost_limit_usd"`
	MonthlyCostLimitUSD float64 `json:"monthly_cost_limit_usd"`
	DailyTokenLimit     int     `json:"daily_tokens_limit"`
	MonthlyTokenLimit   int     `json:"monthly_tokens_limit"`
	WarningThresholdPct float64 `json:"warning_threshold_pct"`
	ActionOnBreach      string  `json:"action_on_breach"`
	Timezone            string  `json:"timezone"`
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
	if c.Guard.ActionOnBreach == "" {
		c.Guard.ActionOnBreach = "block"
	}
	return nil
}

// LegacyLimitsConfig is the old `limits` budget block. It is read only to
// migrate existing configs into Guard; new code should use `guard`.
type LegacyLimitsConfig struct {
	DailyCostLimitUSD   float64 `json:"daily_cost_limit_usd"`
	MonthlyCostLimitUSD float64 `json:"monthly_cost_limit_usd"`
	DailyTokenLimit     int     `json:"daily_tokens_limit"`
	MonthlyTokenLimit   int     `json:"monthly_tokens_limit"`
	WarningThresholdPct float64 `json:"warning_threshold_pct"`
	ActionOnBreach      string  `json:"action_on_breach"`
}
