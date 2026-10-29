package db

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/ens-Emilio/odotoken/internal/importers"
)

// TestRecordImportKeepsCancelledStatus pins the regression that let a stopped
// import appear as a completed one: the record must carry the status the
// engine decided, and the error text must survive into the history row.
func TestRecordImportKeepsCancelledStatus(t *testing.T) {
	database, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer database.Close()

	started := time.Now().UTC()
	report := importers.Report{
		StartedAt:  started,
		FinishedAt: started.Add(time.Second),
		CLIs: []importers.CLIReport{
			{CLI: "claude", Inserted: 4, Cancelled: true, Error: "context canceled"},
		},
		Inserted: 4,
		Status:   "cancelled",
	}
	if err := database.RecordImport([]string{"claude"}, report); err != nil {
		t.Fatalf("record: %v", err)
	}

	runs, err := database.RecentImportRuns(1)
	if err != nil {
		t.Fatalf("history: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("runs = %d, want 1", len(runs))
	}
	if runs[0].Status != "cancelled" || runs[0].Inserted != 4 || runs[0].Error != "context canceled" {
		t.Fatalf("recorded run = %+v; want cancelled status, 4 inserted, and the error text", runs[0])
	}
}
