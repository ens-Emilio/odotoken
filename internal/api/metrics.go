package api

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/ens-Emilio/odotoken/internal/db"
)

// MetricsRouter serves the Prometheus text exposition of the counters that
// matter for operating odotoken: spend, tokens, requests and alert backlog.
// Everything is derived from SQLite, so a restart does not reset the numbers.
func MetricsRouter(database *db.Database) chi.Router {
	r := chi.NewRouter()
	r.Get("/", func(w http.ResponseWriter, req *http.Request) {
		writeMetrics(w, database)
	})
	return r
}

var latencyBounds = []float64{100, 250, 500, 1000, 2500, 5000, 10000, 30000, 60000}

func writeMetrics(w http.ResponseWriter, database *db.Database) {
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

	b.WriteString("# HELP odotoken_alert_outbox_pending Queued alerts awaiting delivery.\n")
	b.WriteString("# TYPE odotoken_alert_outbox_pending gauge\n")
	if pending, err := database.PendingAlertCount(); err == nil {
		fmt.Fprintf(&b, "odotoken_alert_outbox_pending %d\n", pending)
	}

	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(b.String()))
}

// escapeLabel escapes the three characters Prometheus forbids raw in a label.
func escapeLabel(v string) string {
	return strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`).Replace(v)
}
