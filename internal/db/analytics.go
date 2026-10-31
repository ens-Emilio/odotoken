package db

import (
	"fmt"
	"time"
)

// SessionCost is the tenant-neutral rollup of one conversation. Proxy requests
// share a session through X-Odo-Session; imported sessions fall back to their
// source file when the upstream format has no session identifier.
type SessionCost struct {
	SessionID        string  `json:"session_id"`
	SessionFile      string  `json:"session_file,omitempty"`
	ClientApp        string  `json:"client_app"`
	ProjectID        string  `json:"project_id"`
	Models           string  `json:"models"`
	Requests         int     `json:"requests"`
	PromptTokens     int     `json:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens"`
	TotalTokens      int     `json:"total_tokens"`
	CacheReadTokens  int     `json:"cache_read_tokens"`
	CacheWriteTokens int     `json:"cache_write_tokens"`
	CostUSD          float64 `json:"cost_usd"`
	StartedAt        string  `json:"started_at"`
	EndedAt          string  `json:"ended_at"`
}

// SessionListFilter bounds an operator-facing session listing. Limit is part of
// the contract so a large database cannot be pulled into one dashboard payload.
type SessionListFilter struct {
	Since   string
	Project string
	Source  string
	Tag     string
	Limit   int
	Offset  int
}

const sessionKey = `COALESCE(NULLIF(session_id, ''), NULLIF(session_file, ''), 'event:' || id)`

// SessionCosts groups the same token_events rows already used by the dashboard.
// No second accounting table means session cost cannot drift from event cost.
func (d *Database) SessionCosts(f SessionListFilter) ([]SessionCost, error) {
	if f.Limit <= 0 {
		f.Limit = 100
	}
	if f.Limit > 1000 {
		f.Limit = 1000
	}
	q := `SELECT ` + sessionKey + ` AS session_key,
		COALESCE(MAX(NULLIF(session_file, '')), '') AS session_file,
		MAX(client_app), MAX(project_id), group_concat(DISTINCT model),
		COUNT(*), SUM(prompt_tokens), SUM(completion_tokens), SUM(total_tokens),
		SUM(cache_read_tokens), SUM(cache_write_tokens), SUM(cost_usd),
		MIN(timestamp), MAX(timestamp)
		FROM token_events WHERE 1=1`
	q, args := applyAnalyticsEventFilter(q, nil, f.Since, f.Project, f.Source, f.Tag)
	q += ` GROUP BY session_key ORDER BY MAX(timestamp) DESC LIMIT ? OFFSET ?`
	args = append(args, f.Limit, f.Offset)
	rows, err := d.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]SessionCost, 0)
	for rows.Next() {
		var s SessionCost
		if err := rows.Scan(&s.SessionID, &s.SessionFile, &s.ClientApp, &s.ProjectID,
			&s.Models, &s.Requests, &s.PromptTokens, &s.CompletionTokens,
			&s.TotalTokens, &s.CacheReadTokens, &s.CacheWriteTokens, &s.CostUSD,
			&s.StartedAt, &s.EndedAt); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// SessionEvents returns the rows belonging to one already resolved key. A
// caller must pass the exact key returned by SessionCosts; this avoids SQL
// interpolation and keeps both endpoints on one definition of a session.
func (d *Database) SessionEvents(key string, limit int) ([]map[string]interface{}, error) {
	if key == "" {
		return nil, fmt.Errorf("empty session id")
	}
	if limit <= 0 || limit > 5000 {
		limit = 500
	}
	q := `SELECT id, timestamp, provider, model, client_app, project_id,
		prompt_tokens, completion_tokens, total_tokens, cache_read_tokens,
		cache_write_tokens, cost_usd, latency_ms, status_code
		FROM token_events WHERE ` + sessionKey + ` = ? ORDER BY timestamp, id LIMIT ?`
	rows, err := d.db.Query(q, key, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]map[string]interface{}, 0)
	for rows.Next() {
		var id int64
		var ts, provider, model, client, project string
		var prompt, completion, total, cacheRead, cacheWrite, status int
		var cost, latency float64
		if err := rows.Scan(&id, &ts, &provider, &model, &client, &project,
			&prompt, &completion, &total, &cacheRead, &cacheWrite, &cost,
			&latency, &status); err != nil {
			return nil, err
		}
		out = append(out, map[string]interface{}{
			"id": id, "timestamp": ts, "provider": provider, "model": model,
			"client_app": client, "project_id": project,
			"prompt_tokens": prompt, "completion_tokens": completion,
			"total_tokens": total, "cache_read_tokens": cacheRead,
			"cache_write_tokens": cacheWrite, "cost_usd": cost,
			"latency_ms": latency, "status_code": status,
		})
	}
	return out, rows.Err()
}

