package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/ens-Emilio/odotoken/internal/config"
	"github.com/ens-Emilio/odotoken/internal/db"
)

// Doctor must report the OTLP export without contacting the collector: a
// disabled config is informational, an enabled one prints the exact URL and
// cadence the daemon will use, and an enabled-but-endpointless config is the
// one state that would silently export nothing.
func TestCheckOtelReportsConfigWithoutCollector(t *testing.T) {
	database, err := db.Open(filepath.Join(t.TempDir(), "doctor.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer database.Close()

	for _, tc := range []struct {
		name       string
		cfg        config.OtelConfig
		wantStatus string
		wantDetail string
	}{
		{"disabled", config.OtelConfig{}, "ok", "disabled"},
		{
			"enabled",
			config.OtelConfig{Enabled: true, Endpoint: "http://127.0.0.1:4318/", IntervalSeconds: 30},
			"ok",
			"http://127.0.0.1:4318/v1/metrics",
		},
		{"enabled without endpoint", config.OtelConfig{Enabled: true}, "warn", "endpoint is empty"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			report := &doctorReport{}
			checkOtel(report, config.Config{Otel: tc.cfg}, database)
			if len(report.Checks) != 1 {
				t.Fatalf("checks = %+v, want one otel check", report.Checks)
			}
			check := report.Checks[0]
			if check.Name != "otel" || check.Status != tc.wantStatus {
				t.Fatalf("check = %+v, want name otel and status %s", check, tc.wantStatus)
			}
			if !strings.Contains(check.Detail, tc.wantDetail) {
				t.Fatalf("detail = %q, want it to contain %q", check.Detail, tc.wantDetail)
			}
		})
	}
}
