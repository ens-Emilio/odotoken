package db

import (
	"os"
	"testing"
)

func TestAuditLogChain(t *testing.T) {
	tmp := t.TempDir() + "/audit.db"
	d, err := Open(tmp)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer os.Remove(tmp)

	if _, err := d.db.Exec(`CREATE TABLE IF NOT EXISTS audit_log (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp TEXT NOT NULL,
		actor TEXT NOT NULL,
		action TEXT NOT NULL,
		resource TEXT NOT NULL,
		hash TEXT NOT NULL
	)`); err != nil {
		t.Fatalf("create table: %v", err)
	}

	if err := d.InsertAuditLog("admin", "create", "key-1"); err != nil {
		t.Fatalf("insert 1: %v", err)
	}
	if err := d.InsertAuditLog("user", "read", "key-1"); err != nil {
		t.Fatalf("insert 2: %v", err)
	}

	entries, err := d.GetAuditLogs(10)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("want 2 entries, got %d", len(entries))
	}
	if entries[0].Hash == entries[1].Hash {
		t.Error("hashes should differ between entries")
	}
	if entries[0].Action != "read" {
		t.Errorf("newest first: got action %q", entries[0].Action)
	}
}