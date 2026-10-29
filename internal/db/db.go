package db

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"github.com/ens-Emilio/odotoken/internal/domain"
)

const schema = `
CREATE TABLE IF NOT EXISTS token_events (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	timestamp TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
	provider TEXT NOT NULL DEFAULT '',
	model TEXT NOT NULL DEFAULT '',
	client_app TEXT NOT NULL DEFAULT 'unknown',
	prompt_tokens INTEGER NOT NULL DEFAULT 0,
	completion_tokens INTEGER NOT NULL DEFAULT 0,
	total_tokens INTEGER NOT NULL DEFAULT 0,
	cache_read_tokens INTEGER NOT NULL DEFAULT 0,
	cache_write_tokens INTEGER NOT NULL DEFAULT 0,
	cost_usd REAL NOT NULL DEFAULT 0.0,
	shadow_cost REAL NOT NULL DEFAULT 0.0,
	latency_ms REAL NOT NULL DEFAULT 0.0,
	ttfb_ms REAL NOT NULL DEFAULT 0.0,
	streaming_ms REAL NOT NULL DEFAULT 0.0,
	status_code INTEGER NOT NULL DEFAULT 200,
	project_id TEXT NOT NULL DEFAULT 'default',
	tags TEXT NOT NULL DEFAULT '',
	metadata TEXT NOT NULL DEFAULT '{}',
	source TEXT NOT NULL DEFAULT 'proxy',
	session_id TEXT NOT NULL DEFAULT '',
	session_file TEXT NOT NULL DEFAULT '',
	entry_id TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_events_timestamp ON token_events(timestamp);
CREATE INDEX IF NOT EXISTS idx_events_project ON token_events(project_id);
CREATE INDEX IF NOT EXISTS idx_events_model ON token_events(model);
CREATE INDEX IF NOT EXISTS idx_events_provider ON token_events(provider);
CREATE INDEX IF NOT EXISTS idx_events_ts_project_source ON token_events(timestamp, project_id, source);
CREATE INDEX IF NOT EXISTS idx_events_ts_source_project ON token_events(timestamp, source, project_id);

CREATE TABLE IF NOT EXISTS guard_events (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	timestamp TEXT NOT NULL,
	status TEXT NOT NULL,
	message TEXT NOT NULL,
	action TEXT NOT NULL,
	message_code TEXT NOT NULL DEFAULT '',
	message_params TEXT NOT NULL DEFAULT '{}',
	daily_cost_usd REAL NOT NULL DEFAULT 0,
	monthly_cost_usd REAL NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_guard_events_timestamp ON guard_events(timestamp);

CREATE TABLE IF NOT EXISTS active_runs (
	run_id TEXT PRIMARY KEY,
	session_id TEXT NOT NULL DEFAULT '',
	client_app TEXT NOT NULL,
	provider TEXT NOT NULL,
	model TEXT NOT NULL,
	project_id TEXT NOT NULL DEFAULT 'default',
	started_at TEXT NOT NULL,
	last_activity TEXT NOT NULL,
	finished_at TEXT,
	status TEXT NOT NULL DEFAULT 'running',
	error TEXT NOT NULL DEFAULT '',
	prompt_tokens INTEGER NOT NULL DEFAULT 0,
	completion_tokens INTEGER NOT NULL DEFAULT 0,
	cost_usd REAL NOT NULL DEFAULT 0.0
);
CREATE INDEX IF NOT EXISTS idx_active_runs_open ON active_runs(status, last_activity);

CREATE TABLE IF NOT EXISTS import_runs (
	id TEXT PRIMARY KEY,
	started_at TEXT NOT NULL,
	finished_at TEXT,
	status TEXT NOT NULL DEFAULT 'running',
	dry_run INTEGER NOT NULL DEFAULT 0,
	clis TEXT NOT NULL DEFAULT '',
	since TEXT NOT NULL DEFAULT '',
	inserted INTEGER NOT NULL DEFAULT 0,
	duplicates INTEGER NOT NULL DEFAULT 0,
	tokens INTEGER NOT NULL DEFAULT 0,
	cost_usd REAL NOT NULL DEFAULT 0.0,
	error TEXT NOT NULL DEFAULT '',
	report TEXT NOT NULL DEFAULT '{}'
);
CREATE INDEX IF NOT EXISTS idx_import_runs_started ON import_runs(started_at DESC);
`

// Database wraps SQLite operations.
type Database struct {
	db *sql.DB
}

// Open creates or opens a SQLite database at the given path with WAL mode.
func Open(path string) (*Database, error) {
	dsn := fmt.Sprintf("file:%s?cache=shared&mode=rwc&_journal_mode=WAL&_busy_timeout=5000", path)
	sqlDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	sqlDB.SetMaxOpenConns(1) // SQLite is single-writer
	sqlDB.SetMaxIdleConns(1)

	if _, err := sqlDB.Exec(schema); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("create schema: %w", err)
	}

	// The import uniqueness index is created after the schema and after
	// duplicate rows are collapsed: adding it to `schema` would fail on a
	// database that already imported the same session twice.
	// guard_events gained reason columns after the table shipped, so an
	// existing database needs them added rather than recreated.
	if err := migrateGuardReason(sqlDB); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("migrate guard reason: %w", err)
	}

	if err := migrateImportDedup(sqlDB); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("migrate import dedup: %w", err)
	}

	return &Database{db: sqlDB}, nil
}

// Close closes the database connection.
func (d *Database) Close() error {
	return d.db.Close()
}

