package importers

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ens-Emilio/odotoken/internal/domain"
	"github.com/ens-Emilio/odotoken/internal/pricing"
)

// recordingSink captures what the engine would write, so tests exercise
// pricing, batching and reporting without a database.
type recordingSink struct {
	batches [][]domain.TokenEvent
}

func (s *recordingSink) WriteEvents(events []domain.TokenEvent) (int, int, error) {
	batch := make([]domain.TokenEvent, len(events))
	copy(batch, events)
	s.batches = append(s.batches, batch)
	return len(events), 0, nil
}

func (s *recordingSink) written() int {
	n := 0
	for _, b := range s.batches {
		n += len(b)
	}
	return n
}

// fakeImporter replays a fixed set of usages, so the engine can be tested
// without any CLI session files on the machine.
type fakeImporter struct {
	name   string
	usages []domain.RawUsage
	detect bool
}

func (f *fakeImporter) Name() string { return f.name }
func (f *fakeImporter) Detect() bool { return f.detect }

func (f *fakeImporter) Scan(time.Time) domain.ScanResult {
	return domain.ScanResult{CLI: f.name, Available: f.detect, Path: "/fake"}
}

func (f *fakeImporter) IterUsages(_ time.Time, out chan<- domain.RawUsage) {
	defer close(out)
	for _, u := range f.usages {
		out <- u
	}
}

func newTestEngine(f Importer, sink Sink, batch int) *Engine {
	return &Engine{
		Registry:  &Registry{importers: []Importer{f}},
		Pricing:   pricing.NewCalculator(""),
		Sink:      sink,
		BatchSize: batch,
	}
}

// TestEnginePricesFromCatalogNotProvider covers the precedence rule: the
// catalog wins when it knows the model, because a CLI's own cost field is
// frequently absent or stale.
func TestEnginePricesFromCatalogNotProvider(t *testing.T) {
	e := &Engine{Pricing: pricing.NewCalculator("")}

	event, kind := e.toEvent(domain.RawUsage{
		Timestamp: "2026-01-02T03:04:05Z",
		Model:     "claude-3-5-sonnet",
		RawCost:   99,
	})
	if kind != priceCatalog {
		t.Fatalf("kind = %v, want priceCatalog", kind)
	}
	if event.CostUSD >= 99 {
		t.Fatalf("cost = %v, want the catalog price, not the provider's 99", event.CostUSD)
	}
	if !strings.Contains(event.Metadata, `"cost_source":"catalog"`) {
		t.Fatalf("metadata %q must record where the cost came from", event.Metadata)
	}
}

// TestEngineFallsBackToProviderCost covers a model the catalog does not know.
func TestEngineFallsBackToProviderCost(t *testing.T) {
	e := &Engine{Pricing: pricing.NewCalculator("")}

	event, kind := e.toEvent(domain.RawUsage{
		Timestamp: "2026-01-02T03:04:05Z",
		Model:     "totally-unknown-model-xyz",
		RawCost:   1.25,
	})
	if kind != priceProvider {
		t.Fatalf("kind = %v, want priceProvider", kind)
	}
	if event.CostUSD != 1.25 {
		t.Fatalf("cost = %v, want 1.25", event.CostUSD)
	}
}

// TestEngineMarksUnpriced keeps a $0 row honest instead of passing it off as a
// measured zero.
func TestEngineMarksUnpriced(t *testing.T) {
	e := &Engine{Pricing: pricing.NewCalculator("")}

	event, kind := e.toEvent(domain.RawUsage{
		Timestamp: "2026-01-02T03:04:05Z",
		Model:     "totally-unknown-model-xyz",
	})
	if kind != priceMissing {
		t.Fatalf("kind = %v, want priceMissing", kind)
	}
	if event.CostUSD != 0 {
		t.Fatalf("cost = %v, want 0", event.CostUSD)
	}
	if !strings.Contains(event.Metadata, "price_missing") {
		t.Fatalf("metadata %q must mark the row as unpriced", event.Metadata)
	}
}

// TestNormalizeTimestamp covers the regression that matters most: an
// unparseable timestamp used to become time.Now(), which moved months of
// history into today's budget window.
func TestNormalizeTimestamp(t *testing.T) {
	ts, assumed := normalizeTimestamp("not-a-timestamp", "")
	if !assumed {
		t.Fatal("an unparseable timestamp must be marked as assumed")
	}
	if !ts.IsZero() {
		t.Fatalf("no file to fall back on: got %v, want the zero time", ts)
	}

	ts, assumed = normalizeTimestamp("2026-05-06T07:08:09Z", "")
	if assumed {
		t.Fatal("a valid RFC3339 timestamp is not assumed")
	}
	if ts.Year() != 2026 || ts.Month() != 5 {
		t.Fatalf("parsed %v, want 2026-05", ts)
	}
}

