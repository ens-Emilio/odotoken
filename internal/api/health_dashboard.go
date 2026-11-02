package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/ens-Emilio/odotoken/internal/db"
)

// HealthStatus represents the current system health for the dashboard.
type HealthStatus struct {
	Status      string    `json:"status"`
	Uptime      string    `json:"uptime"`
	DBHealthy   bool      `json:"db_healthy"`
	LastCheck   time.Time `json:"last_check"`
	Version     string    `json:"version"`
	ActiveKeys  int       `json:"active_keys"`
	TotalEvents int64     `json:"total_events"`
}

var startTime = time.Now()

// ServeHealthDashboard returns a JSON health status for the self-hosted dashboard.
func ServeHealthDashboard(database *db.Database, version string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		status := HealthStatus{
			Status:    "ok",
			Uptime:    time.Since(startTime).Round(time.Second).String(),
			DBHealthy: true,
			LastCheck: time.Now().UTC(),
			Version:   version,
		}

		// Check DB health via provider health endpoint
		if _, err := database.GetProviderHealth(); err != nil {
			status.Status = "degraded"
			status.DBHealthy = false
		}

		// Get basic stats from snapshot
		if metrics, err := database.MetricsSnapshot(); err == nil {
			for _, m := range metrics {
				status.TotalEvents += m.Requests
			}
		}

		w.Header().Set("Content-Type", "application/json")
		// Do not set Access-Control-Allow-Origin: * by default; self-hosted health
		// endpoints should not be publicly accessible without explicit configuration.
		json.NewEncoder(w).Encode(status)
	}
}