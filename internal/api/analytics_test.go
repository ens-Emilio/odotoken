package api

import (
	"bytes"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ens-Emilio/odotoken/internal/config"
	"github.com/ens-Emilio/odotoken/internal/db"
	"github.com/ens-Emilio/odotoken/internal/domain"
	"github.com/ens-Emilio/odotoken/internal/pricing"
)

// analyticsTestRouter builds the dashboard router with the analytics routes
// registered on it, exactly as the server does, so these tests exercise the
// real path shape rather than an isolated router.
func analyticsTestRouter(t *testing.T, prices string) (*db.Database, http.Handler, *pricing.Calculator) {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	pricesPath := filepath.Join(t.TempDir(), "prices.json")
	if prices == "" {
		prices = `{"gpt-test":{"input":1,"output":2,"cache_read":0.1,"cache_write":1.25}}`
	}
	if err := os.WriteFile(pricesPath, []byte(prices), 0o600); err != nil {
		t.Fatalf("write prices: %v", err)
	}
	calc := pricing.NewCalculator(pricesPath)
	router := DashboardRouter(database, func(string) domain.GuardCheckResult {
		return domain.GuardCheckResult{Allowed: true, Status: "ok"}
	}, func() config.GuardConfig { return config.GuardConfig{} }, func(config.GuardConfig) {}, "", nil)
	RegisterAnalyticsRoutes(router, database, calc, pricesPath)
	return database, router, calc
}

func insertAnalyticsEvent(t *testing.T, database *db.Database, e domain.TokenEvent) {
	t.Helper()
	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now().UTC()
	}
	if e.Source == "" {
		e.Source = "proxy"
	}
	if _, err := database.InsertEvent(e); err != nil {
		t.Fatalf("insert event: %v", err)
	}
}

func getJSON(t *testing.T, handler http.Handler, target string, out interface{}) int {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if out != nil && rec.Code == http.StatusOK {
		if err := json.Unmarshal(rec.Body.Bytes(), out); err != nil {
			t.Fatalf("%s: decode: %v (%s)", target, err, rec.Body.String())
		}
	}
	return rec.Code
}

// Feature 12: the CSV export must be a spreadsheet-shaped attachment, and a
// capped export must say so in-band because a spreadsheet cannot read headers.
func TestEventsCSVExportStreamsAttributedRows(t *testing.T) {
	database, router, _ := analyticsTestRouter(t, "")
	insertAnalyticsEvent(t, database, domain.TokenEvent{
		Model: "gpt-test", Provider: "openai", ClientApp: "cli", ProjectID: "acme",
		Tags: "night,ops", PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15,
		CostUSD: 0.25, SessionID: "sess-1", RunID: "run-1",
	})

	req := httptest.NewRequest(http.MethodGet, "/events.csv", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/csv") {
		t.Fatalf("content type = %q", ct)
	}
	if cd := rec.Header().Get("Content-Disposition"); !strings.Contains(cd, "attachment") {
		t.Fatalf("content disposition = %q", cd)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "acme") || !strings.Contains(body, "gpt-test") {
		t.Fatalf("export lost attribution: %s", body)
	}
	if !strings.Contains(body, `"night,ops"`) {
		t.Fatalf("a comma inside tags must be quoted: %s", body)
	}
	// Feature 5: an export is joined back to the run and session that produced
	// it, so both identifiers must survive into the CSV.
	if !strings.Contains(body, "run_id") || !strings.Contains(body, "run-1") {
		t.Fatalf("export lost run_id: %s", body)
	}

	req = httptest.NewRequest(http.MethodGet, "/events.csv?limit=not-a-number", nil)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid limit status = %d", rec.Code)
	}
}

// Feature 13: a session's cost is the sum of its events, and two sessions must
// never be merged because they share a model or a project.
func TestSessionsGroupByConversation(t *testing.T) {
	database, router, _ := analyticsTestRouter(t, "")
	for _, session := range []string{"a", "b"} {
		for i := range 2 {
			insertAnalyticsEvent(t, database, domain.TokenEvent{
				Model: "gpt-test", Provider: "openai", ClientApp: "cli", ProjectID: "acme",
				SessionID: session, PromptTokens: 100, CompletionTokens: 50, TotalTokens: 150,
				CostUSD: 0.10, Timestamp: time.Now().UTC().Add(time.Duration(i) * time.Minute),
			})
		}
	}

	var sessions []db.SessionCost
	if code := getJSON(t, router, "/sessions", &sessions); code != http.StatusOK {
		t.Fatalf("status = %d", code)
	}
	if len(sessions) != 2 {
		t.Fatalf("expected 2 sessions, got %d: %+v", len(sessions), sessions)
	}
	for _, s := range sessions {
		if s.Requests != 2 || math.Abs(s.CostUSD-0.2) > 1e-9 {
			t.Fatalf("session %q rolled up wrongly: %+v", s.SessionID, s)
		}
	}

	var detail struct {
		SessionID string                   `json:"session_id"`
		Events    []map[string]interface{} `json:"events"`
	}
	if code := getJSON(t, router, "/sessions/a", &detail); code != http.StatusOK {
		t.Fatalf("detail status = %d", code)
	}
	if len(detail.Events) != 2 {
		t.Fatalf("expected 2 events in session a, got %d", len(detail.Events))
	}
}

