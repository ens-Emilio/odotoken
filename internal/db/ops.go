package db

import (
	"database/sql"
	"fmt"
	"time"
)


// RetentionResult reports what a retention pass did, so an operator can tell
// "nothing old" from "retention disabled".
type RetentionResult struct {
	Deleted int64  `json:"deleted"`
	Vacuum  bool   `json:"vacuum"`
	Oldest  string `json:"oldest,omitempty"`
}

// ApplyRetention deletes token_events older than days. A non-positive value
// disables retention and deletes nothing.
//
// VACUUM is a separate opt-in because it rewrites the database file and holds
// the single write lock; it only runs when rows were actually removed.
func (d *Database) ApplyRetention(days int, vacuum bool) (RetentionResult, error) {
	var result RetentionResult
	if days <= 0 {
		return result, nil
	}
	cutoff := time.Now().UTC().AddDate(0, 0, -days).Format(time.RFC3339Nano)
	res, err := d.db.Exec(`DELETE FROM token_events WHERE timestamp < ?`, cutoff)
	if err != nil {
		return result, fmt.Errorf("retention delete: %w", err)
	}
	result.Deleted, _ = res.RowsAffected()

	if vacuum && result.Deleted > 0 {
		if err := d.Vacuum(); err != nil {
			return result, err
		}
		result.Vacuum = true
	}
	var oldest sql.NullString
	if err := d.db.QueryRow(`SELECT MIN(timestamp) FROM token_events`).Scan(&oldest); err == nil {
		result.Oldest = oldest.String
	}
	return result, nil
}

// Vacuum reclaims file space after a large deletion. It cannot run inside a
// transaction and blocks the single writer for its duration.
func (d *Database) Vacuum() error {
	if _, err := d.db.Exec(`VACUUM`); err != nil {
		return fmt.Errorf("vacuum: %w", err)
	}
	return nil
}

// AlertOutboxItem is one queued notification.
type AlertOutboxItem struct {
	ID        int64  `json:"id"`
	Channel   string `json:"channel"`
	Target    string `json:"target"`
	Payload   string `json:"payload"`
	Attempts  int    `json:"attempts"`
	LastError string `json:"last_error,omitempty"`
	NextTryAt string `json:"next_attempt_at"`
	CreatedAt string `json:"created_at"`
}

// EnqueueAlert persists a notification before any network call, so delivery
// survives a restart instead of dying with the goroutine that started it.
func (d *Database) EnqueueAlert(channel, target, payload string, nextAttempt time.Time) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := d.db.Exec(`INSERT INTO alert_outbox
		(created_at, channel, target, payload, attempts, next_attempt_at)
		VALUES (?, ?, ?, ?, 0, ?)`,
		now, channel, target, payload, nextAttempt.UTC().Format(time.RFC3339Nano))
	return err
}

// PendingAlerts returns notifications whose retry time has arrived.
func (d *Database) PendingAlerts(limit int) ([]AlertOutboxItem, error) {
	if limit <= 0 {
		limit = 50
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	rows, err := d.db.Query(`SELECT id, channel, target, payload, attempts, last_error,
		next_attempt_at, created_at
		FROM alert_outbox WHERE delivered_at IS NULL AND next_attempt_at <= ?
		ORDER BY id LIMIT ?`, now, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []AlertOutboxItem
	for rows.Next() {
		var item AlertOutboxItem
		if err := rows.Scan(&item.ID, &item.Channel, &item.Target, &item.Payload,
			&item.Attempts, &item.LastError, &item.NextTryAt, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// MarkAlertDelivered closes an item the channel accepted.
func (d *Database) MarkAlertDelivered(id int64) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := d.db.Exec(`UPDATE alert_outbox SET delivered_at=?, attempts=attempts+1,
		last_error='' WHERE id=?`, now, id)
	return err
}

// MarkAlertFailed records the failed attempt and schedules the next one.
func (d *Database) MarkAlertFailed(id int64, attempts int, nextAttempt time.Time, message string) error {
	_, err := d.db.Exec(`UPDATE alert_outbox SET attempts=?, next_attempt_at=?,
		last_error=? WHERE id=?`,
		attempts, nextAttempt.UTC().Format(time.RFC3339Nano), message, id)
	return err
}

// DropExhaustedAlerts removes undelivered rows that ran out of retries, so a
// permanently broken webhook does not grow the table forever.
func (d *Database) DropExhaustedAlerts(maxAttempts int) (int64, error) {
	if maxAttempts <= 0 {
		maxAttempts = 5
	}
	res, err := d.db.Exec(`DELETE FROM alert_outbox
		WHERE delivered_at IS NULL AND attempts >= ?`, maxAttempts)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return n, nil
}

// PendingAlertCount reports queue depth for doctor and metrics.
func (d *Database) PendingAlertCount() (int, error) {
	var n int
	err := d.db.QueryRow(`SELECT COUNT(*) FROM alert_outbox WHERE delivered_at IS NULL`).Scan(&n)
	return n, err
}

// MetricSeries is one labelled sample set for the Prometheus snapshot.
type MetricSeries struct {
	Model      string
	Provider   string
	Project    string
	Status     int
	Requests   int64
	Prompt     int64
	Completion int64
	CacheRead  int64
	CostUSD    float64
}

// MetricsSnapshot aggregates the counters exposed on /metrics.
//
// The numbers are derived from SQLite instead of in-memory counters so they
// survive a restart and the process stays single-writer.
func (d *Database) MetricsSnapshot() ([]MetricSeries, error) {
	rows, err := d.db.Query(`SELECT COALESCE(model,''), COALESCE(provider,''),
		COALESCE(project_id,''), COALESCE(status_code,0),
		COUNT(*), COALESCE(SUM(prompt_tokens),0), COALESCE(SUM(completion_tokens),0),
		COALESCE(SUM(cache_read_tokens),0), COALESCE(SUM(cost_usd),0)
		FROM token_events
		GROUP BY model, provider, project_id, status_code`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []MetricSeries
	for rows.Next() {
		var s MetricSeries
		if err := rows.Scan(&s.Model, &s.Provider, &s.Project, &s.Status,
			&s.Requests, &s.Prompt, &s.Completion, &s.CacheRead, &s.CostUSD); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// LatencyBucket is one cumulative histogram bucket over latency_ms.
type LatencyBucket struct {
	Le    float64
	Count int64
}

// LatencyBuckets returns cumulative counts for the given upper bounds.
func (d *Database) LatencyBuckets(bounds []float64) ([]LatencyBucket, int64, error) {
	out := make([]LatencyBucket, 0, len(bounds))
	for _, le := range bounds {
		var n int64
		if err := d.db.QueryRow(`SELECT COUNT(*) FROM token_events
			WHERE latency_ms >= 0 AND latency_ms <= ?`, le).Scan(&n); err != nil {
			return nil, 0, err
		}
		out = append(out, LatencyBucket{Le: le, Count: n})
	}
	var total int64
	if err := d.db.QueryRow(`SELECT COUNT(*) FROM token_events WHERE latency_ms >= 0`).Scan(&total); err != nil {
		return nil, 0, err
	}
	return out, total, nil
}
