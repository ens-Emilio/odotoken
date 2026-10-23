package api

import (
	"encoding/json"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/user/odotoken/internal/db"
)

// DashboardRouter creates the /api router for dashboard endpoints.
func DashboardRouter(database *db.Database) chi.Router {
	r := chi.NewRouter()

	r.Get("/stats", func(w http.ResponseWriter, r *http.Request) {
		since := resolveSince(r.URL.Query().Get("since"))
		project := r.URL.Query().Get("project")
		source := r.URL.Query().Get("source")
		tag := r.URL.Query().Get("tag")

		summary, _ := database.GetSummary(since, project, source, tag)
		byModel, _ := database.GetSummaryByModel(since, project, source)
		byClient, _ := database.GetSummaryByClient(since, project, source)
		byProject, _ := database.GetSummaryByProject(since, source)
		bySource, _ := database.GetSummaryBySource(since, project)
		byTag, _ := database.GetSummaryByTag(since, project, source)
		recentEvents, _ := database.RecentEventsFiltered(50, since, project, source, tag)
		daily, _ := database.GetDailyUsage(30, since, source)
 liveSessions, _ := database.GetLiveSessions()
 tps, _ := database.GetTpsSummary(since, project, source)

		// Normalize summary to match frontend Summary interface
		summaryMap := map[string]interface{}{
			"total_requests":          summary.TotalRequests,
			"total_prompt_tokens":     summary.PromptTokens,
			"total_completion_tokens": summary.CompletionTokens,
			"total_tokens":            summary.TotalTokens,
			"total_cache_read_tokens": summary.CacheReadTokens,
			"total_cache_write_tokens": 0,
			"total_cost_usd":          summary.TotalCostUSD,
			"avg_latency_ms":          summary.AvgLatencyMs,
			"avg_ttfb_ms":             0.0,
		}

		// Normalize by_model to match ModelRow interface
		modelRows := make([]map[string]interface{}, 0, len(byModel))
		for _, m := range byModel {
			modelRows = append(modelRows, map[string]interface{}{
				"model":             m.Model,
				"provider":          m.Provider,
				"requests":          m.TotalRequests,
				"prompt_tokens":     m.PromptTokens,
				"completion_tokens": m.CompletionToks,
				"total_tokens":      m.TotalTokens,
				"cache_read_tokens": 0,
				"cache_write_tokens": 0,
				"cost_usd":          m.TotalCostUSD,
			})
		}

		// Normalize by_client to match ClientRow interface
		clientRows := make([]map[string]interface{}, 0, len(byClient))
		for _, c := range byClient {
			clientRows = append(clientRows, map[string]interface{}{
				"client_app":   c.ClientApp,
				"requests":     c.TotalRequests,
				"total_tokens": c.TotalTokens,
				"cost_usd":     c.TotalCostUSD,
			})
		}

		// Normalize by_project to match ProjectRow interface
		projectRows := make([]map[string]interface{}, 0, len(byProject))
		for _, p := range byProject {
			projectRows = append(projectRows, map[string]interface{}{
				"project_id":     p.ProjectID,
				"requests":       p.TotalRequests,
				"total_tokens":   p.TotalTokens,
				"cost_usd":       p.TotalCostUSD,
				"shadow_cost_usd": 0.0,
			})
		}

		// Normalize by_source to match SourceRow interface
		sourceRows := make([]map[string]interface{}, 0, len(bySource))
		for _, s := range bySource {
			sourceRows = append(sourceRows, map[string]interface{}{
				"source":       s.Source,
				"requests":     s.TotalRequests,
				"total_tokens": s.TotalTokens,
				"cost_usd":     s.TotalCostUSD,
			})
		}

		// Normalize by_tag to match TagRow interface
		tagRows := make([]map[string]interface{}, 0, len(byTag))
		for _, t := range byTag {
			tagRows = append(tagRows, map[string]interface{}{
				"tag":          t.Tag,
				"requests":     t.TotalRequests,
				"total_tokens": t.TotalTokens,
				"cost_usd":     t.TotalCostUSD,
			})
		}

		// Normalize recent_events to match EventRow interface
		eventRows := make([]map[string]interface{}, 0, len(recentEvents))
		for _, e := range recentEvents {
			eventRows = append(eventRows, map[string]interface{}{
				"id":                e.ID,
				"timestamp":         e.Timestamp.Format(time.RFC3339Nano),
				"provider":          e.Provider,
				"model":             e.Model,
				"client_app":        e.ClientApp,
				"status_code":       e.StatusCode,
				"prompt_tokens":     e.PromptTokens,
				"total_tokens":      e.TotalTokens,
				"completion_tokens": e.CompletionTokens,
				"cache_read_tokens": e.CacheReadTokens,
				"cache_write_tokens": e.CacheWriteTokens,
				"cost_usd":          e.CostUSD,
				"latency_ms":        e.LatencyMs,
				"ttfb_ms":           e.TTFBMs,
				"streaming_ms":      e.StreamingMs,
				"tags":              e.Tags,
			})
		}

		// Normalize daily to match DailyRow interface (uses "day" not "date")
		dailyRows := make([]map[string]interface{}, 0, len(daily))
		for _, d := range daily {
			dailyRows = append(dailyRows, map[string]interface{}{
				"day":          d.Date,
				"requests":     d.TotalRequests,
				"total_tokens": d.TotalTokens,
				"cost_usd":     d.CostUSD,
			})
		}

		// Normalize tps to match TpsSummary interface
		tpsMap := map[string]interface{}{
			"requests":           tps.TotalReqs,
			"completion_tokens":  0,
			"total_tokens":       0,
			"total_latency_ms":   0.0,
			"avg_latency_ms":     0.0,
			"avg_tps":            tps.AvgTPS,
			"peak_tps":           tps.PeakTPS,
		}

		savings := map[string]float64{
			"actual_usd": summary.TotalCostUSD,
			"shadow_usd": summary.TotalShadowCost,
			"saved_usd":  math.Max(0, summary.TotalShadowCost-summary.TotalCostUSD),
		}

		// Normalize live_sessions to match LiveSession interface
		sessionRows := make([]map[string]interface{}, 0, len(liveSessions))
		for _, s := range liveSessions {
			sessionRows = append(sessionRows, map[string]interface{}{
				"cli":           s.CLI,
				"session_id":    s.SessionID,
				"provider":      s.Provider,
				"model":         s.Model,
				"tokens_input":  s.TokensInput,
				"tokens_output": s.TokensOutput,
				"cost_usd":      s.CostUSD,
				"project":       s.Project,
				"title":         "",
				"last_updated":  s.LastUpdated,
			})
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"summary":        summaryMap,
			"by_model":       modelRows,
			"by_client":      clientRows,
			"by_project":     projectRows,
			"by_source":      sourceRows,
			"by_tag":         tagRows,
			"recent_events":  eventRows,
			"live_sessions":  sessionRows,
			"daily":          dailyRows,
			"tps":            tpsMap,
			"savings":        savings,
			"guard": map[string]interface{}{
				"limits": map[string]interface{}{
					"daily_cost_limit_usd":   0.0,
					"monthly_cost_limit_usd": 0.0,
					"daily_tokens_limit":     0,
					"monthly_tokens_limit":   0,
					"action_on_breach":       "warn",
				},
				"status":  "ok",
				"allowed": true,
				"reason":  "",
			},
		})
	})

	r.Get("/events", func(w http.ResponseWriter, r *http.Request) {
		since := r.URL.Query().Get("since")
		// Default to last 10 seconds if not specified (matches "live · 10s" badge)
		if since == "" {
			since = time.Now().UTC().Add(-10 * time.Second).Format(time.RFC3339)
		} else {
			since = resolveSince(since)
		}
		project := r.URL.Query().Get("project")
		source := r.URL.Query().Get("source")
		tag := r.URL.Query().Get("tag")
		limit := 50
		if l := r.URL.Query().Get("limit"); l != "" {
			if n, err := strconv.Atoi(l); err == nil && n > 0 {
				limit = n
			}
		}
		events, _ := database.RecentEventsFiltered(limit, since, project, source, tag)
		
		// Normalize to match EventRow interface
		eventRows := make([]map[string]interface{}, 0, len(events))
		for _, e := range events {
			eventRows = append(eventRows, map[string]interface{}{
				"id":                e.ID,
				"timestamp":         e.Timestamp.Format(time.RFC3339Nano),
				"provider":          e.Provider,
				"model":             e.Model,
				"client_app":        e.ClientApp,
				"status_code":       e.StatusCode,
				"prompt_tokens":     e.PromptTokens,
				"total_tokens":      e.TotalTokens,
				"completion_tokens": e.CompletionTokens,
				"cache_read_tokens": e.CacheReadTokens,
				"cache_write_tokens": e.CacheWriteTokens,
				"cost_usd":          e.CostUSD,
				"latency_ms":        e.LatencyMs,
				"ttfb_ms":           e.TTFBMs,
				"streaming_ms":      e.StreamingMs,
				"tags":              e.Tags,
			})
		}
		writeJSON(w, http.StatusOK, eventRows)
	})

	r.Get("/tps", func(w http.ResponseWriter, r *http.Request) {
		since := r.URL.Query().Get("since")
		interval := r.URL.Query().Get("interval")
		if interval == "" {
			interval = "1h"
		}
		project := r.URL.Query().Get("project")
		source := r.URL.Query().Get("source")
		series, _ := database.GetTpsSeries(since, interval, project, source)
		summary, _ := database.GetTpsSummary(since, project, source)
		liveTPS := 0.0
		for i := len(series) - 1; i >= 0; i-- {
			if series[i].AvgTPS > 0 {
				liveTPS = series[i].AvgTPS
				break
			}
		}

		// Normalize series to match TpsBucket interface
		seriesRows := make([]map[string]interface{}, 0, len(series))
		for _, s := range series {
			seriesRows = append(seriesRows, map[string]interface{}{
				"bucket":           s.Timestamp,
				"requests":         s.Requests,
				"total_tokens":     s.Tokens,
				"completion_tokens": 0,
				"total_latency_ms": 0.0,
				"avg_latency_ms":   0.0,
				"avg_tps":          s.AvgTPS,
				"peak_tps":         0.0,
			})
		}

		// Normalize summary to match TpsSummary interface
		summaryMap := map[string]interface{}{
			"requests":           summary.TotalReqs,
			"completion_tokens":  0,
			"total_tokens":       0,
			"total_latency_ms":   0.0,
			"avg_latency_ms":     0.0,
			"avg_tps":            summary.AvgTPS,
			"peak_tps":           summary.PeakTPS,
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"series":     seriesRows,
			"summary":    summaryMap,
			"by_model":   []map[string]interface{}{},
			"by_project": []map[string]interface{}{},
			"live_tps":   liveTPS,
			"interval":   interval,
		})
	})

	r.Get("/forecast", func(w http.ResponseWriter, r *http.Request) {
		tag := r.URL.Query().Get("tag")
		daily, _ := database.GetDailyUsage(30, "", "")
		costs := make([]float64, len(daily))
		for i, d := range daily {
			costs[i] = d.CostUSD
		}
		n := len(costs)
		var a, b float64
		if n >= 2 {
			mx, my := 0.0, 0.0
			for i, c := range costs {
				mx += float64(i)
				my += c
			}
			mx /= float64(n)
			my /= float64(n)
			denom := 0.0
			num := 0.0
			for i, c := range costs {
				dx := float64(i) - mx
				denom += dx * dx
				num += dx * (c - my)
			}
			if denom > 0 {
				a = num / denom
				b = my - a*mx
			}
		} else if n == 1 {
			b = costs[0]
		}
		lastDay := time.Now().UTC().Format("2006-01-02")
		if len(daily) > 0 {
			lastDay = daily[len(daily)-1].Date
		}
		base, err := time.Parse("2006-01-02", lastDay)
		if err != nil {
			base = time.Now().UTC()
		}
		forecast := make([]map[string]interface{}, 7)
		for i := range 7 {
			d := base.AddDate(0, 0, i+1).Format("2006-01-02")
			val := math.Max(0, a*float64(n+i)+b)
			forecast[i] = map[string]interface{}{
				"day":      d,
				"cost_usd": roundTo(val, 6),
			}
		}
		anomalies := []map[string]interface{}{}
		if n >= 3 {
			mean := 0.0
			for _, c := range costs {
				mean += c
			}
			mean /= float64(n)
			variance := 0.0
			for _, c := range costs {
				variance += (c - mean) * (c - mean)
			}
			variance /= float64(n)
			std := math.Sqrt(variance)
			thresh := mean + 2*std
			for _, d := range daily {
				if d.CostUSD > thresh && thresh > 0 {
					anomalies = append(anomalies, map[string]interface{}{
						"day":       d.Date,
						"cost_usd":  d.CostUSD,
						"threshold": roundTo(thresh, 6),
					})
				}
			}
		}

		// Normalize daily to match DailyRow interface for Forecast
		dailyRows := make([]map[string]interface{}, 0, len(daily))
		for _, d := range daily {
			dailyRows = append(dailyRows, map[string]interface{}{
				"day":          d.Date,
				"requests":     d.TotalRequests,
				"total_tokens": d.TotalTokens,
				"cost_usd":     d.CostUSD,
			})
		}

		_ = tag
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"forecast":  forecast,
			"anomalies": anomalies,
			"daily":     dailyRows,
		})
	})

	r.Get("/sessions/live", func(w http.ResponseWriter, r *http.Request) {
		sessions, _ := database.GetLiveSessions()
		
		// Normalize to match LiveSession interface
		sessionRows := make([]map[string]interface{}, 0, len(sessions))
		for _, s := range sessions {
			sessionRows = append(sessionRows, map[string]interface{}{
				"cli":          s.CLI,
				"session_id":   s.SessionID,
				"provider":     s.Provider,
				"model":        s.Model,
				"tokens_input": s.TokensInput,
				"tokens_output": s.TokensOutput,
				"cost_usd":     s.CostUSD,
				"project":      s.Project,
				"title":        "",
				"last_updated": s.LastUpdated,
			})
		}
		writeJSON(w, http.StatusOK, sessionRows)
	})

	return r
}

func resolveSince(s string) string {
	if s == "" || s == "all" {
		return ""
	}
	now := time.Now().UTC()
	
	// Handle relative formats: "today", "7d", "30d"
	if s == "today" {
		return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC).Format(time.RFC3339)
	}
	if len(s) > 1 && s[len(s)-1] == 'd' {
		if days, err := strconv.Atoi(s[:len(s)-1]); err == nil {
			return now.AddDate(0, 0, -days).Format(time.RFC3339)
		}
	}
	
	// Handle ISO8601 dates sent by frontend (e.g., "2026-10-16T00:00:00.000Z")
	// Try parsing as time. If successful, re-format to RFC3339 (seconds precision)
	// to ensure consistent comparison with SQLite timestamps which might lack sub-second precision.
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t.UTC().Format(time.RFC3339)
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.UTC().Format(time.RFC3339)
	}
	
	// Fallback: return as-is (might be a valid SQLite date string)
	return s
}

func roundTo(val float64, places int) float64 {
	mul := 1.0
	for range places {
		mul *= 10
	}
	return float64(int(val*mul+0.5)) / mul
}

// Ensure json is used.
var _ = json.Marshal