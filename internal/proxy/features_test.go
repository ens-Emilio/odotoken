package proxy

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/ens-Emilio/odotoken/internal/config"
	"github.com/ens-Emilio/odotoken/internal/db"
	"github.com/ens-Emilio/odotoken/internal/parser"
	"github.com/ens-Emilio/odotoken/internal/pricing"
)

func testProxy(t *testing.T, cfg config.Config) *Proxy {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "proxy.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return New(database, pricing.NewCalculator(""), cfg, logger)
}

// TestFallbackRetriesTransientStatus pins the fallback contract: a 429 from the
// primary is retried against the configured provider, and the client sees the
// fallback's answer as if nothing happened.
func TestProxyFallbackConfigured(t *testing.T) {
	var fallbackHits atomic.Int32
	primary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer primary.Close()
	fallback := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fallbackHits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"usage":{"prompt_tokens":1,"completion_tokens":2}}`))
	}))
	defer fallback.Close()

	cfg := config.Config{Providers: map[string]config.UpstreamConfig{
		"primary": {UpstreamURL: primary.URL, Fallbacks: []string{"backup"}},
		"backup":  {UpstreamURL: fallback.URL},
	}}
	p := testProxy(t, cfg)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/primary/v1/chat", strings.NewReader(`{"model":"m"}`))
	p.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if fallbackHits.Load() != 1 {
		t.Fatalf("fallback hits = %d, want 1", fallbackHits.Load())
	}
}

// TestFallbackIgnoredForClientErrors keeps 4xx client mistakes on the primary:
// retrying them only multiplies the upstream load.
func TestFallbackIgnoredForClientErrors(t *testing.T) {
	var fallbackHits atomic.Int32
	primary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer primary.Close()
	fallback := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fallbackHits.Add(1)
	}))
	defer fallback.Close()

	p := testProxy(t, config.Config{Providers: map[string]config.UpstreamConfig{
		"primary": {UpstreamURL: primary.URL, Fallbacks: []string{"backup"}},
		"backup":  {UpstreamURL: fallback.URL},
	}})

	rec := httptest.NewRecorder()
	p.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/primary/v1/chat", strings.NewReader(`{}`)))
	if rec.Code != http.StatusBadRequest || fallbackHits.Load() != 0 {
		t.Fatalf("status=%d fallback hits=%d, want 400/0", rec.Code, fallbackHits.Load())
	}
}

// TestRateLimitIsolatesProjects proves a throttled project does not consume
// another project's budget.
func TestRateLimitIsolatesProjects(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	p := testProxy(t, config.Config{
		Providers: map[string]config.UpstreamConfig{"openai": {UpstreamURL: upstream.URL}},
		Limits: config.LimitsConfig{RateLimit: config.RateLimitConfig{
			Enabled: true, RPS: 100, BurstSize: 100,
			Projects: map[string]config.RateLimitConfig{"batch": {RPS: 0.001, BurstSize: 1}},
		}},
	})

	send := func(project string) int {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/openai/v1/chat", strings.NewReader(`{}`))
		req.Header.Set("X-Odo-Project", project)
		p.Handler().ServeHTTP(rec, req)
		return rec.Code
	}
	if code := send("batch"); code != http.StatusOK {
		t.Fatalf("first batch request = %d, want 200", code)
	}
	if code := send("batch"); code != http.StatusTooManyRequests {
		t.Fatalf("second batch request = %d, want 429", code)
	}
	if code := send("other"); code != http.StatusOK {
		t.Fatalf("other project = %d, want 200", code)
	}
}

// TestStreamCutOnBudgetBreach checks a breached budget ends an in-flight SSE
// stream with one parseable error frame rather than a truncated connection.
func TestProxyStreamBudgetCut(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)
		for i := 0; i < 5; i++ {
			_, _ = w.Write([]byte("data: {\"delta\":\"x\"}\n\n"))
			if flusher != nil {
				flusher.Flush()
			}
		}
	}))
	defer upstream.Close()

	p := testProxy(t, config.Config{Providers: map[string]config.UpstreamConfig{"openai": {UpstreamURL: upstream.URL}}})
	counting := newCountingReadCloser(io.NopCloser(strings.NewReader(
		"data: {\"delta\":\"x\"}\n\ndata: {\"delta\":\"y\"}\n\n")),
		parser.NewStreamAccumulator("openai", ""))
	counting.budgetExceeded = func() string { return "daily cost limit breached" }
	counting.lastBudget.Store(1) // skip the 2s throttle on the first read

	body, err := io.ReadAll(counting)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(string(body), "budget_exceeded") {
		t.Fatalf("stream did not end with a budget error frame: %q", body)
	}
	if !counting.cutShort.Load() {
		t.Fatal("cutShort not recorded, active run would report ok")
	}
	_ = p
}