// InsertEvent inserts a token event and returns its ID.
func (d *Database) InsertEvent(e domain.TokenEvent) (int64, error) {
	const q = `INSERT OR IGNORE INTO token_events (
		timestamp, provider, model, client_app,
		prompt_tokens, completion_tokens, total_tokens,
		cache_read_tokens, cache_write_tokens,
		cost_usd, shadow_cost, latency_ms, ttfb_ms, streaming_ms,
		status_code, project_id, tags, metadata, source,
		session_id, session_file, entry_id
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	ts := e.Timestamp.Format(time.RFC3339Nano)
	if e.Timestamp.IsZero() {
		ts = time.Now().UTC().Format(time.RFC3339Nano)
	}

	res, err := d.db.Exec(q,
		ts, e.Provider, e.Model, e.ClientApp,
		e.PromptTokens, e.CompletionTokens, e.TotalTokens,
		e.CacheReadTokens, e.CacheWriteTokens,
		e.CostUSD, e.ShadowCost, e.LatencyMs, e.TTFBMs, e.StreamingMs,
		e.StatusCode, e.ProjectID, e.Tags, e.Metadata, e.Source,
		e.SessionID, e.SessionFile, e.EntryID,
	)
	if err != nil {
		return 0, fmt.Errorf("insert event: %w", err)
	}
	return res.LastInsertId()
}

// InsertResult reports what a batch actually wrote. An import needs the
// distinction: a re-run must be able to say "0 new, 12000 already present"
// instead of claiming it imported everything again.
type InsertResult struct {
	Inserted   int
	Duplicates int
}

// InsertEvents inserts multiple events in a single transaction, skipping rows
// that already exist. Uniqueness for imported rows comes from
// idx_events_import_unique, not from application-level bookkeeping.
func (d *Database) InsertEvents(events []domain.TokenEvent) (InsertResult, error) {
	var result InsertResult
	if len(events) == 0 {
		return result, nil
	}

	tx, err := d.db.Begin()
	if err != nil {
		return result, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`INSERT OR IGNORE INTO token_events (
		timestamp, provider, model, client_app,
		prompt_tokens, completion_tokens, total_tokens,
		cache_read_tokens, cache_write_tokens,
		cost_usd, shadow_cost, latency_ms, ttfb_ms, streaming_ms,
		status_code, project_id, tags, metadata, source,
		session_id, session_file, entry_id
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return result, fmt.Errorf("prepare insert: %w", err)
	}
	defer stmt.Close()

	for _, e := range events {
		ts := e.Timestamp.Format(time.RFC3339Nano)
		if e.Timestamp.IsZero() {
			ts = time.Now().UTC().Format(time.RFC3339Nano)
		}
		res, err := stmt.Exec(
			ts, e.Provider, e.Model, e.ClientApp,
			e.PromptTokens, e.CompletionTokens, e.TotalTokens,
			e.CacheReadTokens, e.CacheWriteTokens,
			e.CostUSD, e.ShadowCost, e.LatencyMs, e.TTFBMs, e.StreamingMs,
			e.StatusCode, e.ProjectID, e.Tags, e.Metadata, e.Source,
			e.SessionID, e.SessionFile, e.EntryID,
		)
		if err != nil {
			return result, fmt.Errorf("insert event batch: %w", err)
		}
		if n, _ := res.RowsAffected(); n > 0 {
			result.Inserted++
		} else {
			result.Duplicates++
		}
	}

	if err := tx.Commit(); err != nil {
		return InsertResult{}, fmt.Errorf("commit insert batch: %w", err)
	}
	return result, nil
}

// GuardEvent records a budget state transition for the dashboard.
type GuardEvent struct {
	ID        int64  `json:"id"`
	Timestamp string `json:"timestamp"`
	Status    string `json:"status"`
	// Message is English prose for logs. MessageCode plus MessageParams are
	// what the dashboard renders, so the reader's language wins.
	Message        string  `json:"message"`
	MessageCode    string  `json:"message_code,omitempty"`
	MessageParams  string  `json:"message_params,omitempty"`
	Action         string  `json:"action"`
	DailyCostUSD   float64 `json:"daily_cost_usd"`
	MonthlyCostUSD float64 `json:"monthly_cost_usd"`
}

// InsertGuardEvent persists a state transition, not every budget poll.
func (d *Database) InsertGuardEvent(e GuardEvent) error {
	if e.MessageParams == "" {
		e.MessageParams = "{}"
	}
	_, err := d.db.Exec(`INSERT INTO guard_events
		(timestamp, status, message, action, message_code, message_params, daily_cost_usd, monthly_cost_usd)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		e.Timestamp, e.Status, e.Message, e.Action, e.MessageCode, e.MessageParams,
		e.DailyCostUSD, e.MonthlyCostUSD)
	return err
}

// RecentGuardEvents returns the latest budget transitions.
func (d *Database) RecentGuardEvents(limit int) ([]GuardEvent, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := d.db.Query(`SELECT id, timestamp, status, message, action,
		message_code, message_params, daily_cost_usd, monthly_cost_usd
		FROM guard_events ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []GuardEvent
	for rows.Next() {
		var e GuardEvent
		if err := rows.Scan(&e.ID, &e.Timestamp, &e.Status, &e.Message,
			&e.Action, &e.MessageCode, &e.MessageParams,
			&e.DailyCostUSD, &e.MonthlyCostUSD); err != nil {
			return nil, err
		}
		result = append(result, e)
	}
	return result, rows.Err()
}

// DailyStats holds aggregated stats for a single day.
type DailyStats struct {
	Date             string  `json:"date"`
	TotalRequests    int     `json:"total_requests"`
	TotalTokens      int     `json:"total_tokens"`
	PromptTokens     int     `json:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens"`
	CacheReadTokens  int     `json:"cache_read_tokens"`
	TotalCostUSD     float64 `json:"total_cost_usd"`
	ShadowCost       float64 `json:"shadow_cost"`
	AvgLatencyMs     float64 `json:"avg_latency_ms"`
}

// GetDailyStats returns per-day aggregated stats within a date range.
func (d *Database) GetDailyStats(from, to time.Time, projectID string, tz ...string) ([]DailyStats, error) {
	// Group by local day in Go, not in SQL: a fixed offset applied to the whole
	// range mislabels dates that cross a daylight-saving transition inside it.
	loc := time.UTC
	if len(tz) > 0 && tz[0] != "" && tz[0] != "UTC" {
		if l, err := time.LoadLocation(tz[0]); err == nil {
			loc = l
		}
	}
	q := `SELECT timestamp, total_tokens, prompt_tokens, completion_tokens, cache_read_tokens, cost_usd, shadow_cost, latency_ms
	FROM token_events WHERE timestamp >= ? AND timestamp < ?`
	args := []interface{}{from.UTC().Format(time.RFC3339), to.UTC().Format(time.RFC3339)}
	if projectID != "" {
		q += " AND project_id = ?"
		args = append(args, projectID)
	}
	q += " ORDER BY timestamp ASC"
	rows, err := d.db.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("daily stats: %w", err)
	}
	defer rows.Close()

	type acc struct {
		TotalRequests    int
		TotalTokens      int
		PromptTokens     int
		CompletionTokens int
		CacheReadTokens  int
		TotalCostUSD     float64
		ShadowCost       float64
		latencySum       float64
	}
	byDay := map[string]*acc{}
	var order []string
	for rows.Next() {
		var ts string
		var tokens, prompt, completion, cacheRead int
		var cost, shadow, latency float64
		if err := rows.Scan(&ts, &tokens, &prompt, &completion, &cacheRead, &cost, &shadow, &latency); err != nil {
			return nil, fmt.Errorf("scan daily stats: %w", err)
		}
		t, err := time.Parse(time.RFC3339Nano, ts)
		if err != nil {
			continue
		}
		day := t.In(loc).Format("2006-01-02")
		a := byDay[day]
		if a == nil {
			a = &acc{}
			byDay[day] = a
			order = append(order, day)
		}
		a.TotalRequests++
		a.TotalTokens += tokens
		a.PromptTokens += prompt
		a.CompletionTokens += completion
		a.CacheReadTokens += cacheRead
		a.TotalCostUSD += cost
		a.ShadowCost += shadow
		a.latencySum += latency
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	stats := make([]DailyStats, 0, len(order))
	for i := len(order) - 1; i >= 0; i-- {
		day := order[i]
		a := byDay[day]
		avg := 0.0
		if a.TotalRequests > 0 {
			avg = a.latencySum / float64(a.TotalRequests)
		}
		stats = append(stats, DailyStats{
			Date: day, TotalRequests: a.TotalRequests, TotalTokens: a.TotalTokens,
			PromptTokens: a.PromptTokens, CompletionTokens: a.CompletionTokens,
			CacheReadTokens: a.CacheReadTokens, TotalCostUSD: a.TotalCostUSD,
			ShadowCost: a.ShadowCost, AvgLatencyMs: avg,
		})
	}
	return stats, nil
}

// localDailyUsage scans the day boundaries in the caller's zone and buckets
// each event with time.Time.In, so a spring/fall DST change moves an event to
// the correct local date instead of using one offset for the whole window.
func (d *Database) localDailyUsage(days int, since, source string, tz ...string) ([]DailyUsage, error) {
	loc := time.UTC
	if len(tz) > 0 && tz[0] != "" && tz[0] != "UTC" {
		if l, err := time.LoadLocation(tz[0]); err == nil {
			loc = l
		}
	}
	// Build the list of local days first so days with zero events still show up
	// in order when the chart needs a continuous axis.
	now := time.Now().In(loc)
	startDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc).AddDate(0, 0, -days+1)
	if since != "" {
		if t, err := time.Parse(time.RFC3339Nano, since); err == nil {
			t = t.In(loc)
			startDay = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc)
		}
	}
	endDay := now
	// A caller may ask about a window entirely in the past. Keep the query
	// useful there instead of forcing the range to run until today.
	if since != "" {
		if t, err := time.Parse(time.RFC3339Nano, since); err == nil {
			t = t.In(loc)
			if days > 0 {
				endDay = t.AddDate(0, 0, days-1)
			}
		}
	}
	windowStart := startDay.AddDate(0, 0, -1).UTC()
	windowEnd := endDay.AddDate(0, 0, 2).UTC()
	q := `SELECT timestamp, total_tokens, cost_usd FROM token_events WHERE timestamp >= ? AND timestamp < ?`
	args := []interface{}{windowStart.Format(time.RFC3339), windowEnd.Format(time.RFC3339)}
	q, args = applySourceFilter(q, args, source)
	q += " ORDER BY timestamp ASC"
	rows, err := d.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	type acc struct {
		TotalRequests int
		TotalTokens   int
		CostUSD       float64
	}
	byDay := map[string]*acc{}
	for rows.Next() {
		var ts string
		var tokens int
		var cost float64
		if err := rows.Scan(&ts, &tokens, &cost); err != nil {
			return nil, err
		}
		t, err := time.Parse(time.RFC3339Nano, ts)
		if err != nil {
			continue
		}
		day := t.In(loc).Format("2006-01-02")
		a := byDay[day]
		if a == nil {
			a = &acc{}
			byDay[day] = a
		}
		a.TotalRequests++
		a.TotalTokens += tokens
		a.CostUSD += cost
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	result := make([]DailyUsage, 0, days)
	for day := startDay; !day.After(endDay); day = day.AddDate(0, 0, 1) {
		key := day.Format("2006-01-02")
		a := byDay[key]
		if a == nil {
			result = append(result, DailyUsage{Date: key})
			continue
		}
		result = append(result, DailyUsage{Date: key, TotalRequests: a.TotalRequests, TotalTokens: a.TotalTokens, CostUSD: a.CostUSD})
	}
	return result, nil
}

