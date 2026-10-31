package config

import (
	"encoding/json"
	"os"
)

// DefaultConfig returns safe local-first defaults.
func DefaultConfig() Config {
	return Config{
		TokenCounter: TokenCounterConfig{
			Enabled:   true,
			Heuristic: HeuristicConfig{CharsPerToken: 4.0, WordsPerToken: 0.75},
			Providers: make(map[string]ProviderConfig),
		},
		Sessions:        SessionsConfig{Enabled: true, Paths: make(map[string]string)},
		Guard:           GuardConfig{WarningThresholdPct: 80.0, ActionOnBreach: "block"},
		Server:          ServerConfig{BindHost: "127.0.0.1"},
		ScheduledImport: ScheduledImportConfig{Interval: "24h"},
		Alerts:          AlertsConfig{Retry: AlertRetry{MaxAttempts: 5, BaseDelaySeconds: 30, MaxDelaySeconds: 3600}},
	}
}

// Load reads config from a JSON file, falling back to defaults.
func Load(path string) (Config, error) {
	cfg := DefaultConfig()
	if path == "" {
		path = "config.json"
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, err
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}
	applyEnvOverrides(&cfg)
	if cfg.Server.BindHost == "" {
		cfg.Server.BindHost = "127.0.0.1"
	}
	if cfg.Alerts.Retry.MaxAttempts <= 0 {
		cfg.Alerts.Retry.MaxAttempts = 5
	}
	if cfg.Alerts.Retry.BaseDelaySeconds <= 0 {
		cfg.Alerts.Retry.BaseDelaySeconds = 30
	}
	if cfg.Alerts.Retry.MaxDelaySeconds <= 0 {
		cfg.Alerts.Retry.MaxDelaySeconds = 3600
	}
	return cfg, nil
}

func applyEnvOverrides(cfg *Config) {
	// API key and alert credentials resolve from env so they do not sit in a
	// world-readable config file.
	if cfg.Server.APIKey == "" {
		if key := os.Getenv("ODO_API_KEY"); key != "" {
			cfg.Server.APIKey = key
		}
	}
	if cfg.Alerts.Telegram.BotToken == "" && cfg.Alerts.Telegram.BotTokenEnv != "" {
		cfg.Alerts.Telegram.BotToken = os.Getenv(cfg.Alerts.Telegram.BotTokenEnv)
	}
	if cfg.Alerts.Email.Username == "" {
		cfg.Alerts.Email.Username = os.Getenv("ODO_SMTP_USER")
	}
	if cfg.Alerts.Email.Password == "" {
		cfg.Alerts.Email.Password = os.Getenv("ODO_SMTP_PASS")
	}
	for name, prov := range cfg.TokenCounter.Providers {
		if prov.APIKeyEnv != "" {
			if key := os.Getenv(prov.APIKeyEnv); key != "" {
				prov.Enabled = true
				cfg.TokenCounter.Providers[name] = prov
			}
		}
	}
}
