package db

import "time"

// ProviderHealth holds aggregated health metrics for a single provider.
type ProviderHealth struct {
	Provider      string  `json:"provider"`
	TotalRequests int     `json:"total_requests"`
	ErrorCount    int     `json:"error_count"`
	AvgLatencyMs  float64 `json:"avg_latency_ms"`
	P95LatencyMs  float64 `json:"p95_latency_ms"`
	Uptime24h     float64 `json:"uptime_24h"`
	Uptime7d      float64 `json:"uptime_7d"`
	Uptime30d     float64 `json:"uptime_30d"`
}

// GetProviderHealth returns per-provider health metrics from token_events.
func (d *Database) GetProviderHealth() ([]ProviderHealth, error) {
	now := time.Now().UTC()
	cutoff24h := now.Add(-24 * time.Hour).Format(time.RFC3339)
	cutoff7d := now.Add(-7 * 24 * time.Hour).Format(time.RFC3339)
	cutoff30d := now.Add(-30 * 24 * time.Hour).Format(time.RFC3339)

	query := `
		SELECT
			provider,
			COUNT(*) AS total_requests,
			SUM(CASE WHEN status_code >= 400 THEN 1 ELSE 0 END) AS error_count,
			AVG(latency_ms) AS avg_latency_ms,
			MAX(latency_ms) AS p95_latency_ms,
			CAST(SUM(CASE WHEN timestamp >= ? AND status_code < 400 THEN 1 ELSE 0 END) AS REAL) /
				NULLIF(SUM(CASE WHEN timestamp >= ? THEN 1 ELSE 0 END), 0) * 100 AS uptime_24h,
			CAST(SUM(CASE WHEN timestamp >= ? AND status_code < 400 THEN 1 ELSE 0 END) AS REAL) /
				NULLIF(SUM(CASE WHEN timestamp >= ? THEN 1 ELSE 0 END), 0) * 100 AS uptime_7d,
			CAST(SUM(CASE WHEN timestamp >= ? AND status_code < 400 THEN 1 ELSE 0 END) AS REAL) /
				NULLIF(SUM(CASE WHEN timestamp >= ? THEN 1 ELSE 0 END), 0) * 100 AS uptime_30d
		FROM token_events
		GROUP BY provider
		ORDER BY total_requests DESC
	`

	rows, err := d.db.Query(query, cutoff24h, cutoff24h, cutoff7d, cutoff7d, cutoff30d, cutoff30d)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []ProviderHealth
	for rows.Next() {
		var h ProviderHealth
		var avgLatency interface{}
		var maxLatency interface{}
		var u24, u7, u30 interface{}
		if err := rows.Scan(
			&h.Provider,
			&h.TotalRequests,
			&h.ErrorCount,
			&avgLatency,
			&maxLatency,
			&u24,
			&u7,
			&u30,
		); err != nil {
			return nil, err
		}
		h.AvgLatencyMs = toFloat(avgLatency)
		h.P95LatencyMs = toFloat(maxLatency)
		h.Uptime24h = toFloat(u24)
		h.Uptime7d = toFloat(u7)
		h.Uptime30d = toFloat(u30)
		results = append(results, h)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if results == nil {
		results = []ProviderHealth{}
	}
	return results, nil
}

func toFloat(v interface{}) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int64:
		return float64(n)
	case nil:
		return 0
	default:
		return 0
	}
}