// GetDailyUsage returns per-day usage for the last N days.
func (d *Database) GetDailyUsage(days int, since, source string, tz ...string) ([]DailyUsage, error) {
	if days <= 0 {
		days = 45
	}
	return d.localDailyUsage(days, since, source, tz...)
}

// CostSummary holds aggregate cost data for budget checks.
type CostSummary struct {
	DailyCostUSD   float64 `json:"daily_cost_usd"`
	MonthlyCostUSD float64 `json:"monthly_cost_usd"`
	DailyTokens    int     `json:"daily_tokens"`
	MonthlyTokens  int     `json:"monthly_tokens"`
}

// GetCostSummary returns current day and month totals in UTC.
func (d *Database) GetCostSummary(projectID string) (CostSummary, error) {
	return d.GetCostSummaryInLocation(projectID, time.UTC)
}

// GetCostSummaryInLocation calculates the budget window in loc, so a dashboard
// configured for a local timezone and the guard agree on when a day begins.
func (d *Database) GetCostSummaryInLocation(projectID string, loc *time.Location) (CostSummary, error) {
	if loc == nil {
		loc = time.UTC
	}
	now := time.Now().In(loc)
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, loc)
	q := `SELECT
		COALESCE(SUM(CASE WHEN timestamp >= ? THEN cost_usd ELSE 0 END), 0),
		COALESCE(SUM(CASE WHEN timestamp >= ? THEN cost_usd ELSE 0 END), 0),
		COALESCE(SUM(CASE WHEN timestamp >= ? THEN total_tokens ELSE 0 END), 0),
		COALESCE(SUM(CASE WHEN timestamp >= ? THEN total_tokens ELSE 0 END), 0)
	FROM token_events WHERE project_id = ? AND source NOT LIKE 'import:%'`
	var s CostSummary
	err := d.db.QueryRow(q,
		dayStart.UTC().Format(time.RFC3339),
		monthStart.UTC().Format(time.RFC3339),
		dayStart.UTC().Format(time.RFC3339),
		monthStart.UTC().Format(time.RFC3339),
		projectID,
	).Scan(&s.DailyCostUSD, &s.MonthlyCostUSD, &s.DailyTokens, &s.MonthlyTokens)
	if err != nil {
		return s, fmt.Errorf("cost summary: %w", err)
	}
	return s, nil
}