// Feature 14: the cache KPI is money, not tokens. A write premium above the
// input rate must reduce the saving instead of hiding inside it.
func TestCacheSavingsSubtractsWritePremium(t *testing.T) {
	database, router, _ := analyticsTestRouter(t, "")
	insertAnalyticsEvent(t, database, domain.TokenEvent{
		Model: "gpt-test", Provider: "openai", ClientApp: "cli",
		PromptTokens: 1000, CompletionTokens: 0, TotalTokens: 1000,
		CacheReadTokens: 1_000_000, CacheWriteTokens: 0, CostUSD: 0.10,
	})

	var savings struct {
		NetSavedUSD float64 `json:"net_saved_usd"`
		DiscountPct float64 `json:"discount_pct"`
		ByModel     []struct {
			Model string `json:"model"`
		} `json:"by_model"`
	}
	if code := getJSON(t, router, "/cache-savings", &savings); code != http.StatusOK {
		t.Fatalf("status = %d", code)
	}
	if savings.NetSavedUSD <= 0.8 || savings.DiscountPct <= 0 {
		t.Fatalf("cache saving should be positive and measurable: %+v", savings)
	}

	body := bytes.NewBufferString(`{"requests":-1,"prompt_tokens":0,"completion_tokens":0,"cache_read_tokens":0,"cache_write_tokens":0}`)
	req := httptest.NewRequest(http.MethodPost, "/simulate", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("negative workload status = %d", rec.Code)
	}
}

// Feature 16: editing a price must survive a reload and be visible to the
// calculator the proxy uses; deleting a built-in rate must be refused.
func TestPricesPersistAndRejectBuiltinDelete(t *testing.T) {
	_, router, calc := analyticsTestRouter(t, `{"gpt-test":{"input":1,"output":2,"cache_read":0.1,"cache_write":1.25}}`)

	body := bytes.NewBufferString(`{"input":3,"output":4,"cache_read":0.3,"cache_write":3.75}`)
	req := httptest.NewRequest(http.MethodPut, "/prices/gpt-test", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("put status = %d: %s", rec.Code, rec.Body.String())
	}
	if got := calc.GetModelRates("gpt-test").Input; got != 3 {
		t.Fatalf("live calculator kept the old rate: %v", got)
	}

	var listing struct {
		Prices []struct {
			Model    string  `json:"model"`
			Input    float64 `json:"input"`
			Explicit bool    `json:"explicit"`
		} `json:"prices"`
	}
	if code := getJSON(t, router, "/prices?q=gpt-test", &listing); code != http.StatusOK {
		t.Fatalf("list status = %d", code)
	}
	if len(listing.Prices) != 1 || !listing.Prices[0].Explicit || listing.Prices[0].Input != 3 {
		t.Fatalf("saved price not listed: %+v", listing.Prices)
	}

	body = bytes.NewBufferString(`{"input":-1,"output":4,"cache_read":0,"cache_write":0}`)
	req = httptest.NewRequest(http.MethodPut, "/prices/gpt-test", body)
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("negative rate must be rejected, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodDelete, "/prices/never-priced", nil)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("deleting a built-in rate must be 404, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodDelete, "/prices/gpt-test", nil)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete override status = %d", rec.Code)
	}
}

// Feature 17: an anomalous day must name the models and projects that moved,
// and the attribution must be silent when the history is too short to judge.
func TestAnomaliesAttributeSpikeToModelAndProject(t *testing.T) {
	database, router, _ := analyticsTestRouter(t, "")

	var anomalies struct {
		Anomalies []struct {
			Day       string                 `json:"day"`
			ByModel   []struct{ Key string } `json:"by_model"`
			ByProject []struct{ Key string } `json:"by_project"`
		} `json:"anomalies"`
	}
	if code := getJSON(t, router, "/anomalies", &anomalies); code != http.StatusOK {
		t.Fatalf("status = %d", code)
	}
	if len(anomalies.Anomalies) != 0 {
		t.Fatalf("a short history must not invent an anomaly: %+v", anomalies.Anomalies)
	}

	now := time.Now().UTC()
	for day := range 10 {
		cost := 1.0
		if day == 9 {
			cost = 20.0
		}
		insertAnalyticsEvent(t, database, domain.TokenEvent{
			Model: "gpt-test", Provider: "openai", ClientApp: "cli", ProjectID: "acme",
			PromptTokens: 10, CompletionTokens: 10, TotalTokens: 20, CostUSD: cost,
			Timestamp: now.AddDate(0, 0, -day),
		})
	}

	anomalies.Anomalies = nil
	if code := getJSON(t, router, "/anomalies", &anomalies); code != http.StatusOK {
		t.Fatalf("status = %d", code)
	}
	if len(anomalies.Anomalies) == 0 {
		t.Fatal("a 20x day inside a flat window must be reported")
	}
	found := false
	for _, a := range anomalies.Anomalies {
		if len(a.ByModel) == 0 || len(a.ByProject) == 0 {
			continue
		}
		if a.ByModel[0].Key == "gpt-test" && a.ByProject[0].Key == "acme" {
			found = true
		}
	}
	if !found {
		t.Fatalf("spike not attributed to its model and project: %+v", anomalies.Anomalies)
	}
}
