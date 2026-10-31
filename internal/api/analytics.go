package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/ens-Emilio/odotoken/internal/analytics"
	"github.com/ens-Emilio/odotoken/internal/db"
	"github.com/ens-Emilio/odotoken/internal/pricing"
)

// AnalyticsRouter creates an isolated router for callers that need one.
func AnalyticsRouter(database *db.Database, calc *pricing.Calculator, pricesPath string) chi.Router {
	r := chi.NewRouter()
	RegisterAnalyticsRoutes(r, database, calc, pricesPath)
	return r
}

// RegisterAnalyticsRoutes adds analytics endpoints to an existing router.
func RegisterAnalyticsRoutes(r chi.Router, database *db.Database, calc *pricing.Calculator, pricesPath string) {
	store := pricing.NewStore(pricesPath, calc)

	r.Get("/events.csv", func(w http.ResponseWriter, req *http.Request) {
		since := resolveSince(req.URL.Query().Get("since"))
		until := resolveSince(req.URL.Query().Get("until"))
		project := req.URL.Query().Get("project")
		source := req.URL.Query().Get("source")
		tag := req.URL.Query().Get("tag")
		limit := 0
		if raw := req.URL.Query().Get("limit"); raw != "" {
			n, err := strconv.Atoi(raw)
			if err != nil || n <= 0 {
				writeAPIError(w, http.StatusBadRequest, "error.export.limit", "limit must be a positive integer")
				return
			}
			limit = n
		}
		rows, truncated, err := database.EventsForExport(since, until, project, source, tag, limit)
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, "error.stats.unavailable", "event export unavailable")
			return
		}
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="odotoken-events.csv"`)
		if truncated {
			// A truncated export must say so in-band: a spreadsheet user cannot
			// see a response header.
			w.Header().Set("X-Odo-Truncated", "true")
		}
		if err := analytics.WriteEventsCSV(w, rows); err != nil {
			// Headers are already sent; the connection is the only signal left.
			return
		}
	})

	r.Get("/sessions", func(w http.ResponseWriter, req *http.Request) {
		filter := db.SessionListFilter{
			Since:   resolveSince(req.URL.Query().Get("since")),
			Project: req.URL.Query().Get("project"),
			Source:  req.URL.Query().Get("source"),
			Tag:     req.URL.Query().Get("tag"),
		}
		if raw := req.URL.Query().Get("limit"); raw != "" {
			n, err := strconv.Atoi(raw)
			if err != nil || n <= 0 {
				writeAPIError(w, http.StatusBadRequest, "error.sessions.limit", "limit must be a positive integer")
				return
			}
			filter.Limit = n
		}
		if raw := req.URL.Query().Get("offset"); raw != "" {
			n, err := strconv.Atoi(raw)
			if err != nil || n < 0 {
				writeAPIError(w, http.StatusBadRequest, "error.sessions.offset", "offset must be zero or positive")
				return
			}
			filter.Offset = n
		}
		sessions, err := database.SessionCosts(filter)
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, "error.sessions.unavailable", "sessions unavailable")
			return
		}
		writeJSON(w, http.StatusOK, sessions)
	})

	r.Get("/sessions/{id}", func(w http.ResponseWriter, req *http.Request) {
		key := chi.URLParam(req, "id")
		events, err := database.SessionEvents(key, 500)
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, "error.sessions.unavailable", "session unavailable")
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"session_id": key,
			"events":     events,
		})
	})

	r.Get("/cache-savings", func(w http.ResponseWriter, req *http.Request) {
		rows, err := database.CacheTokenRows(
			resolveSince(req.URL.Query().Get("since")),
			req.URL.Query().Get("project"),
			req.URL.Query().Get("source"),
			req.URL.Query().Get("tag"),
		)
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, "error.cache.unavailable", "cache savings unavailable")
			return
		}
		writeJSON(w, http.StatusOK, analytics.ComputeCacheSavings(rows, calc))
	})

	r.Post("/simulate", func(w http.ResponseWriter, req *http.Request) {
		var body struct {
			Requests         int      `json:"requests"`
			PromptTokens     int      `json:"prompt_tokens"`
			CompletionTokens int      `json:"completion_tokens"`
			CacheReadTokens  int      `json:"cache_read_tokens"`
			CacheWriteTokens int      `json:"cache_write_tokens"`
			Models           []string `json:"models"`
			Baseline         string   `json:"baseline"`
			Limit            int      `json:"limit"`
		}
		dec := json.NewDecoder(http.MaxBytesReader(w, req.Body, 1<<20))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&body); err != nil {
			writeAPIError(w, http.StatusBadRequest, "error.request.invalidJson", "invalid JSON")
			return
		}
		if body.Requests < 0 || body.PromptTokens < 0 || body.CompletionTokens < 0 ||
			body.CacheReadTokens < 0 || body.CacheWriteTokens < 0 {
			writeAPIError(w, http.StatusBadRequest, "error.simulate.negative", "token counts must be non-negative")
			return
		}
		rows, baseline := analytics.Simulate(calc, analytics.SimulationInput{
			Requests:         body.Requests,
			PromptTokens:     body.PromptTokens,
			CompletionTokens: body.CompletionTokens,
			CacheReadTokens:  body.CacheReadTokens,
			CacheWriteTokens: body.CacheWriteTokens,
		}, body.Models, body.Baseline, body.Limit)
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"baseline": baseline,
			"input":    body,
			"rows":     rows,
		})
	})

	r.Get("/prices", func(w http.ResponseWriter, req *http.Request) {
		if err := store.LoadError(); err != nil {
			writeAPIError(w, http.StatusInternalServerError, "error.prices.unreadable", "price catalog is unreadable")
			return
		}
		limit := 0
		if raw := req.URL.Query().Get("limit"); raw != "" {
			n, err := strconv.Atoi(raw)
			if err != nil || n <= 0 {
				writeAPIError(w, http.StatusBadRequest, "error.prices.limit", "limit must be a positive integer")
				return
			}
			limit = n
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"path":   store.Path(),
			"prices": store.List(req.URL.Query().Get("q"), limit),
		})
	})

	r.Put("/prices/{model}", func(w http.ResponseWriter, req *http.Request) {
		model := chi.URLParam(req, "model")
		var rates pricing.ModelRates
		dec := json.NewDecoder(http.MaxBytesReader(w, req.Body, 1<<16))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&rates); err != nil {
			writeAPIError(w, http.StatusBadRequest, "error.request.invalidJson", "invalid JSON")
			return
		}
		row, err := store.Set(model, rates)
		if err != nil {
			writeAPIError(w, http.StatusBadRequest, "error.prices.invalid", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, row)
	})

	r.Delete("/prices/{model}", func(w http.ResponseWriter, req *http.Request) {
		if err := store.Delete(chi.URLParam(req, "model")); err != nil {
			if errors.Is(err, pricing.ErrPriceNotFound) {
				writeAPIError(w, http.StatusNotFound, "error.prices.notFound", "no override for this model")
				return
			}
			writeAPIError(w, http.StatusInternalServerError, "error.prices.persist", "could not persist the catalog")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	r.Get("/anomalies", func(w http.ResponseWriter, req *http.Request) {
		project := req.URL.Query().Get("project")
		source := req.URL.Query().Get("source")
		tag := req.URL.Query().Get("tag")
		tz := req.URL.Query().Get("tz")
		daily, err := database.GetDailyUsage(45, "", source, tz)
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, "error.stats.unavailable", "daily usage unavailable")
			return
		}
		byModel, err := database.DailyModelCosts(project, source, tag, tz)
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, "error.stats.unavailable", "model attribution unavailable")
			return
		}
		byProject, err := database.DailyProjectCosts(source, tag, tz)
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, "error.stats.unavailable", "project attribution unavailable")
			return
		}
		anomalies := analytics.DetectAnomalies(daily, byModel, byProject)
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"anomalies":    anomalies,
			"generated_at": time.Now().UTC().Format(time.RFC3339),
		})
	})
}
