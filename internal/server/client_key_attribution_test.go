package server

import (
	"io"
	"log/slog"
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

const (
	breachedKey = "odt_breached_key_0001"
	healthyKey  = "odt_healthy_key_0002"
)

func newClientKeyServer(t *testing.T) (*Server, *db.Database) {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "keys.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"model":"gpt-4o","usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}`)
	}))
	t.Cleanup(upstream.Close)

	var cfg config.Config
	cfg.Server.ClientKeys = map[string]config.ClientKeyConfig{
		breachedKey: {Client: "breached-client", Project: "breached-project", DailyCostLimitUSD: 0.000001},
		healthyKey:  {Client: "healthy-client", Project: "healthy-project"},
	}
	cfg.Providers = map[string]config.UpstreamConfig{"openai": {UpstreamURL: upstream.URL}}
	cfg.Guard.ActionOnBreach = "block"

	cfgPath := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(cfgPath, []byte(`{}`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewWithConfigPath(database, cfg, logger, cfgPath), database
}

func postChat(srv *Server, headers map[string]string) int {
	req := httptest.NewRequest(http.MethodPost, "/openai/chat/completions",
		strings.NewReader(`{"model":"gpt-4o","messages":[{"role":"user","content":"hi"}]}`))
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	srv.Router.ServeHTTP(rec, req)
	return rec.Code
}

// A caller holding an over-quota key cannot escape the quota by naming a
// healthy project in X-Odo-Project: the key's configured project is authoritative.
func TestClientKeyQuotaIgnoresSpoofedProjectHeader(t *testing.T) {
	srv, database := newClientKeyServer(t)
	if _, err := database.InsertEvent(domain.TokenEvent{
		Timestamp: time.Now().UTC(), Provider: "openai", Model: "gpt-4o",
		ClientApp: "breached-client", ProjectID: "breached-project",
		PromptTokens: 1000, TotalTokens: 1000, CostUSD: 10, StatusCode: 200, Source: "proxy",
	}); err != nil {
		t.Fatalf("seed spend: %v", err)
	}

	code := postChat(srv, map[string]string{
		"Authorization": "Bearer " + breachedKey,
		"X-Odo-Project": "healthy-project",
		"X-Odo-Client":  "healthy-client",
	})
	if code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429: a key must not spend against another project", code)
	}
}

// The recorded event must carry the key's client, because that is the value the
// key quota aggregates; otherwise the key's own spend stays invisible to it.
func TestClientKeyEventCarriesKeyIdentity(t *testing.T) {
	srv, database := newClientKeyServer(t)

	code := postChat(srv, map[string]string{
		"Authorization": "Bearer " + healthyKey,
		"X-Odo-Project": "victim-project",
		"X-Odo-Client":  "victim-client",
	})
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200", code)
	}

	var recorded *domain.TokenEvent
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		events, err := database.RecentEvents(10, "")
		if err != nil {
			t.Fatalf("recent events: %v", err)
		}
		for i := range events {
			if events[i].Source == "proxy" {
				recorded = &events[i]
				break
			}
		}
		if recorded != nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if recorded == nil {
		t.Fatal("no proxy event recorded")
	}
	if recorded.ClientApp != "healthy-client" {
		t.Fatalf("ClientApp = %q, want the key's client", recorded.ClientApp)
	}
	if recorded.ProjectID != "healthy-project" {
		t.Fatalf("ProjectID = %q, want the key's project", recorded.ProjectID)
	}

	// The quota's own query must see the spend just recorded.
	summary, err := database.GetClientCostSummaryInLocation("healthy-project", "healthy-client", time.UTC)
	if err != nil {
		t.Fatalf("client summary: %v", err)
	}
	if summary.DailyTokens < 15 {
		t.Fatalf("key quota sees %d tokens, want the recorded 15", summary.DailyTokens)
	}
}
