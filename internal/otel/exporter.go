package otel

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/ens-Emilio/odotoken/internal/config"
	"github.com/ens-Emilio/odotoken/internal/db"
)

// Exporter pushes usage totals to an OTLP/HTTP collector as JSON metrics.
//
// The payload is the subset of the OTLP spec a collector accepts for sums:
// resourceMetrics -> scopeMetrics -> metrics -> sum. Nothing is invented —
// fields the spec requires (timeUnixNano, asDouble/asInt, aggregation
// temporality, isMonotonic) are all present, and the collector validates them.
type Exporter struct {
	db       *db.Database
	cfg      config.OtelConfig
	client   *http.Client
	logger   *slog.Logger
	lastSent map[string]float64
}

// New builds an exporter. A disabled or endpoint-less config returns nil so the
// daemon can skip the goroutine entirely instead of polling for nothing.
func New(database *db.Database, cfg config.OtelConfig, logger *slog.Logger) *Exporter {
	if !cfg.Enabled || strings.TrimSpace(cfg.Endpoint) == "" {
		return nil
	}
	return &Exporter{
		db:       database,
		cfg:      cfg,
		client:   &http.Client{Timeout: 15 * time.Second},
		logger:   logger,
		lastSent: map[string]float64{},
	}
}

// Endpoint is the exact URL exported to, so doctor can print what it will hit.
func (e *Exporter) Endpoint() string {
	return strings.TrimRight(e.cfg.Endpoint, "/") + "/v1/metrics"
}

// Interval is the configured push cadence with a sane floor.
func (e *Exporter) Interval() time.Duration {
	seconds := e.cfg.IntervalSeconds
	if seconds <= 0 {
		seconds = 60
	}
	return time.Duration(seconds) * time.Second
}

// Start pushes once immediately and then on the interval until ctx is done.
func (e *Exporter) Start(ctx context.Context) {
	ticker := time.NewTicker(e.Interval())
	defer ticker.Stop()
	for {
		if err := e.Export(ctx); err != nil {
			e.logger.Warn("otel export failed", "endpoint", e.Endpoint(), "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// Export sends one OTLP payload built from the cumulative counters.
func (e *Exporter) Export(ctx context.Context) error {
	body, err := e.Payload()
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.Endpoint(), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := e.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("collector answered HTTP %d", resp.StatusCode)
	}
	return nil
}

// Payload builds the OTLP/JSON document. Cumulative sums are reported for
// requests, tokens and cost per project, plus the queued-alert gauge.
func (e *Exporter) Payload() ([]byte, error) {
	series, err := e.db.MetricsSnapshot()
	if err != nil {
		return nil, fmt.Errorf("metrics snapshot: %w", err)
	}
	now := fmt.Sprintf("%d", time.Now().UnixNano())

	byProject := map[string]struct{ requests, tokens, cost float64 }{}
	for _, s := range series {
		agg := byProject[s.Project]
		agg.requests += float64(s.Requests)
		agg.tokens += float64(s.Prompt + s.Completion)
		agg.cost += s.CostUSD
		byProject[s.Project] = agg
	}

	sum := func(name string, unit string, points []map[string]interface{}) map[string]interface{} {
		return map[string]interface{}{
			"name": name,
			"unit": unit,
			"sum": map[string]interface{}{
				"aggregationTemporality": 2, // cumulative
				"isMonotonic":            true,
				"dataPoints":             points,
			},
		}
	}
	point := func(project string, value float64) map[string]interface{} {
		return map[string]interface{}{
			"asDouble":     value,
			"timeUnixNano": now,
			"attributes": []map[string]interface{}{{
				"key":   "project",
				"value": map[string]interface{}{"stringValue": project},
			}},
		}
	}

	var requests, tokens, cost []map[string]interface{}
	for project, agg := range byProject {
		requests = append(requests, point(project, agg.requests))
		tokens = append(tokens, point(project, agg.tokens))
		cost = append(cost, point(project, agg.cost))
	}

	metrics := []map[string]interface{}{
		sum("odotoken.requests", "1", requests),
		sum("odotoken.tokens", "1", tokens),
		sum("odotoken.cost_usd", "USD", cost),
	}
	if pending, err := e.db.PendingAlertCount(); err == nil {
		metrics = append(metrics, map[string]interface{}{
			"name": "odotoken.alerts.pending",
			"unit": "1",
			"gauge": map[string]interface{}{
				"dataPoints": []map[string]interface{}{{
					"asInt":        fmt.Sprintf("%d", pending),
					"timeUnixNano": now,
				}},
			},
		})
	}

	doc := map[string]interface{}{
		"resourceMetrics": []map[string]interface{}{{
			"resource": map[string]interface{}{
				"attributes": []map[string]interface{}{{
					"key":   "service.name",
					"value": map[string]interface{}{"stringValue": "odotoken"},
				}},
			},
			"scopeMetrics": []map[string]interface{}{{
				"scope":   map[string]interface{}{"name": "odotoken"},
				"metrics": metrics,
			}},
		}},
	}
	return json.Marshal(doc)
}

// Status reports what doctor needs to show without contacting the collector.
func (e *Exporter) Status() (endpoint string, interval time.Duration) {
	return e.Endpoint(), e.Interval()
}
