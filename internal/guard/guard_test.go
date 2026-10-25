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
