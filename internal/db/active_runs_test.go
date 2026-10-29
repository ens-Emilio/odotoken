package db

import (
	"path/filepath"
	"testing"
	"time"
)

func TestActiveRunLifecycleAndOrphanExpiry(t *testing.T) {
	d, err := Open(filepath.Join(t.TempDir(), "runs.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if err := d.StartActiveRun("run-a", "session-a", "omp", "relay", "claude-sonnet-5", "project"); err != nil {
		t.Fatal(err)
	}
	live, err := d.GetLiveSessions()
	if err != nil {
		t.Fatal(err)
	}
	if len(live) != 1 || live[0].RunID != "run-a" || live[0].CLI != "omp" {
		t.Fatalf("live runs = %+v", live)
	}
	if err := d.FinishActiveRun("run-a", "ok", "", 13, 1, 0.0001); err != nil {
		t.Fatal(err)
	}
	live, err = d.GetLiveSessions()
	if err != nil {
		t.Fatal(err)
	}
	if len(live) != 0 {
		t.Fatalf("finished run still live: %+v", live)
	}
	if err := d.StartActiveRun("orphan", "orphan-session", "omp", "relay", "claude-sonnet-5", "project"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.db.Exec(`UPDATE active_runs SET last_activity=? WHERE run_id='orphan'`, time.Now().UTC().Add(-11*time.Minute).Format(time.RFC3339Nano)); err != nil {
		t.Fatal(err)
	}
	if err := d.ExpireOrphanRuns(10 * time.Minute); err != nil {
		t.Fatal(err)
	}
	live, err = d.GetLiveSessions()
	if err != nil {
		t.Fatal(err)
	}
	if len(live) != 0 {
		t.Fatalf("expired run still live: %+v", live)
	}
}

func TestConcurrentRunsShareSessionID(t *testing.T) {
	d, err := Open(filepath.Join(t.TempDir(), "runs.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if err := d.StartActiveRun("run-1", "conversation", "omp", "relay", "m1", "p"); err != nil {
		t.Fatal(err)
	}
	if err := d.StartActiveRun("run-2", "conversation", "omp", "relay", "m2", "p"); err != nil {
		t.Fatal(err)
	}
	runs, err := d.GetLiveSessions()
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 2 || runs[0].SessionID != "conversation" || runs[1].SessionID != "conversation" {
		t.Fatalf("concurrent runs = %+v", runs)
	}
}
