package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ens-Emilio/odotoken/internal/analytics"
	"github.com/ens-Emilio/odotoken/internal/db"
	"github.com/ens-Emilio/odotoken/internal/domain"
)

// runCLI drives the real command tree, so a flag that is declared but not wired
// into RunE fails here instead of in a user's terminal.
func runCLI(t *testing.T, args ...string) (string, error) {
	t.Helper()
	root := newRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(args)
	err := root.Execute()
	return out.String(), err
}

func seedExportDB(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "export.db")
	database, err := db.Open(path)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer database.Close()

	base := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	for i, event := range []domain.TokenEvent{
		{
			Timestamp: base, Provider: "openai", Model: "gpt-test", ClientApp: "cli",
			ProjectID: "acme", Source: "proxy", SessionID: "sess-1", RunID: "run-1",
			PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15, CostUSD: 0.25,
			StatusCode: 200, Tags: "night,ops",
		},
		{
			Timestamp: base.Add(time.Hour), Provider: "anthropic", Model: "claude-test",
			ClientApp: "cli", ProjectID: "beta", Source: "proxy",
			PromptTokens: 7, CompletionTokens: 3, TotalTokens: 10, CostUSD: 0.5,
			StatusCode: 200,
		},
	} {
		if _, err := database.InsertEvent(event); err != nil {
			t.Fatalf("insert event %d: %v", i, err)
		}
	}
	return path
}

// The export must carry the same header and attribution the dashboard CSV has:
// a column added to analytics.EventColumns without reaching this path is a
// silent data loss for anyone exporting from a cron job.
func TestExportWritesCSVHeaderAndRows(t *testing.T) {
	dbPath := seedExportDB(t)

	out, err := runCLI(t, "export", "--db", dbPath)
	if err != nil {
		t.Fatalf("export: %v\n%s", err, out)
	}

	records, err := csv.NewReader(strings.NewReader(out)).ReadAll()
	if err != nil {
		t.Fatalf("parse csv: %v\n%s", err, out)
	}
	if len(records) != 3 {
		t.Fatalf("rows = %d, want header + 2 events\n%s", len(records), out)
	}
	if strings.Join(records[0], ",") != strings.Join(analytics.EventColumns, ",") {
		t.Fatalf("header = %v, want %v", records[0], analytics.EventColumns)
	}

	// Newest first, so the anthropic event is the first data row.
	row := map[string]string{}
	for i, column := range records[0] {
		row[column] = records[1][i]
	}
	if row["model"] != "claude-test" || row["project_id"] != "beta" {
		t.Fatalf("first row = %v", row)
	}
	last := map[string]string{}
	for i, column := range records[0] {
		last[column] = records[2][i]
	}
	if last["run_id"] != "run-1" || last["session_id"] != "sess-1" {
		t.Fatalf("run/session attribution lost: %v", last)
	}
	if last["tags"] != "night,ops" {
		t.Fatalf("tags = %q, want the comma preserved by quoting", last["tags"])
	}
}

func TestExportWritesJSONArrayAndFiltersProject(t *testing.T) {
	dbPath := seedExportDB(t)

	out, err := runCLI(t, "export", "--db", dbPath, "--format", "json", "--project", "acme")
	if err != nil {
		t.Fatalf("export: %v\n%s", err, out)
	}
	var rows []map[string]interface{}
	if err := json.Unmarshal([]byte(out), &rows); err != nil {
		t.Fatalf("output is not a JSON array: %v\n%s", err, out)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want only the acme event\n%s", len(rows), out)
	}
	if rows[0]["project_id"] != "acme" {
		t.Fatalf("project filter ignored: %v", rows[0])
	}

	// A range with nothing in it must still be a valid empty array, not `null`.
	out, err = runCLI(t, "export", "--db", dbPath, "--format", "json",
		"--since", "2030-01-01T00:00:00Z")
	if err != nil {
		t.Fatalf("export empty range: %v\n%s", err, out)
	}
	if strings.TrimSpace(out) != "[]" {
		t.Fatalf("empty export = %q, want []", strings.TrimSpace(out))
	}
}