// CacheTokenRow carries only the facts needed to reprice an event as if all
// prompt tokens had been sent without cache reads.
type CacheTokenRow struct {
	Model            string  `json:"model"`
	Provider         string  `json:"provider"`
	Requests         int     `json:"requests"`
	PromptTokens     int     `json:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens"`
	CacheReadTokens  int     `json:"cache_read_tokens"`
	CacheWriteTokens int     `json:"cache_write_tokens"`
	ActualCostUSD    float64 `json:"actual_cost_usd"`
}

func (d *Database) CacheTokenRows(since, project, source, tag string) ([]CacheTokenRow, error) {
	q := `SELECT model, provider, COUNT(*), SUM(prompt_tokens),
		SUM(completion_tokens), SUM(cache_read_tokens), SUM(cache_write_tokens),
		SUM(cost_usd) FROM token_events WHERE 1=1`
	q, args := applyAnalyticsEventFilter(q, nil, since, project, source, tag)
	q += ` GROUP BY model, provider ORDER BY SUM(cost_usd) DESC`
	rows, err := d.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]CacheTokenRow, 0)
	for rows.Next() {
		var r CacheTokenRow
		if err := rows.Scan(&r.Model, &r.Provider, &r.Requests, &r.PromptTokens,
			&r.CompletionTokens, &r.CacheReadTokens, &r.CacheWriteTokens,
			&r.ActualCostUSD); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// DailyDimensionCost is one model or project's share of one day's spend.
type DailyDimensionCost struct {
	Day      string  `json:"day"`
	Key      string  `json:"key"`
	Requests int     `json:"requests"`
	Tokens   int     `json:"tokens"`
	CostUSD  float64 `json:"cost_usd"`
}

// DailyModelCosts and DailyProjectCosts feed anomaly attribution. Both use the
// same local-day expression as the forecast's daily series.
func (d *Database) DailyModelCosts(project, source, tag, tz string) ([]DailyDimensionCost, error) {
	return d.dailyDimensionCosts("model", project, source, tag, tz)
}

func (d *Database) DailyProjectCosts(source, tag, tz string) ([]DailyDimensionCost, error) {
	return d.dailyDimensionCosts("project_id", "", source, tag, tz)
}

func (d *Database) dailyDimensionCosts(column, project, source, tag, tz string) ([]DailyDimensionCost, error) {
	if column != "model" && column != "project_id" {
		return nil, fmt.Errorf("unsupported dimension %q", column)
	}
	loc := loadAnalyticsLocation(tz)
	dayExpr := analyticsDayExpr(loc)
	q := `SELECT ` + dayExpr + ` AS day, COALESCE(NULLIF(` + column + `, ''), 'unknown') AS key_value,
		COUNT(*), SUM(COALESCE(total_tokens, prompt_tokens + completion_tokens)),
		SUM(cost_usd) FROM token_events WHERE 1=1`
	q, args := applyAnalyticsEventFilter(q, nil, "", project, source, tag)
	q += ` GROUP BY day, key_value ORDER BY day, SUM(cost_usd) DESC`
	rows, err := d.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]DailyDimensionCost, 0)
	for rows.Next() {
		var r DailyDimensionCost
		if err := rows.Scan(&r.Day, &r.Key, &r.Requests, &r.Tokens, &r.CostUSD); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ClientAttributionUsage is used by per-client API-key quotas. client_app is the
// stable synthetic attribution written by the key middleware; grouping by the
// same column keeps quota and dashboard attribution identical.
func (d *Database) ClientAttributionUsage(clientApp, since string) (requests, tokens int, cost float64, err error) {
	err = d.db.QueryRow(`SELECT COUNT(*), COALESCE(SUM(total_tokens), 0),
		COALESCE(SUM(cost_usd), 0) FROM token_events
		WHERE client_app = ? AND timestamp >= ?`, clientApp, since).
		Scan(&requests, &tokens, &cost)
	return
}

// EventsForExport is deliberately bounded. An unbounded CSV is a denial of
// service against a metering database and a surprising way to fill a disk.
func (d *Database) EventsForExport(since, until, project, source, tag string, limit int) ([]map[string]interface{}, bool, error) {
	if limit <= 0 {
		limit = 10000
	}
	if limit > 100000 {
		limit = 100000
	}
	// Fetch one extra row to report truncation without a second count query.
	rows, err := d.queryAnalyticsEvents(since, until, project, source, tag, limit+1)
	if err != nil {
		return nil, false, err
	}
	truncated := len(rows) > limit
	if truncated {
		rows = rows[:limit]
	}
	return rows, truncated, nil
}

func (d *Database) queryAnalyticsEvents(since, until, project, source, tag string, limit int) ([]map[string]interface{}, error) {
	q := `SELECT id, timestamp, provider, model, client_app, project_id,
		session_id, run_id, source, status_code, prompt_tokens, completion_tokens,
		total_tokens, cache_read_tokens, cache_write_tokens, cost_usd, shadow_cost,
		latency_ms, ttfb_ms, streaming_ms, tags, metadata
		FROM token_events WHERE 1=1`
	q, args := applyAnalyticsEventFilter(q, nil, since, project, source, tag)
	if until != "" {
		q += " AND timestamp <= ?"
		args = append(args, until)
	}
	q += ` ORDER BY id DESC LIMIT ?`
	args = append(args, limit)
	rows, err := d.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]map[string]interface{}, 0)
	for rows.Next() {
		var id int64
		var ts, provider, model, client, projectID, sessionID, runID, sourceName string
		var status, prompt, completion, total, cacheRead, cacheWrite int
		var cost, shadow, latency, ttfb, streaming float64
		var tags, metadata string
		if err := rows.Scan(&id, &ts, &provider, &model, &client, &projectID,
			&sessionID, &runID, &sourceName, &status, &prompt, &completion, &total,
			&cacheRead, &cacheWrite, &cost, &shadow, &latency, &ttfb, &streaming,
			&tags, &metadata); err != nil {
			return nil, err
		}
		out = append(out, map[string]interface{}{
			"id": id, "timestamp": ts, "provider": provider, "model": model,
			"client_app": client, "project_id": projectID, "session_id": sessionID,
			"run_id": runID, "source": sourceName, "status_code": status,
			"prompt_tokens": prompt, "completion_tokens": completion,
			"total_tokens": total, "cache_read_tokens": cacheRead,
			"cache_write_tokens": cacheWrite, "cost_usd": cost,
			"shadow_cost": shadow, "latency_ms": latency, "ttfb_ms": ttfb,
			"streaming_ms": streaming, "tags": tags, "metadata": metadata,
		})
	}
	return out, rows.Err()
}

func applyAnalyticsEventFilter(q string, args []interface{}, since, project, source, tag string) (string, []interface{}) {
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
		args = append(args, "%"+tag+"%")
	}
	return q, args
}

func loadAnalyticsLocation(tz string) *time.Location {
	if tz == "" {
		return time.UTC
	}
	if loc, err := time.LoadLocation(tz); err == nil {
		return loc
	}
	return time.UTC
}

// analyticsDayExpr mirrors the dashboard's local-day bucketing. SQLite has no
// IANA database, so the offset for the current instant is applied in SQL; the
// dashboard and anomaly view can therefore label the same day consistently.
func analyticsDayExpr(loc *time.Location) string {
	if loc == time.UTC {
		return "substr(timestamp, 1, 10)"
	}
	_, offset := time.Now().In(loc).Zone()
	sign := "+"
	if offset < 0 {
		sign = "-"
		offset = -offset
	}
	return fmt.Sprintf("substr(datetime(timestamp, '%s%02d:%02d'), 1, 10)", sign, offset/3600, (offset%3600)/60)
}
