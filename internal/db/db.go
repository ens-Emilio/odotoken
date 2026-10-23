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

// InsertEvents inserts multiple events in a single transaction.
func (d *Database) InsertEvents(events []domain.TokenEvent) error {
	tx, err := d.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`INSERT INTO token_events (
		timestamp, provider, model, client_app,
		prompt_tokens, completion_tokens, total_tokens,
		cache_read_tokens, cache_write_tokens,
		cost_usd, shadow_cost, latency_ms, ttfb_ms, streaming_ms,
		status_code, project_id, tags, metadata, source,
		session_id, session_file, entry_id
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare insert: %w", err)
	}
	defer stmt.Close()

	for _, e := range events {
		ts := e.Timestamp.Format(time.RFC3339Nano)
		if e.Timestamp.IsZero() {
			ts = time.Now().UTC().Format(time.RFC3339Nano)
		}
		if _, err := stmt.Exec(
			ts, e.Provider, e.Model, e.ClientApp,
			e.PromptTokens, e.CompletionTokens, e.TotalTokens,
			e.CacheReadTokens, e.CacheWriteTokens,
			e.CostUSD, e.ShadowCost, e.LatencyMs, e.TTFBMs, e.StreamingMs,
			e.StatusCode, e.ProjectID, e.Tags, e.Metadata, e.Source,
			e.SessionID, e.SessionFile, e.EntryID,
		); err != nil {
			return fmt.Errorf("insert event batch: %w", err)
		}
	}

	return tx.Commit()
}

// DailyStats holds aggregated stats for a single day.
type DailyStats struct {
	Date            string  `json:"date"`
	TotalRequests   int     `json:"total_requests"`
	TotalTokens     int     `json:"total_tokens"`
	PromptTokens    int     `json:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens"`
	CacheReadTokens int     `json:"cache_read_tokens"`
	TotalCostUSD    float64 `json:"total_cost_usd"`
	ShadowCost      float64 `json:"shadow_cost"`
	AvgLatencyMs    float64 `json:"avg_latency_ms"`
}

// GetDailyStats returns per-day aggregated stats within a date range.
func (d *Database) GetDailyStats(from, to time.Time, projectID string) ([]DailyStats, error) {
	q := `SELECT
		date(timestamp) as date,
		COUNT(*) as total_requests,
		SUM(total_tokens) as total_tokens,
		SUM(prompt_tokens) as prompt_tokens,
		SUM(completion_tokens) as completion_tokens,
		SUM(cache_read_tokens) as cache_read_tokens,
		SUM(cost_usd) as total_cost_usd,
		SUM(shadow_cost) as shadow_cost,
		AVG(latency_ms) as avg_latency_ms
	FROM token_events
	WHERE timestamp >= ? AND timestamp < ?`
	args := []interface{}{from.UTC().Format(time.RFC3339), to.UTC().Format(time.RFC3339)}

	if projectID != "" {
		q += " AND project_id = ?"
		args = append(args, projectID)
	}
	q += " GROUP BY date(timestamp) ORDER BY date DESC"

	rows, err := d.db.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("daily stats: %w", err)
	}
	defer rows.Close()

	var stats []DailyStats
	for rows.Next() {
		var s DailyStats
		if err := rows.Scan(&s.Date, &s.TotalRequests, &s.TotalTokens,
			&s.PromptTokens, &s.CompletionTokens, &s.CacheReadTokens,
			&s.TotalCostUSD, &s.ShadowCost, &s.AvgLatencyMs); err != nil {
			return nil, fmt.Errorf("scan daily stats: %w", err)
		}
		stats = append(stats, s)
	}
	return stats, rows.Err()
}

// CostSummary holds aggregate cost data for budget checks.
type CostSummary struct {
	DailyCostUSD    float64 `json:"daily_cost_usd"`
	MonthlyCostUSD  float64 `json:"monthly_cost_usd"`
	DailyTokens     int     `json:"daily_tokens"`
	MonthlyTokens   int     `json:"monthly_tokens"`
}