// TestEngineBatchesAndReports proves the engine reports what it wrote, batches
// by BatchSize, and keeps the per-CLI breakdown the UI shows.
func TestEngineBatchesAndReports(t *testing.T) {
	sink := &recordingSink{}
	f := &fakeImporter{
		name:   "fake",
		detect: true,
		usages: []domain.RawUsage{
			{Timestamp: "2026-01-01T00:00:00Z", Model: "claude-3-5-sonnet", PromptTokens: 10},
			{Timestamp: "2026-01-01T00:00:01Z", Model: "claude-3-5-sonnet", PromptTokens: 10},
			{Timestamp: "2026-01-01T00:00:02Z", Model: "claude-3-5-sonnet", PromptTokens: 10},
		},
	}
	e := newTestEngine(f, sink, 2)

	report, err := e.Run(context.Background(), Options{}, nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if report.Inserted != 3 {
		t.Fatalf("inserted = %d, want 3", report.Inserted)
	}
	if sink.written() != 3 {
		t.Fatalf("sink saw %d events, want 3", sink.written())
	}
	if len(sink.batches) != 2 {
		t.Fatalf("batches = %d, want 2 (2 + 1)", len(sink.batches))
	}
	if report.CLIs[0].PricedByCatalog != 3 {
		t.Fatalf("priced_by_catalog = %d, want 3", report.CLIs[0].PricedByCatalog)
	}
	if len(report.CLIs) != 1 || report.CLIs[0].CLI != "fake" {
		t.Fatalf("report.clis = %+v, want one entry for fake", report.CLIs)
	}
}

// TestEngineDryRunWritesNothing is the promise the preview button makes.
func TestEngineDryRunWritesNothing(t *testing.T) {
	sink := &recordingSink{}
	f := &fakeImporter{
		name:   "fake",
		detect: true,
		usages: []domain.RawUsage{
			{Timestamp: "2026-01-01T00:00:00Z", Model: "claude-3-5-sonnet", PromptTokens: 5},
		},
	}
	e := newTestEngine(f, sink, 500)

	report, err := e.Run(context.Background(), Options{DryRun: true}, nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if sink.written() != 0 {
		t.Fatalf("dry run wrote %d events, want 0", sink.written())
	}
	if report.Inserted != 1 {
		t.Fatalf("dry run should still report what it would insert, got %d", report.Inserted)
	}
}

// TestEngineReportsUndetectedCLI keeps a missing CLI in the report instead of
// dropping it, so the UI can explain the absence.
func TestEngineReportsUndetectedCLI(t *testing.T) {
	e := newTestEngine(&fakeImporter{name: "gone", detect: false}, &recordingSink{}, 500)

	report, err := e.Run(context.Background(), Options{}, nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(report.CLIs) != 1 {
		t.Fatalf("report.clis = %d entries, want the undetected CLI reported", len(report.CLIs))
	}
	if report.CLIs[0].Error == "" {
		t.Fatal("an undetected CLI must carry a reason")
	}
}

// TestPathsResolveHonorsConfig covers sessions.paths, which used to be
// documented and ignored.
func TestPathsResolveHonorsConfig(t *testing.T) {
	p := Paths{"claude": "/moved/claude", "pi": "~/pi-data"}
	if got := p.Resolve("claude", "/default"); got != "/moved/claude" {
		t.Fatalf("claude = %q, want the configured path", got)
	}
	if got := p.Resolve("codex", "/default"); got != "/default" {
		t.Fatalf("codex = %q, want the default", got)
	}
	if got := p.Resolve("pi", "/default"); !filepath.IsAbs(got) || filepath.Base(got) != "pi-data" {
		t.Fatalf("pi = %q, want the expanded ~/pi-data", got)
	}
}

type cancelAfterFirstBatchSink struct {
	cancel context.CancelFunc
	writes int
}

func (s *cancelAfterFirstBatchSink) WriteEvents(events []domain.TokenEvent) (int, int, error) {
	s.writes++
	s.cancel()
	return len(events), 0, nil
}

func TestEngineCancellationReportsPartialWrites(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	imp := &fakeImporter{name: "fake", detect: true, usages: []domain.RawUsage{
		{Timestamp: "2026-01-01T00:00:00Z", Model: "missing-model", PromptTokens: 3, SessionFile: "session.jsonl", EntryID: "1"},
		{Timestamp: "2026-01-01T00:01:00Z", Model: "missing-model", PromptTokens: 5, SessionFile: "session.jsonl", EntryID: "2"},
	}}
	sink := &cancelAfterFirstBatchSink{cancel: cancel}
	engine := newTestEngine(imp, sink, 1)

	report, err := engine.Run(ctx, Options{CLIs: []string{"fake"}}, nil)
	if err != nil {
		t.Fatalf("Run returned %v; a cancelled run is a report state, not a failure", err)
	}
	if report.Status != "cancelled" || report.Inserted != 1 || report.CLIs[0].Scanned != 1 || !report.CLIs[0].Cancelled {
		t.Fatalf("cancelled report = %+v; want one persisted row and cancelled status", report)
	}
	if status := ReportStatus(report, false); status != "cancelled" {
		t.Fatalf("ReportStatus = %q, want cancelled", status)
	}
}
