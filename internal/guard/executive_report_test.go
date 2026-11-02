package guard

import (
	"strings"
	"testing"
	"time"
)

func TestExecutiveReportFormatText(t *testing.T) {
	report := &ExecutiveReport{
		PeriodStart:   time.Date(2026, 10, 27, 0, 0, 0, 0, time.UTC),
		PeriodEnd:     time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC),
		TotalCostUSD:  42.50,
		TotalRequests: 1500,
		BudgetStatus:  "OK",
		TopModels: []ModelUsage{
			{Model: "gpt-4", Requests: 800, CostUSD: 30.0, AvgLatencyMs: 1200},
			{Model: "claude-3", Requests: 700, CostUSD: 12.5, AvgLatencyMs: 900},
		},
		Anomalies: []string{"Spike de custo em 2026-10-30"},
	}

	text := report.FormatText()

	checks := []string{
		"OdoToken Executive Report",
		"2026-10-27",
		"2026-11-02",
		"$42.5000",
		"1500",
		"gpt-4",
		"claude-3",
		"Spike de custo",
		"Budget Status: OK",
	}
	for _, want := range checks {
		if !strings.Contains(text, want) {
			t.Errorf("FormatText() missing %q in:\n%s", want, text)
		}
	}
}

func TestExecutiveReportFormatTextEmpty(t *testing.T) {
	report := &ExecutiveReport{
		PeriodStart:  time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC),
		PeriodEnd:    time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC),
		BudgetStatus: "N/A",
	}
	text := report.FormatText()
	if !strings.Contains(text, "OdoToken Executive Report") {
		t.Error("expected header in empty report")
	}
	if strings.Contains(text, "Top Models:") {
		t.Error("expected no Top Models section when list is empty")
	}
}