package otel

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/ens-Emilio/odotoken/internal/config"
	"github.com/ens-Emilio/odotoken/internal/db"
	"github.com/ens-Emilio/odotoken/internal/domain"
)

func testDB(t *testing.T) *db.Database {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "otel.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return database
}

// A disabled or endpoint-less config must leave no exporter behind: the daemon
// decides whether to start the goroutine from this nil.
func TestNewReturnsNilWhenDisabledOrEndpointless(t *testing.T) {
	database := testDB(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	for _, tc := range []struct {
		name string
		cfg  config.OtelConfig
	}{
		{"disabled", config.OtelConfig{Enabled: false, Endpoint: "http://127.0.0.1:4318"}},
		{"no endpoint", config.OtelConfig{Enabled: true}},
		{"blank endpoint", config.OtelConfig{Enabled: true, Endpoint: "   "}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := New(database, tc.cfg, logger); got != nil {
				t.Fatalf("New(%+v) = %+v, want nil", tc.cfg, got)
			}
		})
	}
}

type otlpAttribute struct {
	Key   string `json:"key"`
	Value struct {
		StringValue string `json:"stringValue"`
	} `json:"value"`
}

type otlpDataPoint struct {
	AsDouble     float64         `json:"asDouble"`
	AsInt        string          `json:"asInt"`
	TimeUnixNano string          `json:"timeUnixNano"`
	Attributes   []otlpAttribute `json:"attributes"`
}

type otlpMetric struct {
	Name string `json:"name"`
	Unit string `json:"unit"`
	Sum  *struct {
		AggregationTemporality int             `json:"aggregationTemporality"`
		IsMonotonic            bool            `json:"isMonotonic"`
		DataPoints             []otlpDataPoint `json:"dataPoints"`
	} `json:"sum"`
	Gauge *struct {
		DataPoints []otlpDataPoint `json:"dataPoints"`
	} `json:"gauge"`
}

type otlpDocument struct {
	ResourceMetrics []struct {
		Resource struct {
			Attributes []otlpAttribute `json:"attributes"`
		} `json:"resource"`
		ScopeMetrics []struct {
			Scope   struct{ Name string } `json:"scope"`
			Metrics []otlpMetric          `json:"metrics"`
		} `json:"scopeMetrics"`
	} `json:"resourceMetrics"`
}

// Export must be a real OTLP/HTTP push: POST to <endpoint>/v1/metrics with a
// JSON body whose counters match the rows the database actually holds.
func TestExportPostsOTLPMetricsFromDatabase(t *testing.T) {
	database := testDB(t)
	for _, e := range []domain.TokenEvent{
		{
			Timestamp: time.Now().UTC(), ProjectID: "alpha", Provider: "openai", Model: "gpt-4o",
			PromptTokens: 100, CompletionTokens: 50, TotalTokens: 150, CostUSD: 1.25, StatusCode: 200,
		},
		{
			Timestamp: time.Now().UTC(), ProjectID: "alpha", Provider: "openai", Model: "gpt-4o",
			PromptTokens: 10, CompletionTokens: 40, TotalTokens: 50, CostUSD: 0.75, StatusCode: 200,
		},
		{
			Timestamp: time.Now().UTC(), ProjectID: "beta", Provider: "anthropic", Model: "claude",
			PromptTokens: 7, CompletionTokens: 3, TotalTokens: 10, CostUSD: 2, StatusCode: 200,
		},
	} {
		if _, err := database.InsertEvent(e); err != nil {
			t.Fatalf("insert event: %v", err)
		}
	}

	type request struct {
		method      string
		path        string
		contentType string
		body        []byte
	}
	received := make(chan request, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		received <- request{
			method:      r.Method,
			path:        r.URL.Path,
			contentType: r.Header.Get("Content-Type"),
			body:        body,
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	exporter := New(database, config.OtelConfig{Enabled: true, Endpoint: srv.URL + "/"}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if exporter == nil {
		t.Fatal("New returned nil for an enabled config")
	}
	if err := exporter.Export(context.Background()); err != nil {
		t.Fatalf("export: %v", err)
	}

	req := <-received
	if req.method != http.MethodPost {
		t.Errorf("method = %s, want POST", req.method)
	}
	if req.path != "/v1/metrics" {
		t.Errorf("path = %s, want /v1/metrics", req.path)
	}
	if req.contentType != "application/json" {
		t.Errorf("content-type = %q, want application/json", req.contentType)
	}

	var doc otlpDocument
	if err := json.Unmarshal(req.body, &doc); err != nil {
		t.Fatalf("decode OTLP body: %v", err)
	}
	if len(doc.ResourceMetrics) != 1 || len(doc.ResourceMetrics[0].ScopeMetrics) != 1 {
		t.Fatalf("resourceMetrics/scopeMetrics shape = %+v", doc.ResourceMetrics)
	}

	// Per-project values: alpha has two events, beta one.
	values := map[string]map[string]float64{}
	pending := ""
	for _, m := range doc.ResourceMetrics[0].ScopeMetrics[0].Metrics {
		switch {
		case m.Sum != nil:
			for _, p := range m.Sum.DataPoints {
				project := ""
				for _, a := range p.Attributes {
					if a.Key == "project" {
						project = a.Value.StringValue
					}
				}
				if values[m.Name] == nil {
					values[m.Name] = map[string]float64{}
				}
				values[m.Name][project] = p.AsDouble
			}
		case m.Name == "odotoken.alerts.pending" && m.Gauge != nil && len(m.Gauge.DataPoints) == 1:
			pending = m.Gauge.DataPoints[0].AsInt
		}
	}

	want := map[string]map[string]float64{
		"odotoken.requests": {"alpha": 2, "beta": 1},
		"odotoken.tokens":   {"alpha": 200, "beta": 10},
		"odotoken.cost_usd": {"alpha": 2, "beta": 2},
	}
	for metric, projects := range want {
		for project, value := range projects {
			if got := values[metric][project]; got != value {
				t.Errorf("%s[%s] = %v, want %v (all: %+v)", metric, project, got, value, values[metric])
			}
		}
	}
	if pending != "0" {
		t.Errorf("odotoken.alerts.pending = %q, want \"0\"", pending)
	}
}

// A collector that rejects the payload is a failed export, not a silent one:
// the outbox-style retry only exists if the error surfaces.
func TestExportReportsCollectorFailure(t *testing.T) {
	database := testDB(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	exporter := New(database, config.OtelConfig{Enabled: true, Endpoint: srv.URL}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err := exporter.Export(context.Background()); err == nil {
		t.Fatal("export against a 503 collector returned nil, want an error")
	}
}