// GetCostSummary returns current day and month cost/token totals for a project.
func (d *Database) GetCostSummary(projectID string) (CostSummary, error) {
	now := time.Now().UTC()
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)

	q := `SELECT
		COALESCE(SUM(CASE WHEN timestamp >= ? THEN cost_usd ELSE 0 END), 0),
		COALESCE(SUM(CASE WHEN timestamp >= ? THEN cost_usd ELSE 0 END), 0),
		COALESCE(SUM(CASE WHEN timestamp >= ? THEN total_tokens ELSE 0 END), 0),
		COALESCE(SUM(CASE WHEN timestamp >= ? THEN total_tokens ELSE 0 END), 0)
	FROM token_events WHERE project_id = ?`

	var s CostSummary
	err := d.db.QueryRow(q,
		dayStart.Format(time.RFC3339),
		monthStart.Format(time.RFC3339),
		dayStart.Format(time.RFC3339),
		monthStart.Format(time.RFC3339),
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
	Model           string  `json:"model"`
	Provider        string  `json:"provider"`
	TotalRequests   int     `json:"total_requests"`
	TotalTokens     int     `json:"total_tokens"`
	PromptTokens    int     `json:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens"`
	TotalCostUSD    float64 `json:"total_cost_usd"`
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
	TotalRequests     int     `json:"total_requests"`
	TotalTokens       int     `json:"total_tokens"`
	PromptTokens      int     `json:"prompt_tokens"`
	CompletionTokens  int     `json:"completion_tokens"`
	CacheReadTokens   int     `json:"cache_read_tokens"`
	TotalCostUSD      float64 `json:"total_cost_usd"`
	TotalShadowCost   float64 `json:"total_shadow_cost_usd"`
	AvgLatencyMs      float64 `json:"avg_latency_ms"`
}

// GetSummary returns aggregate stats with optional filters.
func (d *Database) GetSummary(since, project, source, tag string) (Summary, error) {
	q := `SELECT COALESCE(COUNT(*),0), COALESCE(SUM(total_tokens),0),
		COALESCE(SUM(prompt_tokens),0), COALESCE(SUM(completion_tokens),0),
		COALESCE(SUM(cache_read_tokens),0), COALESCE(SUM(cost_usd),0),
		COALESCE(SUM(shadow_cost),0), COALESCE(AVG(latency_ms),0)
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
	if source != "" && source != "all" {
		q += " AND source = ?"
		args = append(args, source)
	}
	if tag != "" && tag != "all" {
		q += " AND (',' || tags || ',') LIKE ?"
		args = append(args, "%,"+tag+",%")
	}
	var s Summary
	err := d.db.QueryRow(q, args...).Scan(&s.TotalRequests, &s.TotalTokens,
		&s.PromptTokens, &s.CompletionTokens, &s.CacheReadTokens,
		&s.TotalCostUSD, &s.TotalShadowCost, &s.AvgLatencyMs)
	return s, err
}

// ByModel holds per-model aggregated stats.
type ByModel struct {
	Model           string  `json:"model"`
	Provider        string  `json:"provider"`
	TotalRequests   int     `json:"requests"`
	TotalTokens     int     `json:"total_tokens"`
	PromptTokens    int     `json:"prompt_tokens"`
	CompletionToks  int     `json:"completion_tokens"`
	TotalCostUSD    float64 `json:"total_cost_usd"`
}

// GetSummaryByModel returns per-model aggregated stats.
func (d *Database) GetSummaryByModel(since, project, source string) ([]ByModel, error) {
	q := `SELECT model, provider, COALESCE(COUNT(*),0), COALESCE(SUM(total_tokens),0),
		COALESCE(SUM(prompt_tokens),0), COALESCE(SUM(completion_tokens),0), COALESCE(SUM(cost_usd),0)
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
	if source != "" && source != "all" {
		q += " AND source = ?"
		args = append(args, source)
	}
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
			&m.TotalTokens, &m.PromptTokens, &m.CompletionToks, &m.TotalCostUSD); err != nil {
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
	if source != "" && source != "all" {
		q += " AND source = ?"
		args = append(args, source)
	}
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
}

