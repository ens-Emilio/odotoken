package proxy

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/ens-Emilio/odotoken/internal/parser"
)

// TestFallbackRetriesTransientStatus proves a 429 from the first provider is
// retried against the configured fallback with the same body and path.
func TestFallbackRetriesTransientStatus(t *testing.T) {
	var primaryHits, fallbackHits atomic.Int32
	var gotPath, gotBody atomic.Value

	primary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		primaryHits.Add(1)
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer primary.Close()
	fallback := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fallbackHits.Add(1)
		gotPath.Store(r.URL.Path)
		body, _ := io.ReadAll(r.Body)
		gotBody.Store(string(body))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer fallback.Close()

	p := &Proxy{
		logger:         slog.New(slog.NewTextHandler(io.Discard, nil)),
		upstreams:      map[string]*url.URL{"primary": mustParseURL(primary.URL), "backup": mustParseURL(fallback.URL)},
		fallbacks:      map[string][]fallbackTarget{},
		fallbackStatus: map[string]map[int]bool{},
	}
	p.fallbacks["primary"] = []fallbackTarget{{provider: "backup", url: mustParseURL(fallback.URL)}}
	p.fallbackStatus["primary"] = fallbackStatusSet(nil)

	body := `{"model":"gpt-test","messages":[]}`
	req := httptest.NewRequest(http.MethodPost, primary.URL+"/v1/chat/completions", strings.NewReader(body))
	req = req.WithContext(withProvider(req, "primary"))
	// extractModelMiddleware buffers the body and installs GetBody; the retry
	// relies on that same contract.
	req.GetBody = func() (io.ReadCloser, error) { return io.NopCloser(strings.NewReader(body)), nil }

	resp, err := (&fallbackTransport{base: http.DefaultTransport, proxy: p}).RoundTrip(req)
	if err != nil {
		t.Fatalf("round trip: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if primaryHits.Load() != 1 || fallbackHits.Load() != 1 {
		t.Fatalf("hits primary=%d fallback=%d, want 1/1", primaryHits.Load(), fallbackHits.Load())
	}
	if gotPath.Load() != "/v1/chat/completions" {
		t.Fatalf("fallback path = %v", gotPath.Load())
	}
	if gotBody.Load() != body {
		t.Fatalf("fallback body = %v, want the original request body", gotBody.Load())
	}
}

// TestFallbackSkippedForClientErrors keeps a 400 on the primary: retrying a
// malformed request against another provider only doubles the failure.
func TestFallbackSkippedForClientErrors(t *testing.T) {
	var fallbackHits atomic.Int32
	primary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer primary.Close()
	fallback := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fallbackHits.Add(1)
	}))
	defer fallback.Close()

	p := &Proxy{
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		fallbacks: map[string][]fallbackTarget{"primary": {
			{provider: "backup", url: mustParseURL(fallback.URL)},
		}},
		fallbackStatus: map[string]map[int]bool{"primary": fallbackStatusSet(nil)},
	}
	req := httptest.NewRequest(http.MethodPost, primary.URL+"/v1/chat/completions", strings.NewReader("{}"))
	req = req.WithContext(withProvider(req, "primary"))

	resp, err := (&fallbackTransport{base: http.DefaultTransport, proxy: p}).RoundTrip(req)
	if err != nil {
		t.Fatalf("round trip: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest || fallbackHits.Load() != 0 {
		t.Fatalf("status=%d fallbackHits=%d, want 400/0", resp.StatusCode, fallbackHits.Load())
	}
}

// TestStreamCutOnBudgetBreach checks the mid-stream cutoff emits one parseable
// SSE frame and then EOF, rather than truncating the connection.
func TestStreamCutOnBudgetBreach(t *testing.T) {
	upstream := strings.NewReader("data: {\"delta\":\"one\"}\n\ndata: {\"delta\":\"two\"}\n\n")
	counting := newCountingReadCloser(io.NopCloser(upstream), parser.NewStreamAccumulator("openai", ""))
	counting.budgetExceeded = func() string { return "daily cost limit breached" }
	// Skip the two second throttle so the very first Read re-checks.
	counting.lastBudget.Store(1)

	var out strings.Builder
	buf := make([]byte, 64)
	for range 8 {
		n, err := counting.Read(buf)
		out.Write(buf[:n])
		if err == io.EOF {
			break
		}
	}
	got := out.String()
	if !strings.Contains(got, `"type":"budget_exceeded"`) {
		t.Fatalf("cut frame missing, got %q", got)
	}
	if !strings.HasSuffix(got, "\n\n") {
		t.Fatalf("cut frame must end the SSE event, got %q", got)
	}
	if !counting.cutShort.Load() {
		t.Fatal("cutShort not set, active run would be reported as ok")
	}
}

func withProvider(r *http.Request, provider string) context.Context {
	return context.WithValue(r.Context(), providerContextKey, provider)
}
