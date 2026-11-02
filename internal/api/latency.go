package api

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/ens-Emilio/odotoken/internal/db"
)

// RegisterLatencyRoutes mounts the latency-distribution endpoint on the given router.
func RegisterLatencyRoutes(r chi.Router, database *db.Database) {
	r.Get("/latency-distribution", func(w http.ResponseWriter, req *http.Request) {
		provider := req.URL.Query().Get("provider")
		sinceStr := req.URL.Query().Get("since")
		if sinceStr == "" {
			sinceStr = "24h"
		}
		dur, err := time.ParseDuration(sinceStr)
		if err != nil {
			http.Error(w, "invalid since duration", http.StatusBadRequest)
			return
		}
		since := time.Now().Add(-dur).UTC().Format(time.RFC3339Nano)

		dist, err := database.GetLatencyDistribution(provider, since)
		if err != nil {
			http.Error(w, "query failed", http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, dist)
	})
}