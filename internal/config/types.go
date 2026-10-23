package config

// Config is the root configuration structure.
type Config struct {
	TokenCounter TokenCounterConfig `json:"token_counter"`
	Sessions     SessionsConfig     `json:"sessions"`
	Cache        CacheConfig        `json:"cache"`
	Limits       LimitsConfig       `json:"limits"`
	Alerts       AlertsConfig       `json:"alerts"`
	Guard        GuardConfig        `json:"guard"`
	Providers    map[string]UpstreamConfig `json:"providers"`
}

// UpstreamConfig holds the upstream URL for a provider.
type UpstreamConfig struct {
	UpstreamURL string `json:"upstream_url"`
}

// TokenCounterConfig controls token counting behavior.
type TokenCounterConfig struct {
	Enabled   bool                       `json:"enabled"`
	Providers map[string]ProviderConfig  `json:"providers"`
	Heuristic HeuristicConfig            `json:"heuristic"`
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

// LimitsConfig for rate limiting.
type LimitsConfig struct {
	RateLimit RateLimitConfig `json:"rate_limit"`
}

// RateLimitConfig for rate limiter settings.
type RateLimitConfig struct {
	Enabled    bool    `json:"enabled"`
	RPS        float64 `json:"rps"`
	BurstSize  int     `json:"burst_size"`
}

// AlertsConfig for notification channels.
type AlertsConfig struct {
	SlackURL   string            `json:"slack_url"`
	DiscordURL string            `json:"discord_url"`
	Telegram   TelegramConfig    `json:"telegram"`
	Email      EmailConfig       `json:"email"`
}

// TelegramConfig for Telegram alerts.
type TelegramConfig struct {
	BotTokenEnv string `json:"bot_token_env"`
	BotToken    string `json:"bot_token"`
	ChatID      string `json:"chat_id"`
}

// EmailConfig for email alerts.
type EmailConfig struct {
	SMTPHost  string `json:"smtp_host"`
	SMTPPort  int    `json:"smtp_port"`
	Username  string `json:"username"`
	Password  string `json:"password"`
	FromAddr  string `json:"from_addr"`
	ToAddr    string `json:"to_addr"`
}

// GuardConfig for budget enforcement.
type GuardConfig struct {
	DailyCostLimitUSD   float64 `json:"daily_cost_limit_usd"`
	MonthlyCostLimitUSD float64 `json:"monthly_cost_limit_usd"`
	DailyTokenLimit     int     `json:"daily_token_limit"`
	MonthlyTokenLimit   int     `json:"monthly_token_limit"`
	WarningThresholdPct float64 `json:"warning_threshold_pct"`
}