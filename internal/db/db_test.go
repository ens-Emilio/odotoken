package db

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/ens-Emilio/odotoken/internal/domain"
)

// TestThroughputNeverExceedsPlausibleRate pins the one invariant every
// throughput surface shares: no derived rate may exceed maxPlausibleTPS,
// whatever the recorded timing looks like. A relay that replays a cached body,
// or an import whose streaming_ms is a sampling artifact, must not turn the
// dashboard into a fictitious number.
func TestThroughputNeverExceedsPlausibleRate(t *testing.T) {
	database, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer database.Close()

	now := time.Now().UTC()
	cases := []struct {
		name    string
		event   domain.TokenEvent
		wantMax float64
	}{
		{
			name: "streaming_ms artifact (30k tokens in 2ms)",
			event: domain.TokenEvent{
				PromptTokens: 100, CompletionTokens: 30243, TotalTokens: 30343,
				LatencyMs: 8974, StreamingMs: 2,
			},
			wantMax: maxPlausibleTPS,
		},
		{
			name: "sync completion with tiny latency",
			event: domain.TokenEvent{
				PromptTokens: 100, CompletionTokens: 1479, TotalTokens: 1579,
				LatencyMs: 270, StreamingMs: 0,
			},
			wantMax: maxPlausibleTPS,
		},
		{
			name: "honest streaming response",
			event: domain.TokenEvent{
				PromptTokens: 100, CompletionTokens: 500, TotalTokens: 600,
				LatencyMs: 4000, StreamingMs: 2000,
			},
			wantMax: maxPlausibleTPS,
		},
	}
	for _, c := range cases {
		c.event.Timestamp = now
		c.event.Source = "proxy"
		if _, err := database.InsertEvent(c.event); err != nil {
			t.Fatalf("%s: insert: %v", c.name, err)
		}
	}

	summary, err := database.GetTpsSummary("", "", "")
	if err != nil {
		t.Fatalf("summary: %v", err)
	}
	if summary.AvgTPS > maxPlausibleTPS {
		t.Errorf("avg_tps = %.1f, want <= %.0f", summary.AvgTPS, maxPlausibleTPS)
	}
	if summary.PeakTPS > maxPlausibleTPS {
		t.Errorf("peak_tps = %.1f, want <= %.0f", summary.PeakTPS, maxPlausibleTPS)
	}

	live, err := database.GetLiveTPS(3600)
	if err != nil {
		t.Fatalf("live: %v", err)
	}
	if live > maxPlausibleTPS {
		t.Errorf("live_tps = %.1f, want <= %.0f", live, maxPlausibleTPS)
	}
}

// TestLiveTPSIsZeroWithoutRecentTraffic pins the second half of the bug: live
// throughput must describe the last N seconds, not repeat a stale average from
// an older bucket while the machine is idle.
func TestLiveTPSIsZeroWithoutRecentTraffic(t *testing.T) {
	database, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer database.Close()

	// An event five minutes old: still inside the 60s/1h reporting windows the
	// dashboard uses for averages, but outside the live window. A live card
	// that repeats this number while the machine is idle is the bug.
	if _, err := database.InsertEvent(domain.TokenEvent{
		Timestamp:        time.Now().UTC().Add(-5 * time.Minute),
		Source:           "proxy",
		CompletionTokens: 900,
		TotalTokens:      900,
		LatencyMs:        3000,
		StreamingMs:      3000,
	}); err != nil {
		t.Fatalf("insert: %v", err)
	}

	live, err := database.GetLiveTPS(10)
	if err != nil {
		t.Fatalf("live: %v", err)
	}
	if live != 0 {
		t.Errorf("live_tps = %.2f, want 0", live)
	}

	recent, err := database.GetLiveTPS(3600)
	if err != nil {
		t.Fatalf("live: %v", err)
	}
	if recent <= 0 {
		t.Errorf("live_tps over 1h = %.2f, want > 0", recent)
	}
}

// TestDailyUsageRespectsDaylightSaving covers the bug a fixed-offset
// implementation cannot see: events on opposite sides of a DST transition must
// stay on their correct local dates.
func TestDailyUsageRespectsDaylightSaving(t *testing.T) {
	database, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer database.Close()

	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skipf("timezone database unavailable: %v", err)
	}
	// 04:30 UTC is 00:30 EDT on Nov 1, 2026; one hour later the clock falls
	// back to 00:30 EST. Both events belong to Nov 1 locally.
	first := time.Date(2026, time.November, 1, 4, 30, 0, 0, time.UTC)
	second := first.Add(time.Hour)
	for _, ts := range []time.Time{first, second} {
		if _, err := database.InsertEvent(domain.TokenEvent{
			Timestamp: ts, Source: "proxy", Provider: "test", Model: "m",
			TotalTokens: 1, CostUSD: 1,
		}); err != nil {
			t.Fatalf("insert: %v", err)
		}
	}

	stats, err := database.GetDailyUsage(2, first.Format(time.RFC3339Nano), "", loc.String())
	if err != nil {
		t.Fatalf("daily: %v", err)
	}
	var nov1 int
	for _, row := range stats {
		if row.Date == "2026-11-01" {
			nov1 = row.TotalRequests
		}
	}
	if nov1 != 2 {
		t.Fatalf("Nov 1 requests = %d, want 2 across the DST fall-back", nov1)
	}
}