// RecentEvents returns the N most recent token events.
func (d *Database) RecentEvents(limit int, projectID string) ([]domain.TokenEvent, error) {
	if limit <= 0 {
		limit = 20
	}
	q := `SELECT id, timestamp, provider, model, client_app,
		prompt_tokens, completion_tokens, total_tokens,
		cache_read_tokens, cache_write_tokens,
		cost_usd, shadow_cost, latency_ms, ttfb_ms, streaming_ms,
		status_code, project_id, tags, metadata, source,
		session_id, session_file, entry_id
	FROM token_events`
	args := []interface{}{}
	if projectID != "" {
		q += " WHERE project_id = ?"
		args = append(args, projectID)
	}
	q += " ORDER BY id DESC LIMIT ?"
	args = append(args, limit)

	rows, err := d.db.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("recent events: %w", err)
	}
	defer rows.Close()

	var events []domain.TokenEvent
	for rows.Next() {
		var e domain.TokenEvent
		var ts string
		if err := rows.Scan(&e.ID, &ts, &e.Provider, &e.Model, &e.ClientApp,
			&e.PromptTokens, &e.CompletionTokens, &e.TotalTokens,
			&e.CacheReadTokens, &e.CacheWriteTokens,
			&e.CostUSD, &e.ShadowCost, &e.LatencyMs, &e.TTFBMs, &e.StreamingMs,
			&e.StatusCode, &e.ProjectID, &e.Tags, &e.Metadata, &e.Source,
			&e.SessionID, &e.SessionFile, &e.EntryID); err != nil {
			return nil, fmt.Errorf("scan event: %w", err)
		}
		e.Timestamp, _ = time.Parse(time.RFC3339Nano, ts)
		events = append(events, e)
	}
	return events, rows.Err()
}

// ModelStats holds per-model aggregated usage.
type ModelStats struct {
	Model            string  `json:"model"`
	Provider         string  `json:"provider"`
	TotalRequests    int     `json:"total_requests"`
	TotalTokens      int     `json:"total_tokens"`
	PromptTokens     int     `json:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens"`
	TotalCostUSD     float64 `json:"total_cost_usd"`
}

// GetModelStats returns per-model aggregated stats.
func (d *Database) GetModelStats(from, to time.Time, projectID string) ([]ModelStats, error) {
	q := `SELECT model, provider,
		COUNT(*), SUM(total_tokens), SUM(prompt_tokens), SUM(completion_tokens), SUM(cost_usd)
	FROM token_events
	WHERE timestamp >= ? AND timestamp < ?`
	args := []interface{}{from.UTC().Format(time.RFC3339), to.UTC().Format(time.RFC3339)}
	if projectID != "" {
		q += " AND project_id = ?"
		args = append(args, projectID)
	}
	q += " GROUP BY model, provider ORDER BY total_cost_usd DESC"

	rows, err := d.db.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("model stats: %w", err)
	}
	defer rows.Close()

	var stats []ModelStats
	for rows.Next() {
		var s ModelStats
		if err := rows.Scan(&s.Model, &s.Provider, &s.TotalRequests,
			&s.TotalTokens, &s.PromptTokens, &s.CompletionTokens, &s.TotalCostUSD); err != nil {
			return nil, fmt.Errorf("scan model stats: %w", err)
		}
		stats = append(stats, s)
	}
	return stats, rows.Err()
}

// UpdateEventCost recalculates cost for an event (used by replay command).
func (d *Database) UpdateEventCost(id int64, costUSD, shadowCost float64) error {
	_, err := d.db.Exec("UPDATE token_events SET cost_usd = ?, shadow_cost = ? WHERE id = ?",
		costUSD, shadowCost, id)
	return err
}

