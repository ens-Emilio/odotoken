package guard

import "sync"

// DowngradeRule maps a primary model to its fallback when cost or latency exceeds limits.
type DowngradeRule struct {
	PrimaryModel   string  `json:"primary_model"`
	FallbackModel  string  `json:"fallback_model"`
	MaxCostUSD     float64 `json:"max_cost_usd,omitempty"`
	MaxLatencyMs   int     `json:"max_latency_ms,omitempty"`
}

var (
	downgradeRulesMu sync.RWMutex
	downgradeRules   []DowngradeRule
)

// SetDowngradeRules replaces the active downgrade configuration.
func SetDowngradeRules(rules []DowngradeRule) {
	downgradeRulesMu.Lock()
	defer downgradeRulesMu.Unlock()
	downgradeRules = rules
}

// ResolveFallback returns the fallback model for a primary when limits are exceeded.
// Returns empty string if no rule matches or limits are not breached.
func ResolveFallback(primaryModel string, currentCostUSD float64, latencyMs int) string {
	downgradeRulesMu.RLock()
	defer downgradeRulesMu.RUnlock()
	for _, r := range downgradeRules {
		if r.PrimaryModel != primaryModel {
			continue
		}
		costExceeded := r.MaxCostUSD > 0 && currentCostUSD > r.MaxCostUSD
		latencyExceeded := r.MaxLatencyMs > 0 && latencyMs > r.MaxLatencyMs
		if costExceeded || latencyExceeded {
			return r.FallbackModel
		}
	}
	return ""
}