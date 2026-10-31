package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/ens-Emilio/odotoken/internal/auth"
	"github.com/ens-Emilio/odotoken/internal/config"
)

// RegisterClientKeyRoutes mounts /client-keys. The store owns persistence, so
// the handlers stay thin and the CLI sees the same data.
func RegisterClientKeyRoutes(r chi.Router, store *auth.Store) {
	r.Get("/client-keys", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"keys": store.List()})
	})

	r.Put("/client-keys", func(w http.ResponseWriter, req *http.Request) {
		var body struct {
			Key                 string  `json:"key"`
			Client              string  `json:"client"`
			Project             string  `json:"project"`
			DailyCostLimitUSD   float64 `json:"daily_cost_limit_usd"`
			MonthlyCostLimitUSD float64 `json:"monthly_cost_limit_usd"`
			DailyTokenLimit     int     `json:"daily_tokens_limit"`
			MonthlyTokenLimit   int     `json:"monthly_tokens_limit"`
		}
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			writeAPIError(w, http.StatusBadRequest, "error.clientKeys.invalid", "invalid JSON body")
			return
		}
		plaintext, entry, err := store.Set(strings.TrimSpace(body.Key), config.ClientKeyConfig{
			Client:              strings.TrimSpace(body.Client),
			Project:             strings.TrimSpace(body.Project),
			DailyCostLimitUSD:   body.DailyCostLimitUSD,
			MonthlyCostLimitUSD: body.MonthlyCostLimitUSD,
			DailyTokenLimit:     body.DailyTokenLimit,
			MonthlyTokenLimit:   body.MonthlyTokenLimit,
		})
		switch {
		case errors.Is(err, auth.ErrMissingIdentity):
			writeAPIError(w, http.StatusBadRequest, "error.clientKeys.identity", "client and project are required")
			return
		case errors.Is(err, auth.ErrNegativeLimit):
			writeAPIError(w, http.StatusBadRequest, "error.clientKeys.limit", "limits must be non-negative")
			return
		case errors.Is(err, auth.ErrNotFound):
			writeAPIError(w, http.StatusNotFound, "error.clientKeys.unavailable", "client key not found")
			return
		case err != nil:
			writeAPIError(w, http.StatusInternalServerError, "error.clientKeys.persist", "could not persist client key")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"key":                    entry.Key,
			"client":                 entry.Client,
			"project":                entry.Project,
			"daily_cost_limit_usd":   entry.DailyCostLimitUSD,
			"monthly_cost_limit_usd": entry.MonthlyCostLimitUSD,
			"daily_tokens_limit":     entry.DailyTokenLimit,
			"monthly_tokens_limit":   entry.MonthlyTokenLimit,
			"plaintext_key":          plaintext,
		})
	})

	r.Delete("/client-keys/{key}", func(w http.ResponseWriter, req *http.Request) {
		err := store.Delete(chi.URLParam(req, "key"))
		switch {
		case errors.Is(err, auth.ErrNotFound):
			writeAPIError(w, http.StatusNotFound, "error.clientKeys.unavailable", "client key not found")
		case err != nil:
			writeAPIError(w, http.StatusInternalServerError, "error.clientKeys.persist", "could not persist client key")
		default:
			w.WriteHeader(http.StatusNoContent)
		}
	})
}
