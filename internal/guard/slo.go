package guard

import (
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/ens-Emilio/odotoken/internal/config"
	"github.com/ens-Emilio/odotoken/internal/db"
)

// SLOTracker measures error-budget burn per provider over a rolling 1h window.
type SLOTracker struct {
	db       *db.Database
	logger   *slog.Logger
	configs  map[string]config.SLOConfig
	exceeded map[string]*atomic.Int64
}

// NewSLOTracker builds a tracker from the upstream configs that have SLO enabled.
func NewSLOTracker(database *db.Database, providers map[string]config.UpstreamConfig, logger *slog.Logger) *SLOTracker {
	cfgs := make(map[string]config.SLOConfig, len(providers))
	exceeded := make(map[string]*atomic.Int64, len(providers))
	for name, u := range providers {
		if u.SLO.Enabled && u.SLO.Availability > 0 {
			cfgs[name] = u.SLO
			exceeded[name] = &atomic.Int64{}
		}
	}
	return &SLOTracker{db: database, logger: logger, configs: cfgs, exceeded: exceeded}
}

// Check returns the current burn rate for a provider and whether it is breached.
// Burn rate = actual_error_rate / allowed_error_rate. Breached when > 2.0.
// A DB error or zero traffic returns (0, false): fail open, same as Guard.
func (t *SLOTracker) Check(provider string) (float64, bool) {
	slo, ok := t.configs[provider]
	if !ok {
		return 0, false
	}

	since := time.Now().Add(-time.Hour)
	total, errors, err := t.db.ErrorBudgetWindow(provider, since)
	if err != nil {
		t.logger.Warn("slo query failed", "provider", provider, "err", err)
		return 0, false
	}
	if total == 0 {
		return 0, false
	}

	actualErrorRate := float64(errors) / float64(total)
	allowedErrorRate := 1.0 - slo.Availability
	if allowedErrorRate <= 0 {
		return 0, false
	}
	burnRate := actualErrorRate / allowedErrorRate

	if burnRate > 2.0 {
		t.exceeded[provider].Add(1)
		return burnRate, true
	}
	return burnRate, false
}

// BurnExceededTotal returns the cumulative breach count for a provider.
func (t *SLOTracker) BurnExceededTotal(provider string) int64 {
	c, ok := t.exceeded[provider]
	if !ok {
		return 0
	}
	return c.Load()
}

// Providers returns the set of tracked provider names.
func (t *SLOTracker) Providers() []string {
	out := make([]string, 0, len(t.configs))
	for p := range t.configs {
		out = append(out, p)
	}
	return out
}