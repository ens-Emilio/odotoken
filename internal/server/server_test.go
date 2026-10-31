package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ens-Emilio/odotoken/internal/config"
	"github.com/ens-Emilio/odotoken/internal/db"
	"github.com/ens-Emilio/odotoken/internal/domain"
)

// The assembled router is what the daemon actually serves, so a missing
// Router assignment or an unmounted /metrics turns into 404s for every
// endpoint. This walks the mounts the operator relies on.
func TestAssembledRouterServesMetricsAndAPI(t *testing.T) {
	database, err := db.Open(filepath.Join(t.TempDir(), "routes.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer database.Close()

	var cfg config.Config
	cfg.Server.APIKey = "test-key"
	cfgPath := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(cfgPath, []byte(`{}`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	srv := NewWithConfigPath(database, cfg, slog.New(slog.NewTextHandler(os.Stderr, nil)), cfgPath)

	for path, want := range map[string]int{
		"/metrics":           http.StatusOK,
		"/api/events":        http.StatusOK,
		"/api/guard/limits":  http.StatusOK,
		"/api/cache-savings": http.StatusOK,
	} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("X-Api-Key", "test-key")
		srv.Router.ServeHTTP(rec, req)
		if rec.Code != want {
			t.Errorf("%s = %d, want %d", path, rec.Code, want)
		}
	}
}

// /health is odotoken's own liveness, so it must never reach the proxy: a probe
// that is forwarded is metered, spends the rate-limit burst, and starts
// answering 429 once a budget is breached — which is when it matters most.
func TestHealthIsLocalLivenessNotProxied(t *testing.T) {
	var upstreamHits atomic.Int64
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamHits.Add(1)
		w.WriteHeader(http.StatusTeapot)
		_, _ = w.Write([]byte(`{"proxied":true}`))
	}))
	defer upstream.Close()

	database, err := db.Open(filepath.Join(t.TempDir(), "health.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer database.Close()
	// A breached budget would block the proxy path; liveness must ignore it.
	if _, err := database.InsertEvent(domain.TokenEvent{
		Timestamp:   time.Now().UTC(),
		Provider:    "openai",
		Model:       "gpt-test",
		TotalTokens: 1000,
		StatusCode:  200,
		ProjectID:   "default",
		Source:      "proxy",
	}); err != nil {
		t.Fatalf("insert event: %v", err)
	}

	var cfg config.Config
	cfg.Server.APIKey = "test-key"
	cfg.Providers = map[string]config.UpstreamConfig{"openai": {UpstreamURL: upstream.URL}}
	cfg.Guard.DailyTokenLimit = 1
	cfg.Guard.ActionOnBreach = "block"
	cfg.Limits.RateLimit.Enabled = true
	cfg.Limits.RateLimit.RPS = 1
	cfg.Limits.RateLimit.BurstSize = 1
	cfgPath := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(cfgPath, []byte(`{}`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	srv := NewWithConfigPath(database, cfg, slog.New(slog.NewTextHandler(os.Stderr, nil)), cfgPath)

	// No X-Api-Key and repeated calls on purpose: liveness needs no secret and
	// must not drain the limiter.
	for i := range 3 {
		rec := httptest.NewRecorder()
		srv.Router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("call %d: /health = %d, want 200: %s", i, rec.Code, rec.Body.String())
		}
		if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
			t.Fatalf("call %d: Content-Type = %q, want application/json", i, ct)
		}
		var body map[string]string
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("call %d: decode %q: %v", i, rec.Body.String(), err)
		}
		if body["status"] != "ok" {
			t.Fatalf("call %d: body = %v, want status ok", i, body)
		}
	}

	if hits := upstreamHits.Load(); hits != 0 {
		t.Fatalf("/health reached the upstream %d time(s); it must not be proxied", hits)
	}
	// One event is the fixture above; a metered probe would add more.
	events, err := database.RecentEventsFiltered(50, "", "", "", "")
	if err != nil {
		t.Fatalf("recent events: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("events = %d, want only the fixture: /health must not be metered", len(events))
	}
}

// A `?lang=xx` link is how a shared dashboard opens in a specific language, so
// the root redirect must not drop the query on the floor.
func TestRootRedirectKeepsQuery(t *testing.T) {
	cases := []struct {
		name   string
		target string
		want   string
	}{
		{"no query", "/", "/app/"},
		{"language", "/?lang=ar", "/app/?lang=ar"},
		{"language and view", "/?lang=de&view=guard", "/app/?lang=de&view=guard"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, tc.target, nil)
			http.Redirect(rec, req, "/app/"+querySuffix(req), http.StatusTemporaryRedirect)
			if got := rec.Header().Get("Location"); got != tc.want {
				t.Fatalf("Location = %q, want %q", got, tc.want)
			}
		})
	}
}
