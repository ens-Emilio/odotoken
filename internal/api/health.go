package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/ens-Emilio/odotoken/internal/db"
)

// RegisterHealthRoutes adds provider health endpoints to the given router.
func RegisterHealthRoutes(r chi.Router, database *db.Database) {
	r.Get("/provider-health", func(w http.ResponseWriter, r *http.Request) {
		health, err := database.GetProviderHealth()
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, "error.provider_health", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, health)
	})
}