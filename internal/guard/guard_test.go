package guard

import (
	"bytes"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	"github.com/ens-Emilio/odotoken/internal/config"
	"github.com/ens-Emilio/odotoken/internal/db"
	"github.com/ens-Emilio/odotoken/internal/domain"
)

func TestCheckHonorsConfiguredBreachAction(t *testing.T) {
	database, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer database.Close()
	_, err = database.InsertEvent(domain.TokenEvent{
		Timestamp:        time.Now().UTC(),
		Provider:         "openai",
		Model:            "gpt-4o",
		PromptTokens:     100,
		CompletionTokens: 50,
		TotalTokens:      150,
		CostUSD:          2.5,
		ProjectID:        "default",
	})
	if err != nil {
		t.Fatalf("insert event: %v", err)
	}

	limits := config.GuardConfig{DailyCostLimitUSD: 1, ActionOnBreach: "alert"}
	logger := slog.New(slog.NewTextHandler(bytes.NewBuffer(nil), nil))
	result := New(database, limits, config.AlertsConfig{}, logger).Check("default")
	if !result.Allowed {
		t.Fatalf("alert-mode breach must continue serving requests: %+v", result)
	}
	if result.Status != "breached" || result.ActionOnBreach != "alert" {
		t.Fatalf("alert-mode breach result = %+v", result)
	}

	limits.ActionOnBreach = "block"
	result = New(database, limits, config.AlertsConfig{}, logger).Check("default")
	if result.Allowed {
		t.Fatalf("block-mode breach was unexpectedly allowed: %+v", result)
	}
}

func openTestGuard(t *testing.T, limits config.GuardConfig) (*db.Database, *Guard) {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	logger := slog.New(slog.NewTextHandler(bytes.NewBuffer(nil), nil))
	return database, New(database, limits, config.AlertsConfig{}, logger)
}

func insertSpend(t *testing.T, database *db.Database, project, client string, cost float64, tokens int) {
	t.Helper()
	if _, err := database.InsertEvent(domain.TokenEvent{
		Timestamp:        time.Now().UTC(),
		Provider:         "openai",
		Model:            "gpt-4o",
		ClientApp:        client,
		PromptTokens:     tokens,
		CompletionTokens: 0,
		TotalTokens:      tokens,
		CostUSD:          cost,
		ProjectID:        project,
	}); err != nil {
		t.Fatalf("insert event: %v", err)
	}
}

// A key that is well inside its own quota must still be blocked once the
// project as a whole is over budget: the project limit is not a per-key
// allowance, and measuring the key alone hid the overrun.
func TestQuotaKeepsProjectLimitWhenKeyIsUnderItsQuota(t *testing.T) {
	database, guard := openTestGuard(t, config.GuardConfig{DailyCostLimitUSD: 1, ActionOnBreach: "block"})
	insertSpend(t, database, "default", "client-a", 0.10, 100)
	insertSpend(t, database, "default", "client-b", 1.50, 100)

	result := guard.CheckWithQuota("default", "client-a", config.GuardConfig{DailyCostLimitUSD: 5})
	if result.Allowed {
		t.Fatalf("project over budget but request allowed: %+v", result)
	}
	if result.Status != "breached" || result.ReasonCode != "guard.dailyCostBreached" {
		t.Fatalf("result = %+v, want a project cost breach", result)
	}
	if result.CurrentDailyCost < 1.5 {
		t.Fatalf("reported spend = %v, want the project total so the block is explainable", result.CurrentDailyCost)
	}
}

// The same overrun must not be ignored by a second key just because that key
// carries no quota of its own.
func TestQuotaWithoutKeyLimitsStillHonorsProjectLimit(t *testing.T) {
	database, guard := openTestGuard(t, config.GuardConfig{DailyCostLimitUSD: 1, ActionOnBreach: "block"})
	insertSpend(t, database, "default", "client-a", 2.00, 100)

	result := guard.CheckWithQuota("default", "client-b", config.GuardConfig{})
	if result.Allowed {
		t.Fatalf("key without a quota bypassed the project limit: %+v", result)
	}
	if result.Status != "breached" {
		t.Fatalf("status = %q, want breached", result.Status)
	}
}

// A key's own quota is still enforced while the project is well under its
// limits, and the result reports the key's spend against the key's limit.
func TestKeyQuotaBlocksInsideHealthyProject(t *testing.T) {
	database, guard := openTestGuard(t, config.GuardConfig{DailyCostLimitUSD: 100, ActionOnBreach: "block"})
	insertSpend(t, database, "default", "client-a", 0.60, 100)
	insertSpend(t, database, "default", "client-b", 0.10, 100)

	result := guard.CheckWithQuota("default", "client-a", config.GuardConfig{DailyCostLimitUSD: 0.50})
	if result.Allowed {
		t.Fatalf("key over its own quota was allowed: %+v", result)
	}
	if result.DailyLimit != 0.50 {
		t.Fatalf("daily limit = %v, want the key quota", result.DailyLimit)
	}
	if result.CurrentDailyCost != 0.60 {
		t.Fatalf("reported spend = %v, want the key's own spend", result.CurrentDailyCost)
	}

	other := guard.CheckWithQuota("default", "client-b", config.GuardConfig{DailyCostLimitUSD: 0.50})
	if !other.Allowed {
		t.Fatalf("a different key inside its quota was blocked: %+v", other)
	}
}

// Token quotas follow the same split, and the breach action still comes from
// the project guard so alert-mode deployments keep serving.
func TestKeyTokenQuotaInheritsBreachAction(t *testing.T) {
	database, guard := openTestGuard(t, config.GuardConfig{DailyTokenLimit: 10000, ActionOnBreach: "alert"})
	insertSpend(t, database, "default", "client-a", 0.01, 5000)
	insertSpend(t, database, "default", "client-b", 0.01, 100)

	result := guard.CheckWithQuota("default", "client-a", config.GuardConfig{DailyTokenLimit: 1000})
	if result.Status != "breached" {
		t.Fatalf("status = %q, want breached on the key token quota", result.Status)
	}
	if !result.Allowed || result.ActionOnBreach != "alert" {
		t.Fatalf("alert-mode breach must keep serving: %+v", result)
	}
}

// A project warning must survive a key that is comfortably inside its quota,
// so the dashboard keeps showing the warning the operator configured.
func TestProjectWarningSurvivesHealthyKeyQuota(t *testing.T) {
	database, guard := openTestGuard(t, config.GuardConfig{DailyCostLimitUSD: 1, WarningThresholdPct: 80, ActionOnBreach: "block"})
	insertSpend(t, database, "default", "client-a", 0.90, 100)
	insertSpend(t, database, "default", "client-b", 0.01, 100)

	result := guard.CheckWithQuota("default", "client-b", config.GuardConfig{DailyCostLimitUSD: 50})
	if result.Status != "warning" {
		t.Fatalf("status = %q, want the project warning to survive: %+v", result.Status, result)
	}
	if !result.Allowed {
		t.Fatalf("a warning must not block: %+v", result)
	}
}