// --json with --out keeps stdout machine-readable: the rows go to the file and
// the summary reports how many landed.
func TestExportOutFileWithJSONSummary(t *testing.T) {
	dbPath := seedExportDB(t)
	target := filepath.Join(t.TempDir(), "events.csv")

	out, err := runCLI(t, "export", "--db", dbPath, "--out", target, "--json")
	if err != nil {
		t.Fatalf("export: %v\n%s", err, out)
	}
	var meta exportMeta
	if err := json.Unmarshal([]byte(out), &meta); err != nil {
		t.Fatalf("summary is not JSON: %v\n%s", err, out)
	}
	if meta.Rows != 2 || meta.Format != "csv" || meta.Path != target {
		t.Fatalf("summary = %+v", meta)
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read export: %v", err)
	}
	if !strings.HasPrefix(string(data), strings.Join(analytics.EventColumns, ",")) {
		t.Fatalf("file does not start with the CSV header: %q", string(data))
	}
}

func TestExportRejectsInvertedRange(t *testing.T) {
	dbPath := seedExportDB(t)
	out, err := runCLI(t, "export", "--db", dbPath,
		"--since", "2026-02-01T00:00:00Z", "--until", "2026-01-01T00:00:00Z")
	if err == nil {
		t.Fatalf("inverted range accepted:\n%s", out)
	}
	if !strings.Contains(err.Error(), "after --until") {
		t.Fatalf("error = %v", err)
	}
}

// set -> list -> delete must be visible through a fresh Store each time, which
// is what a user does: three separate invocations.
func TestPricesSetListDeleteRoundTrip(t *testing.T) {
	pricesPath := filepath.Join(t.TempDir(), "prices.json")

	if out, err := runCLI(t, "prices", "--prices", pricesPath,
		"set", "--model", "my-model", "--input", "1.5", "--output", "6",
		"--cache-read", "0.15", "--cache-write", "2"); err != nil {
		t.Fatalf("set: %v\n%s", err, out)
	}

	out, err := runCLI(t, "prices", "--prices", pricesPath, "list", "--json", "--query", "my-model")
	if err != nil {
		t.Fatalf("list: %v\n%s", err, out)
	}
	var rows []struct {
		Model      string  `json:"model"`
		Input      float64 `json:"input"`
		Output     float64 `json:"output"`
		CacheRead  float64 `json:"cache_read"`
		CacheWrite float64 `json:"cache_write"`
		Explicit   bool    `json:"explicit"`
	}
	if err := json.Unmarshal([]byte(out), &rows); err != nil {
		t.Fatalf("list output is not JSON: %v\n%s", err, out)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want the override\n%s", len(rows), out)
	}
	if got := rows[0]; got.Model != "my-model" || !got.Explicit ||
		got.Input != 1.5 || got.Output != 6 || got.CacheRead != 0.15 || got.CacheWrite != 2 {
		t.Fatalf("row = %+v", got)
	}

	// The human table marks the source, so an override is distinguishable from
	// a built-in rate without --json.
	table, err := runCLI(t, "prices", "--prices", pricesPath, "list", "--query", "my-model")
	if err != nil {
		t.Fatalf("list table: %v\n%s", err, table)
	}
	if !strings.Contains(table, "override") || !strings.Contains(table, "MODEL") {
		t.Fatalf("table missing source column:\n%s", table)
	}

	if out, err := runCLI(t, "prices", "--prices", pricesPath, "delete", "--model", "my-model"); err != nil {
		t.Fatalf("delete: %v\n%s", err, out)
	}
	out, err = runCLI(t, "prices", "--prices", pricesPath, "list", "--json", "--query", "my-model")
	if err != nil {
		t.Fatalf("list after delete: %v\n%s", err, out)
	}
	if strings.TrimSpace(out) != "[]" {
		t.Fatalf("override survived delete: %s", out)
	}

	data, err := os.ReadFile(pricesPath)
	if err != nil {
		t.Fatalf("read catalog: %v", err)
	}
	if strings.Contains(string(data), "my-model") {
		t.Fatalf("deleted override still on disk: %s", data)
	}
}

// A built-in rate has nothing to fall back to: deleting it would price the
// model at $0 forever.
func TestPricesDeleteRefusesBuiltin(t *testing.T) {
	pricesPath := filepath.Join(t.TempDir(), "prices.json")
	out, err := runCLI(t, "prices", "--prices", pricesPath, "delete", "--model", "gpt-4o")
	if err == nil {
		t.Fatalf("builtin delete accepted:\n%s", out)
	}
	if !strings.Contains(err.Error(), "built-in") {
		t.Fatalf("error = %v, want it to name the built-in rate", err)
	}
}
