package api

import (
	"bytes"
	"encoding/json"
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
)

func TestGuardLimitsPatchPersistsAndUpdatesLiveGuard(t *testing.T) {
	database, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer database.Close()

	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"providers":{"openai":{"upstream_url":"https://api.openai.com"}}}`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	current := config.GuardConfig{WarningThresholdPct: 80}
	checker := func(string) domain.GuardCheckResult {
		return domain.GuardCheckResult{Allowed: true, Status: "ok"}
	}
	get := func() config.GuardConfig { return current }
	update := func(v config.GuardConfig) { current = v }
	router := DashboardRouter(database, checker, get, update, path)

	body := bytes.NewBufferString(`{
		"daily_cost_limit_usd": 2.5,
		"monthly_cost_limit_usd": 20,
		"daily_tokens_limit": 1000,
		"monthly_tokens_limit": 10000,
		"warning_threshold_pct": 75
	}`)
	req := httptest.NewRequest(http.MethodPatch, "/guard/limits", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("patch status = %d: %s", rec.Code, rec.Body.String())
	}
	if current.DailyTokenLimit != 1000 || current.MonthlyTokenLimit != 10000 || current.WarningThresholdPct != 75 {
		t.Fatalf("in-memory limits not updated: %+v", current)
	}

	persisted, err := config.Load(path)
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}
	if persisted.Guard.DailyTokenLimit != 1000 || persisted.Guard.MonthlyTokenLimit != 10000 {
		t.Fatalf("persisted token limits = %+v", persisted.Guard)
	}
	if persisted.Providers["openai"].UpstreamURL != "https://api.openai.com" {
		t.Fatalf("provider config was not preserved")
	}

	getReq := httptest.NewRequest(http.MethodGet, "/guard/limits", nil)
	getRec := httptest.NewRecorder()
	router.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("get status = %d", getRec.Code)
	}
	var got config.GuardConfig
	if err := json.NewDecoder(getRec.Body).Decode(&got); err != nil {
		t.Fatalf("decode get: %v", err)
	}
	if got.DailyTokenLimit != 1000 {
		t.Fatalf("GET returned stale limits: %+v", got)
	}
}

func TestGuardLimitsPatchAcceptsNullsAndBreachAction(t *testing.T) {
	database, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer database.Close()

	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"limits":{"rate_limit":{"enabled":true,"rps":2}}}`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	current := config.GuardConfig{
		DailyCostLimitUSD:   5,
		MonthlyCostLimitUSD: 10,
		DailyTokenLimit:     100,
		MonthlyTokenLimit:   200,
		WarningThresholdPct: 80,
		ActionOnBreach:      "block",
	}
	router := DashboardRouter(database,
		func(string) domain.GuardCheckResult { return domain.GuardCheckResult{Allowed: true, Status: "ok"} },
		func() config.GuardConfig { return current },
		func(v config.GuardConfig) { current = v },
		path,
	)

	body := bytes.NewBufferString(`{
		"daily_cost_limit_usd": null,
		"monthly_cost_limit_usd": 0,
		"daily_tokens_limit": null,
		"monthly_tokens_limit": 0,
		"warning_threshold_pct": null,
		"action_on_breach": "alert"
	}`)
	req := httptest.NewRequest(http.MethodPatch, "/guard/limits", body)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("patch status = %d: %s", rec.Code, rec.Body.String())
	}
	if current.DailyCostLimitUSD != 0 || current.MonthlyCostLimitUSD != 0 ||
		current.DailyTokenLimit != 0 || current.MonthlyTokenLimit != 0 ||
		current.WarningThresholdPct != 0 || current.ActionOnBreach != "alert" {
		t.Fatalf("null/zero limits not normalized: %+v", current)
	}

	persisted, err := config.Load(path)
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}
	if persisted.Guard.ActionOnBreach != "alert" || persisted.Guard.DailyCostLimitUSD != 0 {
		t.Fatalf("persisted guard = %+v", persisted.Guard)
	}
	if !persisted.Limits.RateLimit.Enabled || persisted.Limits.RateLimit.RPS != 2 {
		t.Fatalf("legacy rate limit block was not preserved: %+v", persisted.Limits.RateLimit)
	}
}

func TestNewDashboardRoutesExposeLightweightContracts(t *testing.T) {
	database, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer database.Close()
	if _, err := database.InsertEvent(domain.TokenEvent{
		Timestamp:        time.Now().UTC(),
		Provider:         "ghostcli",
		Model:            "claude-sonnet-5",
		ClientApp:        "omp",
		PromptTokens:     13,
		CompletionTokens: 1,
		TotalTokens:      14,
		CostUSD:          0.000045,
		ProjectID:        "smoke",
		Tags:             "prod",
		Source:           "proxy",
	}); err != nil {
		t.Fatalf("insert event: %v", err)
	}
	if err := database.InsertGuardEvent(db.GuardEvent{
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		Status:    "breached",
		Message:   "daily cost limit breached",
		Action:    "block",
	}); err != nil {
		t.Fatalf("insert guard event: %v", err)
	}
	router := DashboardRouter(database,
		func(string) domain.GuardCheckResult {
			return domain.GuardCheckResult{Status: "breached", Allowed: false, Message: "daily cost limit breached"}
		},
		func() config.GuardConfig { return config.GuardConfig{ActionOnBreach: "block"} },
		func(config.GuardConfig) {},
		"",
	)

	statusRec := httptest.NewRecorder()
	router.ServeHTTP(statusRec, httptest.NewRequest(http.MethodGet, "/guard/status", nil))
	if statusRec.Code != http.StatusOK {
		t.Fatalf("guard status = %d: %s", statusRec.Code, statusRec.Body.String())
	}
	var status struct {
		Status  string          `json:"status"`
		History []db.GuardEvent `json:"history"`
	}
	if err := json.NewDecoder(statusRec.Body).Decode(&status); err != nil {
		t.Fatalf("decode guard status: %v", err)
	}
	if status.Status != "breached" || len(status.History) != 1 {
		t.Fatalf("guard status = %+v", status)
	}

	filterRec := httptest.NewRecorder()
	router.ServeHTTP(filterRec, httptest.NewRequest(http.MethodGet, "/filters", nil))
	if filterRec.Code != http.StatusOK {
		t.Fatalf("filters = %d: %s", filterRec.Code, filterRec.Body.String())
	}
	var filters map[string]interface{}
	if err := json.NewDecoder(filterRec.Body).Decode(&filters); err != nil {
		t.Fatalf("decode filters: %v", err)
	}
	for _, key := range []string{"by_project", "by_source", "by_tag"} {
		if filters[key] == nil {
			t.Fatalf("filters.%s must be an array, got null", key)
		}
	}

	csvRec := httptest.NewRecorder()
	router.ServeHTTP(csvRec, httptest.NewRequest(http.MethodGet, "/stats.csv", nil))
	if csvRec.Code != http.StatusOK || !strings.HasPrefix(csvRec.Body.String(), "requests,total_tokens") {
		t.Fatalf("csv = %d: %s", csvRec.Code, csvRec.Body.String())
	}
	svgRec := httptest.NewRecorder()
	router.ServeHTTP(svgRec, httptest.NewRequest(http.MethodGet, "/stats.svg", nil))
	if svgRec.Code != http.StatusOK || !strings.Contains(svgRec.Body.String(), "<svg") {
		t.Fatalf("svg = %d: %s", svgRec.Code, svgRec.Body.String())
	}
}
