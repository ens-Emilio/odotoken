package db

import (
	"path/filepath"
	"testing"
)

// TestTableCountsRequiresSchema proves doctor's schema check fails loudly on an
// empty file instead of reporting a healthy database.
func TestTableCountsRequiresSchema(t *testing.T) {
	database, err := Open(filepath.Join(t.TempDir(), "doctor.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer database.Close()

	counts, err := database.TableCounts()
	if err != nil {
		t.Fatalf("table counts: %v", err)
	}
	if counts.TokenEvents != 0 || counts.AlertPending != 0 {
		t.Fatalf("counts on a fresh database = %+v, want zeroes", counts)
	}
}
