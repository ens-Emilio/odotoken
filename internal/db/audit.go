package db

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

// AuditEntry represents an immutable audit log record with integrity chaining.
type AuditEntry struct {
	ID        int64     `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	Actor     string    `json:"actor"`
	Action    string    `json:"action"`
	Resource  string    `json:"resource"`
	Hash      string    `json:"hash"`
}

// InsertAuditLog appends an immutable audit entry with SHA-256 integrity chain.
func (d *Database) InsertAuditLog(actor, action, resource string) error {
	var prevHash string
	err := d.db.QueryRow("SELECT COALESCE(MAX(hash), '') FROM audit_log").Scan(&prevHash)
	if err != nil {
		return fmt.Errorf("read prev hash: %w", err)
	}

	payload := fmt.Sprintf("%s|%s|%s|%s", time.Now().UTC().Format(time.RFC3339Nano), actor, action, resource)
	h := sha256.Sum256([]byte(prevHash + "|" + payload))
	hash := hex.EncodeToString(h[:])

	_, err = d.db.Exec(
		"INSERT INTO audit_log (timestamp, actor, action, resource, hash) VALUES (?, ?, ?, ?, ?)",
		time.Now().UTC(), actor, action, resource, hash,
	)
	return err
}

// GetAuditLogs returns the most recent audit entries, newest first.
func (d *Database) GetAuditLogs(limit int) ([]AuditEntry, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := d.db.Query(
		"SELECT id, timestamp, actor, action, resource, hash FROM audit_log ORDER BY id DESC LIMIT ?",
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []AuditEntry
	for rows.Next() {
		var e AuditEntry
		var ts string
		if err := rows.Scan(&e.ID, &ts, &e.Actor, &e.Action, &e.Resource, &e.Hash); err != nil {
			return nil, err
		}
		e.Timestamp, _ = time.Parse(time.RFC3339Nano, ts)
		entries = append(entries, e)
	}
	return entries, rows.Err()
}