// AllEventsForReplay returns all events for cost recalculation.
func (d *Database) AllEventsForReplay() ([]domain.TokenEvent, error) {
	rows, err := d.db.Query(`SELECT id, provider, model,
		prompt_tokens, completion_tokens, cache_read_tokens, cache_write_tokens
	FROM token_events ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []domain.TokenEvent
	for rows.Next() {
		var e domain.TokenEvent
		if err := rows.Scan(&e.ID, &e.Provider, &e.Model,
			&e.PromptTokens, &e.CompletionTokens, &e.CacheReadTokens, &e.CacheWriteTokens); err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, rows.Err()
}

// Summary holds aggregate stats for the dashboard.
type Summary struct {
	TotalRequests    int     `json:"total_requests"`
	TotalTokens      int     `json:"total_tokens"`
	PromptTokens     int     `json:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens"`
	CacheReadTokens  int     `json:"cache_read_tokens"`
	CacheWriteTokens int     `json:"cache_write_tokens"`
	TotalCostUSD     float64 `json:"total_cost_usd"`
	TotalShadowCost  float64 `json:"total_shadow_cost_usd"`
	AvgLatencyMs     float64 `json:"avg_latency_ms"`
	AvgTTFBMs        float64 `json:"avg_ttfb_ms"`
}

// GetSummary returns aggregate stats with optional filters.
func (d *Database) GetSummary(since, project, source, tag string) (Summary, error) {
	q := `SELECT COALESCE(COUNT(*),0), COALESCE(SUM(total_tokens),0),
		COALESCE(SUM(prompt_tokens),0), COALESCE(SUM(completion_tokens),0),
		COALESCE(SUM(cache_read_tokens),0), COALESCE(SUM(cache_write_tokens),0),
		COALESCE(SUM(cost_usd),0), COALESCE(SUM(shadow_cost),0),
		COALESCE(AVG(latency_ms),0), COALESCE(AVG(ttfb_ms),0)
		FROM token_events WHERE 1=1`
	args := []interface{}{}
	if since != "" {
		q += " AND timestamp >= ?"
		args = append(args, since)
	}
	if project != "" && project != "all" {
		q += " AND project_id = ?"
		args = append(args, project)
	}
	q, args = applySourceFilter(q, args, source)
	if tag != "" && tag != "all" {
		q += " AND (',' || tags || ',') LIKE ?"
		args = append(args, "%,"+tag+",%")
	}
	var s Summary
	err := d.db.QueryRow(q, args...).Scan(&s.TotalRequests, &s.TotalTokens,
		&s.PromptTokens, &s.CompletionTokens, &s.CacheReadTokens, &s.CacheWriteTokens,
		&s.TotalCostUSD, &s.TotalShadowCost, &s.AvgLatencyMs, &s.AvgTTFBMs)
	return s, err
}

// ByModel holds per-model aggregated stats.
type ByModel struct {
	Model          string  `json:"model"`
	Provider       string  `json:"provider"`
	TotalRequests  int     `json:"requests"`
	TotalTokens    int     `json:"total_tokens"`
	PromptTokens   int     `json:"prompt_tokens"`
	CompletionToks int     `json:"completion_tokens"`
	CacheReadToks  int     `json:"cache_read_tokens"`
	CacheWriteToks int     `json:"cache_write_tokens"`
	TotalCostUSD   float64 `json:"total_cost_usd"`
	PriceMissing   int     `json:"price_missing_count"`
}

// GetSummaryByModel returns per-model aggregated stats.
func (d *Database) GetSummaryByModel(since, project, source string) ([]ByModel, error) {
	q := `SELECT model, provider, COALESCE(COUNT(*),0), COALESCE(SUM(total_tokens),0),
		COALESCE(SUM(prompt_tokens),0), COALESCE(SUM(completion_tokens),0),
		COALESCE(SUM(cache_read_tokens),0), COALESCE(SUM(cache_write_tokens),0),
		COALESCE(SUM(cost_usd),0),
		COALESCE(SUM(CASE WHEN metadata LIKE '%"price_missing":true%' THEN 1 ELSE 0 END),0)
		FROM token_events WHERE 1=1`
	args := []interface{}{}
	if since != "" {
		q += " AND timestamp >= ?"
		args = append(args, since)
	}
	if project != "" && project != "all" {
		q += " AND project_id = ?"
		args = append(args, project)
	}
	q, args = applySourceFilter(q, args, source)
	q += " GROUP BY model, provider ORDER BY SUM(cost_usd) DESC"
	rows, err := d.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []ByModel
	for rows.Next() {
		var m ByModel
		if err := rows.Scan(&m.Model, &m.Provider, &m.TotalRequests,
			&m.TotalTokens, &m.PromptTokens, &m.CompletionToks,
			&m.CacheReadToks, &m.CacheWriteToks, &m.TotalCostUSD, &m.PriceMissing); err != nil {
			return nil, err
		}
		result = append(result, m)
	}
	return result, rows.Err()
}

// ByClient holds per-client-app aggregated stats.
type ByClient struct {
	ClientApp     string  `json:"client_app"`
	TotalRequests int     `json:"requests"`
	TotalTokens   int     `json:"total_tokens"`
	TotalCostUSD  float64 `json:"total_cost_usd"`
}

// GetSummaryByClient returns per-client-app aggregated stats.
func (d *Database) GetSummaryByClient(since, project, source string) ([]ByClient, error) {
	q := `SELECT client_app, COALESCE(COUNT(*),0), COALESCE(SUM(total_tokens),0), COALESCE(SUM(cost_usd),0)
		FROM token_events WHERE 1=1`
	args := []interface{}{}
	if since != "" {
		q += " AND timestamp >= ?"
		args = append(args, since)
	}
	if project != "" && project != "all" {
		q += " AND project_id = ?"
		args = append(args, project)
	}
	q, args = applySourceFilter(q, args, source)
	q += " GROUP BY client_app ORDER BY SUM(cost_usd) DESC"
	rows, err := d.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []ByClient
	for rows.Next() {
		var c ByClient
		if err := rows.Scan(&c.ClientApp, &c.TotalRequests, &c.TotalTokens, &c.TotalCostUSD); err != nil {
			return nil, err
		}
		result = append(result, c)
	}
	return result, rows.Err()
}

// ByProject holds per-project aggregated stats.
type ByProject struct {
	ProjectID     string  `json:"project_id"`
	TotalRequests int     `json:"requests"`
	TotalTokens   int     `json:"total_tokens"`
	TotalCostUSD  float64 `json:"total_cost_usd"`
	ShadowCost    float64 `json:"shadow_cost_usd"`
}

// GetSummaryByProject returns per-project aggregated stats.
func (d *Database) GetSummaryByProject(since, source string) ([]ByProject, error) {
	q := `SELECT project_id, COALESCE(COUNT(*),0), COALESCE(SUM(total_tokens),0),
		COALESCE(SUM(cost_usd),0), COALESCE(SUM(shadow_cost),0)
		FROM token_events WHERE 1=1`
	args := []interface{}{}
	if since != "" {
		q += " AND timestamp >= ?"
		args = append(args, since)
	}
	q, args = applySourceFilter(q, args, source)
	q += " GROUP BY project_id ORDER BY SUM(cost_usd) DESC"
	rows, err := d.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []ByProject
	for rows.Next() {
		var p ByProject
		if err := rows.Scan(&p.ProjectID, &p.TotalRequests, &p.TotalTokens, &p.TotalCostUSD, &p.ShadowCost); err != nil {
			return nil, err
		}
		result = append(result, p)
	}
	return result, rows.Err()
}

// BySource holds per-source aggregated stats.
type BySource struct {
	Source        string  `json:"source"`
	TotalRequests int     `json:"requests"`
	TotalTokens   int     `json:"total_tokens"`
	TotalCostUSD  float64 `json:"total_cost_usd"`
}

// GetSummaryBySource returns per-source aggregated stats.
func (d *Database) GetSummaryBySource(since, project string) ([]BySource, error) {
	q := `SELECT source, COALESCE(COUNT(*),0), COALESCE(SUM(total_tokens),0), COALESCE(SUM(cost_usd),0)
		FROM token_events WHERE 1=1`
	args := []interface{}{}
	if since != "" {
		q += " AND timestamp >= ?"
		args = append(args, since)
	}
	if project != "" && project != "all" {
		q += " AND project_id = ?"
		args = append(args, project)
	}
	q += " GROUP BY source ORDER BY SUM(cost_usd) DESC"
	rows, err := d.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []BySource
	for rows.Next() {
		var s BySource
		if err := rows.Scan(&s.Source, &s.TotalRequests, &s.TotalTokens, &s.TotalCostUSD); err != nil {
			return nil, err
		}
		result = append(result, s)
	}
	return result, rows.Err()
}

// ByTag holds per-tag aggregated stats.
type ByTag struct {
	Tag           string  `json:"tag"`
	TotalRequests int     `json:"requests"`
	TotalTokens   int     `json:"total_tokens"`
	TotalCostUSD  float64 `json:"total_cost_usd"`
}

// GetSummaryByTag returns per-tag aggregated stats.
func (d *Database) GetSummaryByTag(since, project, source string) ([]ByTag, error) {
	q := `SELECT tags, COALESCE(COUNT(*),0), COALESCE(SUM(total_tokens),0), COALESCE(SUM(cost_usd),0)
		FROM token_events WHERE tags != '' AND 1=1`
	args := []interface{}{}
	if since != "" {
		q += " AND timestamp >= ?"
		args = append(args, since)
	}
	if project != "" && project != "all" {
		q += " AND project_id = ?"
		args = append(args, project)
	}
	q, args = applySourceFilter(q, args, source)
	q += " GROUP BY tags ORDER BY SUM(cost_usd) DESC"
	rows, err := d.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []ByTag
	for rows.Next() {
		var t ByTag
		if err := rows.Scan(&t.Tag, &t.TotalRequests, &t.TotalTokens, &t.TotalCostUSD); err != nil {
			return nil, err
		}
		result = append(result, t)
	}
	return result, rows.Err()
}

// DailyUsage holds per-day aggregated usage.
type DailyUsage struct {
	Date          string  `json:"date"`
	TotalRequests int     `json:"requests"`
	TotalTokens   int     `json:"total_tokens"`
	CostUSD       float64 `json:"cost_usd"`
}

// LiveSession represents a model request currently in flight.
type LiveSession struct {
	RunID        string  `json:"run_id"`
	CLI          string  `json:"cli"`
	SessionID    string  `json:"session_id"`
	Provider     string  `json:"provider"`
	Model        string  `json:"model"`
	TokensInput  int     `json:"tokens_input"`
	TokensOutput int     `json:"tokens_output"`
	CostUSD      float64 `json:"cost_usd"`
	Project      string  `json:"project"`
	StartedAt    string  `json:"started_at"`
	LastUpdated  string  `json:"last_updated"`
	ElapsedMs    int64   `json:"elapsed_ms"`
}

// StartActiveRun records a request before its upstream response is received.
func (d *Database) StartActiveRun(runID, sessionID, clientApp, provider, model, projectID string) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if sessionID == "" {
		sessionID = runID
	}
	_, err := d.db.Exec(`INSERT INTO active_runs
		(run_id, session_id, client_app, provider, model, project_id, started_at, last_activity, status)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, 'running')
		ON CONFLICT(run_id) DO UPDATE SET
			session_id=excluded.session_id, client_app=excluded.client_app,
			provider=excluded.provider, model=excluded.model,
			project_id=excluded.project_id, last_activity=excluded.last_activity,
			finished_at=NULL, status='running', error=''`,
		runID, sessionID, clientApp, provider, model, projectID, now, now)
	return err
}

