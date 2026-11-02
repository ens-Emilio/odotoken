package guard

import "testing"

func TestResolveFallback(t *testing.T) {
	SetDowngradeRules([]DowngradeRule{
		{PrimaryModel: "gpt-4", FallbackModel: "gpt-3.5", MaxCostUSD: 1.0},
		{PrimaryModel: "claude-3", FallbackModel: "claude-haiku", MaxLatencyMs: 5000},
	})
	defer SetDowngradeRules(nil)

	tests := []struct {
		name    string
		model   string
		cost    float64
		latency int
		want    string
	}{
		{"no breach", "gpt-4", 0.5, 1000, ""},
		{"cost breach", "gpt-4", 1.5, 1000, "gpt-3.5"},
		{"latency breach", "claude-3", 0.1, 6000, "claude-haiku"},
		{"no rule", "unknown-model", 10.0, 99999, ""},
		{"both ok", "claude-3", 0.1, 1000, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveFallback(tt.model, tt.cost, tt.latency)
			if got != tt.want {
				t.Errorf("ResolveFallback(%q,%v,%v) = %q, want %q",
					tt.model, tt.cost, tt.latency, got, tt.want)
			}
		})
	}
}