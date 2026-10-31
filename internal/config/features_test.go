package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestLoadReadsFallbacksAndRetention pins the new operational keys: a typo in
// one of them silently disables the feature, so the JSON names are the test.
func TestLoadReadsFallbacksAndRetention(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	content := []byte(`{
		"providers": {
			"relay": {"upstream_url": "https://relay.example/v1", "fallbacks": ["openai"], "fallback_on_status": [429, 503]},
			"openai": {"upstream_url": "https://api.openai.com"}
		},
		"limits": {"rate_limit": {"enabled": true, "rps": 10, "burst_size": 20,
			"projects": {"batch": {"rps": 2}}, "clients": {"claude": {"rps": 5}}}},
		"alerts": {"retry": {"max_attempts": 3, "base_delay_seconds": 10, "max_delay_seconds": 60}},
		"storage": {"token_events_retention_days": 30, "vacuum_after_retention": true},
		"scheduled_import": {"enabled": true, "interval": "6h", "clis": ["claude"]}
	}`)
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	relay := cfg.Providers["relay"]
	if len(relay.Fallbacks) != 1 || relay.Fallbacks[0] != "openai" {
		t.Fatalf("fallbacks = %v", relay.Fallbacks)
	}
	if len(relay.FallbackOnStatus) != 2 || relay.FallbackOnStatus[0] != 429 {
		t.Fatalf("fallback statuses = %v", relay.FallbackOnStatus)
	}
	rl := cfg.Limits.RateLimit
	if rl.Projects["batch"].RPS != 2 || rl.Clients["claude"].RPS != 5 {
		t.Fatalf("rate limit overrides = %+v", rl)
	}
	if cfg.Alerts.Retry.MaxAttempts != 3 {
		t.Fatalf("retry attempts = %d", cfg.Alerts.Retry.MaxAttempts)
	}
	if cfg.Storage.TokenEventsRetentionDays != 30 || !cfg.Storage.VacuumAfterRetention {
		t.Fatalf("storage = %+v", cfg.Storage)
	}
	if !cfg.ScheduledImport.Enabled || cfg.ScheduledImport.Interval != "6h" {
		t.Fatalf("scheduled import = %+v", cfg.ScheduledImport)
	}
}
