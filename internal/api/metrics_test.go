package api

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ens-Emilio/odotoken/internal/db"
	"github.com/ens-Emilio/odotoken/internal/domain"
)

// TestMetricsExposesRecordedSpend pins the contract Prometheus scrapes: the
// counters carry the recorded values, not zeroes, and labels are escaped.
func TestMetricsExposesRecordedSpend(t *testing.T) {
	database, err := db.Open(filepath.Join(t.TempDir(), "metrics.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer database.Close()

	if _, err := database.InsertEvent(domain.TokenEvent{
		Timestamp:        time.Now().UTC(),
		Provider:         "openai",
		Model:            `gpt-"quoted"`,
		ClientApp:        "claude",
		PromptTokens:     100,
		CompletionTokens: 50,
		TotalTokens:      150,
		CostUSD:          0.25,
		LatencyMs:        120,
		ProjectID:        "alpha",
		StatusCode:       200,
	}); err != nil {
		t.Fatalf("insert event: %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	MetricsRouter(database).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{
		`odotoken_requests_total{provider="openai",model="gpt-\"quoted\"",project="alpha",status="200"} 1`,
		`odotoken_cost_usd_total{provider="openai",model="gpt-\"quoted\"",project="alpha",status="200"} 0.25`,
		`odotoken_tokens_total{provider="openai",model="gpt-\"quoted\"",project="alpha",status="200",kind="prompt"} 100`,
		`odotoken_alert_outbox_pending 0`,
		`odotoken_request_latency_ms_bucket{le="+Inf"} 1`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("metrics missing %q\n%s", want, body)
		}
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/plain") {
		t.Fatalf("Content-Type = %q, want text/plain", ct)
	}
}