// TouchActiveRun keeps a long-running stream visible.
func (d *Database) TouchActiveRun(runID string) error {
	_, err := d.db.Exec(`UPDATE active_runs SET last_activity=? WHERE run_id=? AND status='running'`,
		time.Now().UTC().Format(time.RFC3339Nano), runID)
	return err
}

// FinishActiveRun closes a run. Repeated completion paths are idempotent.
func (d *Database) FinishActiveRun(runID, status, errMessage string, promptTokens, completionTokens int, costUSD float64) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := d.db.Exec(`UPDATE active_runs SET finished_at=?, last_activity=?,
		status=?, error=?, prompt_tokens=?, completion_tokens=?, cost_usd=?
		WHERE run_id=? AND finished_at IS NULL`,
		now, now, status, errMessage, promptTokens, completionTokens, costUSD, runID)
	return err
}

// ExpireOrphanRuns closes runs whose process vanished and applies a 7-day
// retention window to completed rows. The 10 minute ceiling prevents a stale
// row surviving a crash while a response is still streaming.
func (d *Database) ExpireOrphanRuns(maxIdle time.Duration) error {
	cutoff := time.Now().UTC().Add(-maxIdle).Format(time.RFC3339Nano)
	now := time.Now().UTC().Format(time.RFC3339Nano)
	result, err := d.db.Exec(`UPDATE active_runs SET finished_at=?, last_activity=?,
		status='expired', error='no activity before timeout'
		WHERE finished_at IS NULL AND status='running' AND last_activity < ?`,
		now, now, cutoff)
	if err != nil {
		return err
	}
	_ = result
	retentionCutoff := time.Now().UTC().Add(-7 * 24 * time.Hour).Format(time.RFC3339Nano)
	_, err = d.db.Exec(`DELETE FROM active_runs WHERE finished_at IS NOT NULL AND finished_at < ?`, retentionCutoff)
	return err
}

// DailyDetailRow holds aggregated usage for a single day filtered by client or source.
type DailyDetailRow struct {
	Key         string  `json:"key"`
	Requests    int     `json:"requests"`
	TotalTokens int     `json:"total_tokens"`
	CostUSD     float64 `json:"cost_usd"`
}

// GetDailyDetail returns per-client and per-source breakdown for a specific day,
// respecting the user's timezone so the clicked calendar cell matches local midnight.
func (d *Database) GetDailyDetail(day, tz, project, source, tag string) (summary Summary, byClient []DailyDetailRow, bySource []DailyDetailRow, err error) {
	loc := time.UTC
	if tz != "" && tz != "UTC" {
		if l, loadErr := time.LoadLocation(tz); loadErr == nil {
			loc = l
		}
	}
	dayStart, parseErr := time.ParseInLocation("2006-01-02", day, loc)
	if parseErr != nil {
		return summary, nil, nil, fmt.Errorf("invalid day format: %w", parseErr)
	}
	dayEnd := dayStart.Add(24 * time.Hour)
	startStr := dayStart.UTC().Format(time.RFC3339)
	endStr := dayEnd.UTC().Format(time.RFC3339)

	baseWhere := "WHERE timestamp >= ? AND timestamp < ?"
	args := []interface{}{startStr, endStr}
	if project != "" && project != "all" {
		baseWhere += " AND project_id = ?"
		args = append(args, project)
	}
	baseWhere, args = applySourceFilter(baseWhere, args, source)
	if tag != "" && tag != "all" {
		baseWhere += " AND (',' || tags || ',') LIKE ?"
		args = append(args, "%,"+tag+",%")
	}

	sumQ := fmt.Sprintf(`SELECT COALESCE(COUNT(*),0), COALESCE(SUM(prompt_tokens),0), COALESCE(SUM(completion_tokens),0),
		COALESCE(SUM(total_tokens),0), COALESCE(SUM(cache_read_tokens),0), COALESCE(SUM(cache_write_tokens),0),
		COALESCE(SUM(cost_usd),0), COALESCE(AVG(latency_ms),0), COALESCE(AVG(ttfb_ms),0)
		FROM token_events %s`, baseWhere)
	row := d.db.QueryRow(sumQ, args...)
	if scanErr := row.Scan(&summary.TotalRequests, &summary.PromptTokens, &summary.CompletionTokens,
		&summary.TotalTokens, &summary.CacheReadTokens, &summary.CacheWriteTokens,
		&summary.TotalCostUSD, &summary.AvgLatencyMs, &summary.AvgTTFBMs); scanErr != nil && scanErr != sql.ErrNoRows {
		return summary, nil, nil, scanErr
	}

	clientQ := fmt.Sprintf(`SELECT client_app, COUNT(*) AS requests, SUM(total_tokens) AS total_tokens, SUM(cost_usd) AS cost_usd
		FROM token_events %s GROUP BY client_app ORDER BY cost_usd DESC`, baseWhere)
	cRows, cErr := d.db.Query(clientQ, args...)
	if cErr != nil {
		return summary, nil, nil, cErr
	}
	defer cRows.Close()
	for cRows.Next() {
		var r DailyDetailRow
		if scanErr := cRows.Scan(&r.Key, &r.Requests, &r.TotalTokens, &r.CostUSD); scanErr != nil {
			return summary, nil, nil, scanErr
		}
		byClient = append(byClient, r)
	}
	if cRows.Err() != nil {
		return summary, nil, nil, cRows.Err()
	}

	sourceQ := fmt.Sprintf(`SELECT source, COUNT(*) AS requests, SUM(total_tokens) AS total_tokens, SUM(cost_usd) AS cost_usd
		FROM token_events %s GROUP BY source ORDER BY cost_usd DESC`, baseWhere)
	sRows, sErr := d.db.Query(sourceQ, args...)
	if sErr != nil {
		return summary, nil, nil, sErr
	}
	defer sRows.Close()
	for sRows.Next() {
		var r DailyDetailRow
		if scanErr := sRows.Scan(&r.Key, &r.Requests, &r.TotalTokens, &r.CostUSD); scanErr != nil {
			return summary, nil, nil, scanErr
		}
		bySource = append(bySource, r)
	}
	if sRows.Err() != nil {
		return summary, nil, nil, sRows.Err()
	}

	return summary, byClient, bySource, nil
}

