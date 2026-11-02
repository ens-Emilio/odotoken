package guard

import "testing"

func TestCheckBudgetThreshold(t *testing.T) {
	tests := []struct {
		name       string
		current    float64
		limit      float64
		threshold  float64
		want       bool
	}{
		{"below", 50, 100, 80, false},
		{"at threshold", 80, 100, 80, true},
		{"above", 90, 100, 80, true},
		{"zero limit", 50, 0, 80, false},
		{"zero threshold", 50, 100, 0, false},
		{"exact match", 100, 100, 100, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CheckBudgetThreshold(tt.current, tt.limit, tt.threshold)
			if got != tt.want {
				t.Errorf("CheckBudgetThreshold(%v,%v,%v) = %v, want %v",
					tt.current, tt.limit, tt.threshold, got, tt.want)
			}
		})
	}
}

func TestRecordAndGetBudgetAlerts(t *testing.T) {
	budgetAlerts = nil // reset
	alert := BudgetAlert{Project: "p1", LimitUSD: 100, CurrentUSD: 85}
	RecordBudgetAlert(alert)
	got := GetBudgetAlerts()
	if len(got) != 1 || got[0].Project != "p1" {
		t.Errorf("GetBudgetAlerts() = %+v, want 1 alert for p1", got)
	}
}