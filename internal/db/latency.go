package db

import (
	"sort"
)

// LatencyDistribution holds percentile stats and histogram buckets for a provider.
type LatencyDistribution struct {
	Provider string          `json:"provider"`
	P50      float64         `json:"p50"`
	P95      float64         `json:"p95"`
	P99      float64         `json:"p99"`
	Buckets  []LatencyBucket `json:"buckets"`
	Total    int             `json:"total"`
}

var latencyBounds = []float64{100, 250, 500, 1000, 2500, 5000, 10000, 30000, 60000}

// GetLatencyDistribution queries token_events for the given provider since the
// given timestamp and computes percentiles + histogram buckets in Go.
func (d *Database) GetLatencyDistribution(provider, since string) (LatencyDistribution, error) {
	rows, err := d.db.Query(
		`SELECT latency_ms FROM token_events WHERE provider = ? AND timestamp >= ?`,
		provider, since,
	)
	if err != nil {
		return LatencyDistribution{}, err
	}
	defer rows.Close()

	var latencies []float64
	for rows.Next() {
		var v float64
		if err := rows.Scan(&v); err != nil {
			return LatencyDistribution{}, err
		}
		latencies = append(latencies, v)
	}
	if err := rows.Err(); err != nil {
		return LatencyDistribution{}, err
	}

	total := len(latencies)
	dist := LatencyDistribution{
		Provider: provider,
		Total:    total,
	}
	if total == 0 {
		dist.Buckets = make([]LatencyBucket, 0, len(latencyBounds))
		for _, le := range latencyBounds {
			dist.Buckets = append(dist.Buckets, LatencyBucket{Le: le, Count: 0})
		}
		return dist, nil
	}

	sort.Float64s(latencies)
	dist.P50 = percentile(latencies, 0.50)
	dist.P95 = percentile(latencies, 0.95)
	dist.P99 = percentile(latencies, 0.99)

	dist.Buckets = make([]LatencyBucket, 0, len(latencyBounds))
	for _, le := range latencyBounds {
		count := sort.SearchFloat64s(latencies, le+1) // <= le
		dist.Buckets = append(dist.Buckets, LatencyBucket{Le: le, Count: int64(count)})
	}
	return dist, nil
}

func percentile(sorted []float64, p float64) float64 {
	n := len(sorted)
	idx := int(p * float64(n-1))
	if idx < 0 {
		idx = 0
	}
	if idx >= n {
		idx = n - 1
	}
	return sorted[idx]
}