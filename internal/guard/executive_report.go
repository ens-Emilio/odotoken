package guard

import (
	"fmt"
	"strings"
	"time"
)

// ExecutiveReport holds summary data for weekly cost and usage reporting.
type ExecutiveReport struct {
	PeriodStart    time.Time `json:"period_start"`
	PeriodEnd      time.Time `json:"period_end"`
	TotalCostUSD   float64   `json:"total_cost_usd"`
	TotalRequests  int64     `json:"total_requests"`
	TopModels      []ModelUsage `json:"top_models"`
	Anomalies      []string  `json:"anomalies"`
	BudgetStatus   string    `json:"budget_status"`
}

// ModelUsage summarizes usage for a single model in the report.
type ModelUsage struct {
	Model       string  `json:"model"`
	Requests    int64   `json:"requests"`
	CostUSD     float64 `json:"cost_usd"`
	AvgLatencyMs int    `json:"avg_latency_ms"`
}

// FormatText renders the executive report as human-readable text for email/webhook delivery.
func (r *ExecutiveReport) FormatText() string {
	var b strings.Builder
	fmt.Fprintf(&b, "📊 OdoToken Executive Report\n")
	fmt.Fprintf(&b, "Period: %s → %s\n", r.PeriodStart.Format("2006-01-02"), r.PeriodEnd.Format("2006-01-02"))
	fmt.Fprintf(&b, "Budget Status: %s\n\n", r.BudgetStatus)
	fmt.Fprintf(&b, "💰 Total Cost: $%.4f USD\n", r.TotalCostUSD)
	fmt.Fprintf(&b, "📡 Total Requests: %d\n\n", r.TotalRequests)

	if len(r.TopModels) > 0 {
		b.WriteString("🏆 Top Models:\n")
		for i, m := range r.TopModels {
			fmt.Fprintf(&b, "  %d. %s — $%.4f (%d reqs, avg %dms)\n",
				i+1, m.Model, m.CostUSD, m.Requests, m.AvgLatencyMs)
		}
		b.WriteString("\n")
	}

	if len(r.Anomalies) > 0 {
		b.WriteString("⚠️ Anomalies Detected:\n")
		for _, a := range r.Anomalies {
			fmt.Fprintf(&b, "  • %s\n", a)
		}
	}

	return b.String()
}