// GetSummaryByProject returns per-project aggregated stats.
func (d *Database) GetSummaryByProject(since, source string) ([]ByProject, error) {
	q := `SELECT project_id, COALESCE(COUNT(*),0), COALESCE(SUM(total_tokens),0), COALESCE(SUM(cost_usd),0)
		FROM token_events WHERE 1=1`
	args := []interface{}{}
	if since != "" {
		q += " AND timestamp >= ?"
		args = append(args, since)
	}
	if source != "" && source != "all" {
		q += " AND source = ?"
		args = append(args, source)
	}
	q += " GROUP BY project_id ORDER BY SUM(cost_usd) DESC"
	rows, err := d.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []ByProject
	for rows.Next() {
		var p ByProject
		if err := rows.Scan(&p.ProjectID, &p.TotalRequests, &p.TotalTokens, &p.TotalCostUSD); err != nil {
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
	if source != "" && source != "all" {
		q += " AND source = ?"
		args = append(args, source)
	}
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

// GetDailyUsage returns per-day usage for the last N days.
func (d *Database) GetDailyUsage(days int, since, source string) ([]DailyUsage, error) {
	q := `SELECT substr(timestamp,1,10) as day, COUNT(*), SUM(total_tokens), SUM(cost_usd)
		FROM token_events WHERE 1=1`
	args := []interface{}{}
	if since != "" {
		q += " AND timestamp >= ?"
		args = append(args, since)
	} else {
		cutoff := time.Now().UTC().AddDate(0, 0, -days).Format(time.RFC3339)
		q += " AND timestamp >= ?"
		args = append(args, cutoff)
	}
	if source != "" && source != "all" {
		q += " AND source = ?"
		args = append(args, source)
	}
	q += " GROUP BY day ORDER BY day ASC"
	rows, err := d.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []DailyUsage
	for rows.Next() {
		var du DailyUsage
		if err := rows.Scan(&du.Date, &du.TotalRequests, &du.TotalTokens, &du.CostUSD); err != nil {
			return nil, err
		}
		result = append(result, du)
	}
	return result, rows.Err()
}

// TpsSummary holds throughput summary.
type TpsSummary struct {
	AvgTPS    float64 `json:"avg_tps"`
	PeakTPS   float64 `json:"peak_tps"`
	TotalReqs int     `json:"total_requests"`
}

// GetTpsSummary returns throughput summary.
func (d *Database) GetTpsSummary(since, project, source string) (TpsSummary, error) {
	q := `SELECT COUNT(*), COALESCE(MAX(total_tokens/NULLIF(latency_ms/1000.0,0)),0)
		FROM token_events WHERE latency_ms > 0 AND 1=1`
	args := []interface{}{}
	if since != "" {
		q += " AND timestamp >= ?"
		args = append(args, since)
	}
	if project != "" && project != "all" {
		q += " AND project_id = ?"
		args = append(args, project)
	}
	if source != "" && source != "all" {
		q += " AND source = ?"
		args = append(args, source)
	}
	var s TpsSummary
	err := d.db.QueryRow(q, args...).Scan(&s.TotalReqs, &s.PeakTPS)
	if err != nil {
		return s, err
	}
	// avg_tps: total tokens / total time window in seconds
	q2 := `SELECT COALESCE(SUM(total_tokens),0),
		COALESCE((strftime('%s','now') - MIN(strftime('%s',timestamp))),1)
		FROM token_events WHERE 1=1`
	args2 := []interface{}{}
	if since != "" {
		q2 += " AND timestamp >= ?"
		args2 = append(args2, since)
	}
	if project != "" && project != "all" {
		q2 += " AND project_id = ?"
		args2 = append(args2, project)
	}
	if source != "" && source != "all" {
		q2 += " AND source = ?"
		args2 = append(args2, source)
	}
	var totalTok int
	var windowSecs float64
	if err := d.db.QueryRow(q2, args2...).Scan(&totalTok, &windowSecs); err == nil && windowSecs > 0 {
		s.AvgTPS = float64(totalTok) / windowSecs
	}
	return s, nil
}

// TpsSeriesPoint holds a single time-series bucket.
type TpsSeriesPoint struct {
	Timestamp string  `json:"timestamp"`
	AvgTPS    float64 `json:"avg_tps"`
	Requests  int     `json:"requests"`
	Tokens    int     `json:"tokens"`
}

// GetTpsSeries returns time-series throughput data.
func (d *Database) GetTpsSeries(since, interval, project, source string) ([]TpsSeriesPoint, error) {
	groupExpr := "strftime('%Y-%m-%dT%H:00:00Z', timestamp)"
	if interval == "1d" {
		groupExpr = "substr(timestamp,1,10)"
	} else if interval == "5m" {
		groupExpr = "strftime('%Y-%m-%dT%H:', timestamp) || printf('%02d', (CAST(strftime('%M', timestamp) AS INTEGER)/5)*5) || ':00Z'"
	}
	q := fmt.Sprintf(`SELECT %s as ts, COUNT(*), SUM(total_tokens),
		COALESCE(SUM(total_tokens)/NULLIF(COUNT(*)*3600.0/NULLIF(CAST(? AS REAL),0),0),0)
		FROM token_events WHERE 1=1`, groupExpr)
	args := []interface{}{float64(3600)} // default 1h bucket seconds
	if interval == "5m" {
		args[0] = float64(300)
	} else if interval == "1d" {
		args[0] = float64(86400)
	}
	if since != "" {
		q += " AND timestamp >= ?"
		args = append(args, since)
	}
	if project != "" && project != "all" {
		q += " AND project_id = ?"
		args = append(args, project)
	}
	if source != "" && source != "all" {
		q += " AND source = ?"
		args = append(args, source)
	}
	q += " GROUP BY ts ORDER BY ts ASC"
	rows, err := d.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []TpsSeriesPoint
	for rows.Next() {
		var p TpsSeriesPoint
		if err := rows.Scan(&p.Timestamp, &p.Requests, &p.Tokens, &p.AvgTPS); err != nil {
			return nil, err
		}
		result = append(result, p)
	}
	return result, rows.Err()
}

// LiveSession represents an active session.
type LiveSession struct {
	CLI          string  `json:"cli"`
	SessionID    string  `json:"session_id"`
	Provider     string  `json:"provider"`
	Model        string  `json:"model"`
	TokensInput  int     `json:"tokens_input"`
	TokensOutput int     `json:"tokens_output"`
	CostUSD      float64 `json:"cost_usd"`
	Project      string  `json:"project"`
	LastUpdated  string  `json:"last_updated"`
}

// GetLiveSessions returns sessions active in the last 10 seconds.
func (d *Database) GetLiveSessions() ([]LiveSession, error) {
	cutoff := time.Now().UTC().Add(-10 * time.Second).Format(time.RFC3339)
	q := `SELECT source, session_file, project_id, provider, model,
		SUM(prompt_tokens), SUM(completion_tokens), SUM(cost_usd), MAX(timestamp)
		FROM token_events
		WHERE timestamp != '' AND timestamp >= ?
		GROUP BY source, session_file, provider, model
		ORDER BY MAX(timestamp) DESC LIMIT 50`
	rows, err := d.db.Query(q, cutoff)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []LiveSession
	for rows.Next() {
		var ls LiveSession
		var src string
		if err := rows.Scan(&src, &ls.SessionID, &ls.Project, &ls.Provider, &ls.Model,
			&ls.TokensInput, &ls.TokensOutput, &ls.CostUSD, &ls.LastUpdated); err != nil {
			return nil, err
		}
		ls.CLI = src
		if idx := strings.Index(src, ":"); idx >= 0 {
			ls.CLI = src[idx+1:]
		}
		if ls.Project == "" {
			ls.Project = "default"
		}
		result = append(result, ls)
	}
	return result, rows.Err()
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
	if source != "" && source != "all" {
		q += " AND source = ?"
		args = append(args, source)
	}
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