// GetLiveSessions returns actual in-flight runs, not completed event rows.
func (d *Database) GetLiveSessions() ([]LiveSession, error) {
	if err := d.ExpireOrphanRuns(10 * time.Minute); err != nil {
		return nil, err
	}
	rows, err := d.db.Query(`SELECT run_id, session_id, client_app, provider, model,
		project_id, started_at, last_activity, prompt_tokens, completion_tokens, cost_usd
		FROM active_runs WHERE finished_at IS NULL AND status='running'
		ORDER BY started_at DESC LIMIT 50`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	now := time.Now().UTC()
	var result []LiveSession
	for rows.Next() {
		var ls LiveSession
		var started string
		if err := rows.Scan(&ls.RunID, &ls.SessionID, &ls.CLI, &ls.Provider, &ls.Model,
			&ls.Project, &started, &ls.LastUpdated, &ls.TokensInput,
			&ls.TokensOutput, &ls.CostUSD); err != nil {
			return nil, err
		}
		if ls.SessionID == "" {
			ls.SessionID = ls.RunID
		}
		ls.StartedAt = started
		if t, err := time.Parse(time.RFC3339Nano, started); err == nil {
			ls.ElapsedMs = now.Sub(t).Milliseconds()
		}
		result = append(result, ls)
	}
	return result, rows.Err()
}

// TpsSummary holds measured output throughput. Total tokens are reported
// separately because prompt and cache tokens are not streamed back.
type TpsSummary struct {
	AvgTPS           float64 `json:"avg_tps"`
	PeakTPS          float64 `json:"peak_tps"`
	TotalReqs        int     `json:"total_requests"`
	TotalTokens      int     `json:"total_tokens"`
	CompletionTokens int     `json:"completion_tokens"`
	AvgLatencyMs     float64 `json:"avg_latency_ms"`
}

// maxPlausibleTPS caps a single response's implied output rate. A row above it
// has a broken streaming_ms and must not reach the dashboard.
const maxPlausibleTPS = 1000.0

// genSecsExpr is the wall time the model spent producing output, in ms.
// streaming_ms wins when plausible; otherwise latency_ms is floored at the time
// the completion itself requires at maxPlausibleTPS.
var genSecsExpr = `(CASE
	WHEN streaming_ms >= 50
		AND completion_tokens * 1000.0 / streaming_ms <= ` + fmt.Sprintf("%f", maxPlausibleTPS) + `
	THEN streaming_ms
	ELSE MAX(latency_ms, completion_tokens * 1000.0 / ` + fmt.Sprintf("%f", maxPlausibleTPS) + `)
	END)`

// GetTpsSummary returns throughput summary.
func (d *Database) GetTpsSummary(since, project, source string) (TpsSummary, error) {
	var s TpsSummary
	where := ` WHERE (streaming_ms > 0 OR latency_ms > 0)`
	args := []interface{}{}
	if since != "" {
		where += " AND timestamp >= ?"
		args = append(args, since)
	}
	if project != "" && project != "all" {
		where += " AND project_id = ?"
		args = append(args, project)
	}
	where, args = applySourceFilter(where, args, source)
	q := `SELECT COUNT(*), COALESCE(SUM(completion_tokens),0),
		COALESCE(SUM(total_tokens),0), COALESCE(AVG(latency_ms),0),
		COALESCE(SUM(` + genSecsExpr + `),0)/1000.0
		FROM token_events` + where
	var genSecs float64
	if err := d.db.QueryRow(q, args...).
		Scan(&s.TotalReqs, &s.CompletionTokens, &s.TotalTokens, &s.AvgLatencyMs, &genSecs); err != nil {
		return s, err
	}
	if genSecs > 0 {
		s.AvgTPS = float64(s.CompletionTokens) / genSecs
	}
	peakQ := `SELECT COALESCE(MAX(b), 0) FROM (
		SELECT SUM(completion_tokens) /
			MAX(SUM(` + genSecsExpr + `) / 1000.0,
				CAST(strftime('%s', MAX(timestamp)) AS INTEGER) - CAST(strftime('%s', MIN(timestamp)) AS INTEGER),
				10.0) AS b
		FROM token_events` + where + `
		GROUP BY CAST(strftime('%s', timestamp) AS INTEGER)/10)`
	if err := d.db.QueryRow(peakQ, args...).Scan(&s.PeakTPS); err != nil {
		s.PeakTPS = 0
	}
	return s, nil
}

// TpsSeriesPoint holds a single time-series bucket.
type TpsSeriesPoint struct {
	Timestamp string  `json:"timestamp"`
	AvgTPS    float64 `json:"avg_tps"`
	PeakTPS   float64 `json:"peak_tps"`
	Requests  int     `json:"requests"`
	Tokens    int     `json:"tokens"`
}

// GetTpsSeries returns throughput per wall-clock bucket.
func (d *Database) GetTpsSeries(since, interval, project, source string) ([]TpsSeriesPoint, error) {
	groupExpr := "strftime('%Y-%m-%dT%H:00:00Z', timestamp)"
	switch interval {
	case "1d":
		groupExpr = "substr(timestamp,1,10)"
	case "1m":
		groupExpr = "strftime('%Y-%m-%dT%H:%M:00Z', timestamp)"
	case "5m":
		groupExpr = "strftime('%Y-%m-%dT%H:', timestamp) || printf('%02d', (CAST(strftime('%M', timestamp) AS INTEGER)/5)*5) || ':00Z'"
	}
	peakExpr := `MAX(CASE
		WHEN streaming_ms >= 50 AND completion_tokens * 1000.0 / streaming_ms <= ` + fmt.Sprintf("%f", maxPlausibleTPS) + `
		THEN completion_tokens * 1000.0 / streaming_ms ELSE 0 END)`
	q := fmt.Sprintf(`SELECT %s as ts, COUNT(*), COALESCE(SUM(completion_tokens),0),
		COALESCE(SUM(completion_tokens)*1000.0/NULLIF(SUM(`+genSecsExpr+`),0),0),
		COALESCE(`+peakExpr+`,0)
		FROM token_events WHERE (streaming_ms > 0 OR latency_ms > 0)`, groupExpr)
	args := []interface{}{}
	if since != "" {
		q += " AND timestamp >= ?"
		args = append(args, since)
	}
	if project != "" && project != "all" {
		q += " AND project_id = ?"
		args = append(args, project)
	}
	q, args = applySourceFilter(q, args, source)
	q += " GROUP BY ts ORDER BY ts ASC"
	rows, err := d.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []TpsSeriesPoint
	for rows.Next() {
		var p TpsSeriesPoint
		if err := rows.Scan(&p.Timestamp, &p.Requests, &p.Tokens, &p.AvgTPS, &p.PeakTPS); err != nil {
			return nil, err
		}
		result = append(result, p)
	}
	return result, rows.Err()
}

// GetLiveTPS returns measured output throughput (completion tokens per
// measured generation second) for events in the last N seconds. Total tokens
// include prompt/cache, so dividing them by the wall-clock window inflates the
// number; and streaming latency includes thinking time before the first token,
// so per-second throughput comes from completion tokens over the time the
// model actually spent generating (streaming_ms when plausible, else latency).
// Only proxy rows count: imported rows carry historical sessions with no
// timing and would fabricate activity the moment an import command runs.
func (d *Database) GetLiveTPS(seconds int) (float64, error) {
	if seconds <= 0 {
		seconds = 10
	}
	cutoffUnix := time.Now().UTC().Add(-time.Duration(seconds) * time.Second).Unix()
	var completion int
	var genSecs float64
	err := d.db.QueryRow(
		`SELECT COALESCE(SUM(completion_tokens),0),
		 COALESCE(SUM(`+genSecsExpr+`)/1000.0, 0)
		 FROM token_events
		 WHERE CAST(strftime('%s', timestamp) AS INTEGER) >= ?
		   AND source = 'proxy'
		   AND source = 'proxy'
		   AND (streaming_ms > 0 OR latency_ms > 0)`, cutoffUnix).
		Scan(&completion, &genSecs)
	if err != nil || genSecs <= 0 {
		return 0, err
	}
	return float64(completion) / genSecs, nil
}

// RecentEventsFiltered returns recent events with all dashboard filters.
func (d *Database) RecentEventsFiltered(limit int, since, project, source, tag string) ([]domain.TokenEvent, error) {
	if limit <= 0 {
		limit = 50
	}
	q := `SELECT id, timestamp, provider, model, client_app,
		prompt_tokens, completion_tokens, total_tokens,
		cache_read_tokens, cache_write_tokens,
		cost_usd, shadow_cost, latency_ms, ttfb_ms, streaming_ms,
		status_code, project_id, tags, metadata, source,
		session_id, session_file, entry_id
		FROM token_events WHERE 1=1`
	args := []interface{}{}
	if since != "" {
		q += " AND timestamp >= ?"
		args = append(args, since)
	}
	if project != "" && project != "all" {
		q += " AND project_id = ?"
		args = append(args, project)
	}
	q, args = applySourceFilter(q, args, source)
	if tag != "" && tag != "all" {
		q += " AND (',' || tags || ',') LIKE ?"
		args = append(args, "%,"+tag+",%")
	}
	q += " ORDER BY id DESC LIMIT ?"
	args = append(args, limit)
	rows, err := d.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var events []domain.TokenEvent
	for rows.Next() {
		var e domain.TokenEvent
		var ts string
		if err := rows.Scan(&e.ID, &ts, &e.Provider, &e.Model, &e.ClientApp,
			&e.PromptTokens, &e.CompletionTokens, &e.TotalTokens,
			&e.CacheReadTokens, &e.CacheWriteTokens,
			&e.CostUSD, &e.ShadowCost, &e.LatencyMs, &e.TTFBMs, &e.StreamingMs,
			&e.StatusCode, &e.ProjectID, &e.Tags, &e.Metadata, &e.Source,
			&e.SessionID, &e.SessionFile, &e.EntryID); err != nil {
			return nil, err
		}
		e.Timestamp, _ = time.Parse(time.RFC3339Nano, ts)
		events = append(events, e)
	}
	return events, rows.Err()
}

// migrateImportDedup makes re-running an import idempotent.
//
// The uniqueness index is partial so proxy rows, whose session_file and
// entry_id are empty, are never affected. It is created after the schema and
// after duplicates are collapsed: creating it against a database that already
// imported the same session twice would fail and lock the daemon out.
func migrateImportDedup(sqlDB *sql.DB) error {
	tx, err := sqlDB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Keep the oldest row of each duplicate group: its id preserves the order
	// the original import produced, and cost is recomputed by the import
	// engine rather than by this migration.
	if _, err := tx.Exec(`DELETE FROM token_events
		WHERE source LIKE 'import:%'
		  AND session_file != ''
		  AND entry_id != ''
		  AND id NOT IN (
			SELECT MIN(id) FROM token_events
			WHERE source LIKE 'import:%' AND session_file != '' AND entry_id != ''
			GROUP BY source, session_file, entry_id
		  )`); err != nil {
		return err
	}

	if _, err := tx.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_events_import_unique
		ON token_events(source, session_file, entry_id)
		WHERE source LIKE 'import:%'`); err != nil {
		return err
	}

	return tx.Commit()
}

// migrateGuardReason adds the reason columns to a guard_events table created
// before they existed. A table created by the current schema already has them,
// so the ALTER is allowed to fail with the duplicate-column error.
func migrateGuardReason(sqlDB *sql.DB) error {
	for _, statement := range []string{
		`ALTER TABLE guard_events ADD COLUMN message_code TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE guard_events ADD COLUMN message_params TEXT NOT NULL DEFAULT '{}'`,
	} {
		if _, err := sqlDB.Exec(statement); err != nil &&
			!strings.Contains(err.Error(), "duplicate column name") {
			return err
		}
	}
	return nil
}

// sourceFilter renders the `source` dashboard filter.
//
// The dashboard offers "Imported" as one choice while the rows it selects are
// stored per CLI as `import:claude`, `import:omp` and so on. Matching that
// choice with equality silently returned nothing, so the bucket exists in the
// filter and is always empty.
func sourceFilter(source string) (clause string, arg interface{}, ok bool) {
	if source == "" || source == "all" {
		return "", nil, false
	}
	if source == "import" {
		return " AND source LIKE 'import:%'", nil, true
	}
	return " AND source = ?", source, true
}

func applySourceFilter(q string, args []interface{}, source string) (string, []interface{}) {
	clause, arg, ok := sourceFilter(source)
	if !ok {
		return q, args
	}
	if arg != nil {
		args = append(args, arg)
	}
	return q + clause, args
}
