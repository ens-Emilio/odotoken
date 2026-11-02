package guard

import (
	"sync"
	"time"
)

// BudgetAlert tracks projected spend against configured limits.
type BudgetAlert struct {
	Project       string    `json:"project"`
	LimitUSD      float64   `json:"limit_usd"`
	CurrentUSD    float64   `json:"current_usd"`
	ProjectedUSD  float64   `json:"projected_usd"`
	ThresholdPct  float64   `json:"threshold_pct"`
	TriggeredAt   time.Time `json:"triggered_at"`
}

var (
	budgetAlertsMu sync.RWMutex
	budgetAlerts   []BudgetAlert
)

// RecordBudgetAlert stores a triggered budget alert for later retrieval.
func RecordBudgetAlert(alert BudgetAlert) {
	budgetAlertsMu.Lock()
	defer budgetAlertsMu.Unlock()
	budgetAlerts = append(budgetAlerts, alert)
}

// GetBudgetAlerts returns all recorded budget alerts.
func GetBudgetAlerts() []BudgetAlert {
	budgetAlertsMu.RLock()
	defer budgetAlertsMu.RUnlock()
	out := make([]BudgetAlert, len(budgetAlerts))
	copy(out, budgetAlerts)
	return out
}

// CheckBudgetThreshold returns true when current spend exceeds threshold percent of limit.
func CheckBudgetThreshold(currentUSD, limitUSD, thresholdPct float64) bool {
	if limitUSD <= 0 || thresholdPct <= 0 {
		return false
	}
	return currentUSD >= limitUSD*(thresholdPct/100.0)
}