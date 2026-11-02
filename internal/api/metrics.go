package api

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/ens-Emilio/odotoken/internal/db"
	"github.com/ens-Emilio/odotoken/internal/guard"
)

// MetricsRouter serves the Prometheus text exposition of the counters that
// matter for operating odotoken: spend, tokens, requests and alert backlog.
// Everything is derived from SQLite, so a restart does not reset the numbers.
func MetricsRouter(database *db.Database, sloTracker *guard.SLOTracker) chi.Router {
	r := chi.NewRouter()
	r.Get("/", func(w http.ResponseWriter, req *http.Request) {
		writeMetrics(w, database, sloTracker)
	})
	return r
}

var latencyBounds = []float64{100, 250, 500, 1000, 2500, 5000, 10000, 30000, 60000}

func writeMetrics(w http.ResponseWriter, database *db.Database, sloTracker *guard.SLOTracker) {
	var b strings.Builder
	b.WriteString("# HELP odotoken_requests_total Requests recorded, by provider, model, project and status.\n")
	b.WriteString("# TYPE odotoken_requests_total counter\n")
	b.WriteString("# HELP odotoken_tokens_total Tokens recorded, by kind.\n")
	b.WriteString("# TYPE odotoken_tokens_total counter\n")
	b.WriteString("# HELP odotoken_cost_usd_total Recorded cost in USD.\n")
	b.WriteString("# TYPE odotoken_cost_usd_total counter\n")

	series, err := database.MetricsSnapshot()
	if err != nil {
		http.Error(w, "metrics unavailable", http.StatusInternalServerError)
		return
	}
	for _, s := range series {
		labels := fmt.Sprintf(`provider="%s",model="%s",project="%s",status="%d"`,
			escapeLabel(s.Provider), escapeLabel(s.Model), escapeLabel(s.Project), s.Status)
		fmt.Fprintf(&b, "odotoken_requests_total{%s} %d\n", labels, s.Requests)
		fmt.Fprintf(&b, "odotoken_tokens_total{%s,kind=\"prompt\"} %d\n", labels, s.Prompt)
		fmt.Fprintf(&b, "odotoken_tokens_total{%s,kind=\"completion\"} %d\n", labels, s.Completion)
		fmt.Fprintf(&b, "odotoken_tokens_total{%s,kind=\"cache_read\"} %d\n", labels, s.CacheRead)
		fmt.Fprintf(&b, "odotoken_cost_usd_total{%s} %g\n", labels, s.CostUSD)
	}

	b.WriteString("# HELP odotoken_request_latency_ms Request latency histogram.\n")
	b.WriteString("# TYPE odotoken_request_latency_ms histogram\n")
	buckets, total, err := database.LatencyBuckets(latencyBounds)
	if err == nil {
		for _, bucket := range buckets {
			fmt.Fprintf(&b, "odotoken_request_latency_ms_bucket{le=%q} %d\n", fmt.Sprint(bucket.Le), bucket.Count)
		}
		fmt.Fprintf(&b, "odotoken_request_latency_ms_bucket{le=\"+Inf\"} %d\n", total)
		fmt.Fprintf(&b, "odotoken_request_latency_ms_count %d\n", total)
	}

		b.WriteString("# HELP odotoken_client_request_latency_ms Request latency histogram per client_app.\n")
		b.WriteString("# TYPE odotoken_client_request_latency_ms histogram\n")
		clientBuckets, clientStats, err := database.ClientLatencyBuckets(latencyBounds)
		if err == nil {
			for _, cb := range clientBuckets {
				labels := fmt.Sprintf(`client="%s",le="%s"`, escapeLabel(cb.ClientApp), fmt.Sprint(cb.Le))
				fmt.Fprintf(&b, "odotoken_client_request_latency_ms_bucket{%s} %d\n", labels, cb.Count)
			}
			// Emit +Inf bucket, _sum and _count per client to make the histogram valid for percentile queries.
			statsByClient := make(map[string]db.ClientLatencyStats, len(clientStats))
			for _, s := range clientStats {
				statsByClient[s.ClientApp] = s
			}
			seen := make(map[string]bool)
			for _, cb := range clientBuckets {
				if seen[cb.ClientApp] {
					continue
				}
				seen[cb.ClientApp] = true
				clientLabel := fmt.Sprintf(`client="%s"`, escapeLabel(cb.ClientApp))
				fmt.Fprintf(&b, "odotoken_client_request_latency_ms_bucket{%s,le=\"+Inf\"} %d\n", clientLabel, cb.Count)
				if s, ok := statsByClient[cb.ClientApp]; ok {
					fmt.Fprintf(&b, "odotoken_client_request_latency_ms_sum{%s} %g\n", clientLabel, s.SumMs)
					fmt.Fprintf(&b, "odotoken_client_request_latency_ms_count{%s} %d\n", clientLabel, s.Count)
				}
			}
		}

	b.WriteString("# HELP odotoken_alert_outbox_pending Queued alerts awaiting delivery.\n")
	b.WriteString("# TYPE odotoken_alert_outbox_pending gauge\n")
	if pending, err := database.PendingAlertCount(); err == nil {
		fmt.Fprintf(&b, "odotoken_alert_outbox_pending %d\n", pending)
	}

	b.WriteString("# HELP odotoken_guard_dry_run_blocks_total Requests allowed through in dry-run mode that would have been blocked.\n")
	b.WriteString("# TYPE odotoken_guard_dry_run_blocks_total counter\n")

	b.WriteString("# HELP odotoken_token_drift_ratio Latest token drift ratio per model.\n")
	b.WriteString("# TYPE odotoken_token_drift_ratio gauge\n")
	b.WriteString("# HELP odotoken_token_drift_alerts_total Cumulative drift alert transitions.\n")
	b.WriteString("# TYPE odotoken_token_drift_alerts_total counter\n")
	fmt.Fprintf(&b, "odotoken_token_drift_alerts_total %d\n", guard.GetDriftAlertsTotal())

	if sloTracker != nil {
		b.WriteString("# HELP odotoken_slo_burn_rate Error budget burn rate over last 1h.\n")
		b.WriteString("# TYPE odotoken_slo_burn_rate gauge\n")
		b.WriteString("# HELP odotoken_slo_budget_exceeded_total Times SLO burn rate exceeded threshold.\n")
		b.WriteString("# TYPE odotoken_slo_budget_exceeded_total counter\n")
		for _, p := range sloTracker.Providers() {
			burnRate, _ := sloTracker.Check(p)
			fmt.Fprintf(&b, "odotoken_slo_burn_rate{provider=\"%s\"} %g\n", escapeLabel(p), burnRate)
			fmt.Fprintf(&b, "odotoken_slo_budget_exceeded_total{provider=\"%s\"} %d\n", escapeLabel(p), sloTracker.BurnExceededTotal(p))
		}
	}

	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(b.String()))
}

// escapeLabel escapes the three characters Prometheus forbids raw in a label.
func escapeLabel(v string) string {
	return strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`).Replace(v)
}
