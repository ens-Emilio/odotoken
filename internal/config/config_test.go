package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFoldsLegacyLimitsIntoGuard(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	content := []byte(`{
		"limits": {
			"daily_cost_limit_usd": 10.5,
			"monthly_cost_limit_usd": 42,
			"daily_tokens_limit": 5000,
			"monthly_tokens_limit": 50000,
			"warning_threshold_pct": 75
		}
	}`)
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.Guard.DailyCostLimitUSD != 10.5 || cfg.Guard.MonthlyCostLimitUSD != 42 {
		t.Fatalf("cost limits = %v/%v", cfg.Guard.DailyCostLimitUSD, cfg.Guard.MonthlyCostLimitUSD)
	}
	if cfg.Guard.DailyTokenLimit != 5000 || cfg.Guard.MonthlyTokenLimit != 50000 {
		t.Fatalf("token limits = %v/%v", cfg.Guard.DailyTokenLimit, cfg.Guard.MonthlyTokenLimit)
	}
	if cfg.Guard.WarningThresholdPct != 75 {
		t.Fatalf("warning threshold = %v", cfg.Guard.WarningThresholdPct)
	}
}

func TestLoadResolvesAPIKeyFromEnv(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	content := []byte(`{"server": {}}`)
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	t.Setenv("ODO_API_KEY", "env-key")
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.Server.APIKey != "env-key" {
		t.Fatalf("api key = %q", cfg.Server.APIKey)
	}
}

func TestDefaultConfigBindsLoopback(t *testing.T) {
	if got := DefaultConfig().Server.BindHost; got != "127.0.0.1" {
		t.Fatalf("default bind host = %q, want loopback", got)
	}
}
