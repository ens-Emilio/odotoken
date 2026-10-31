package guard

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ens-Emilio/odotoken/internal/config"
	"github.com/ens-Emilio/odotoken/internal/db"
	"github.com/ens-Emilio/odotoken/internal/domain"
)

// TestAlertDeliverySurvivesFailureThenRetries proves the queue is the source
// of truth: a breach is persisted before any network call, a failed attempt
// schedules a later one instead of dropping the alert, and the next pass
// delivers it and closes the row.
func TestAlertDeliverySurvivesFailureThenRetries(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hits.Add(1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	database, err := db.Open(filepath.Join(t.TempDir(), "queue.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer database.Close()

	alerts := config.AlertsConfig{
		WebhookURL: srv.URL,
		Retry:      config.AlertRetry{MaxAttempts: 3, BaseDelaySeconds: 60, MaxDelaySeconds: 600},
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	g := New(database, config.GuardConfig{}, alerts, logger)

	g.enqueueAlerts(domain.GuardCheckResult{Status: "breached", Message: "daily budget exceeded"})

	pending, err := database.PendingAlerts(10)
	if err != nil {
		t.Fatalf("pending: %v", err)
	}
	if len(pending) != 1 {
		t.Fatalf("queued alerts = %d, want the breach persisted before delivery", len(pending))
	}
	id := pending[0].ID

	if delivered := g.DeliverPending(context.Background()); delivered != 0 {
		t.Fatalf("delivered = %d on a failing channel, want 0", delivered)
	}
	pending, err = database.PendingAlerts(10)
	if err != nil {
		t.Fatalf("pending after failure: %v", err)
	}
	if len(pending) != 0 {
		t.Fatalf("failed alert is due again immediately, want a backoff delay: %+v", pending)
	}

	// Move the retry into the past instead of waiting out the 60s backoff.
	if err := database.MarkAlertFailed(id, 1, time.Now().UTC().Add(-time.Second), "retry now"); err != nil {
		t.Fatalf("reschedule: %v", err)
	}
	if delivered := g.DeliverPending(context.Background()); delivered != 1 {
		t.Fatalf("delivered = %d on a healthy channel, want 1", delivered)
	}
	count, err := database.PendingAlertCount()
	if err != nil {
		t.Fatalf("pending count: %v", err)
	}
	if count != 0 {
		t.Fatalf("pending count = %d after delivery, want 0", count)
	}
	if hits.Load() != 2 {
		t.Fatalf("channel hits = %d, want one failed and one successful attempt", hits.Load())
	}
}

func TestGenericWebhookDelivery(t *testing.T) {
	type received struct {
		method      string
		contentType string
		body        []byte
	}
	got := make(chan received, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		got <- received{method: r.Method, contentType: r.Header.Get("Content-Type"), body: body}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	database, err := db.Open(filepath.Join(t.TempDir(), "webhook.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer database.Close()
	if err := database.EnqueueAlert("webhook", srv.URL, `{"text":"budget warning"}`, time.Now().UTC()); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	g := New(database, config.GuardConfig{}, config.AlertsConfig{
		WebhookURL: srv.URL,
		Retry:      config.AlertRetry{MaxAttempts: 3, BaseDelaySeconds: 1, MaxDelaySeconds: 10},
	}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if delivered := g.DeliverPending(context.Background()); delivered != 1 {
		t.Fatalf("delivered = %d, want 1", delivered)
	}
	req := <-got
	if req.method != http.MethodPost {
		t.Errorf("method = %s, want POST", req.method)
	}
	if req.contentType != "application/json" {
		t.Errorf("content-type = %q, want application/json", req.contentType)
	}
	var payload map[string]string
	if err := json.Unmarshal(req.body, &payload); err != nil {
		t.Fatalf("decode webhook body: %v", err)
	}
	if payload["source"] != "odotoken" || payload["text"] != "budget warning" {
		t.Fatalf("payload = %+v, want source odotoken and text budget warning", payload)
	}
}
