package db

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/ens-Emilio/odotoken/internal/domain"
)

// TestApplyRetentionKeepsRecentEvents pins the operator-visible contract: a
// positive day count deletes only what is older than the cutoff, zero deletes
// nothing at all (retention disabled), and VACUUM is reported only when rows
// were actually removed.
func TestApplyRetentionKeepsRecentEvents(t *testing.T) {
	database, err := Open(filepath.Join(t.TempDir(), "retention.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer database.Close()

	now := time.Now().UTC()
	old := domain.TokenEvent{Timestamp: now.AddDate(0, 0, -60), Provider: "openai", Model: "gpt-4o", TotalTokens: 10}
	recent := domain.TokenEvent{Timestamp: now.Add(-time.Hour), Provider: "openai", Model: "gpt-4o", TotalTokens: 10}
	for _, e := range []domain.TokenEvent{old, recent} {
		if _, err := database.InsertEvent(e); err != nil {
			t.Fatalf("insert: %v", err)
		}
	}

	disabled, err := database.ApplyRetention(0, true)
	if err != nil {
		t.Fatalf("retention disabled: %v", err)
	}
	if disabled.Deleted != 0 || disabled.Vacuum {
		t.Fatalf("retention 0 deleted %d rows (vacuum %v), want no-op", disabled.Deleted, disabled.Vacuum)
	}

	result, err := database.ApplyRetention(30, false)
	if err != nil {
		t.Fatalf("retention 30: %v", err)
	}
	if result.Deleted != 1 {
		t.Fatalf("deleted = %d, want 1 (only the 60-day-old event)", result.Deleted)
	}
	if result.Vacuum {
		t.Fatalf("vacuum ran without being requested")
	}

	events, err := database.RecentEvents(10, "")
	if err != nil {
		t.Fatalf("recent events: %v", err)
	}
	if len(events) != 1 || events[0].Timestamp.Unix() != recent.Timestamp.Unix() {
		t.Fatalf("remaining events = %+v, want the recent one only", events)
	}
}

// TestAlertOutboxRetriesAndDropsExhausted pins the durable outbox: a row is
// pending only after its scheduled time, a failure pushes it into the future,
// and a row that burned every attempt leaves the table.
func TestAlertOutboxRetriesAndDropsExhausted(t *testing.T) {
	database, err := Open(filepath.Join(t.TempDir(), "alerts.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer database.Close()

	now := time.Now().UTC()
	if err := database.EnqueueAlert("slack", "https://hooks.example/x", `{"text":"hi"}`, now.Add(time.Hour)); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	pending, err := database.PendingAlerts(10)
	if err != nil {
		t.Fatalf("pending: %v", err)
	}
	if len(pending) != 0 {
		t.Fatalf("future alert was already due: %+v", pending)
	}

	if err := database.EnqueueAlert("webhook", "https://example/y", `{"text":"now"}`, now); err != nil {
		t.Fatalf("enqueue due: %v", err)
	}
	pending, err = database.PendingAlerts(10)
	if err != nil {
		t.Fatalf("pending due: %v", err)
	}
	if len(pending) != 1 || pending[0].Channel != "webhook" {
		t.Fatalf("due alerts = %+v, want the webhook only", pending)
	}

	if err := database.MarkAlertFailed(pending[0].ID, 5, now, "boom"); err != nil {
		t.Fatalf("mark failed: %v", err)
	}
	dropped, err := database.DropExhaustedAlerts(5)
	if err != nil {
		t.Fatalf("drop exhausted: %v", err)
	}
	if dropped != 1 {
		t.Fatalf("dropped = %d, want the exhausted row", dropped)
	}
	count, err := database.PendingAlertCount()
	if err != nil {
		t.Fatalf("pending count: %v", err)
	}
	if count != 1 {
		t.Fatalf("pending count = %d, want the still-scheduled alert", count)
	}
}
