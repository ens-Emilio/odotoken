package api

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ens-Emilio/odotoken/internal/config"
	"github.com/ens-Emilio/odotoken/internal/db"
	"github.com/ens-Emilio/odotoken/internal/domain"
	"github.com/ens-Emilio/odotoken/internal/importers"
	"github.com/go-chi/chi/v5"
)

// DashboardRouter creates the /api router for dashboard endpoints.
func DashboardRouter(
	database *db.Database,
	guardChecker func(string) domain.GuardCheckResult,
	limits func() config.GuardConfig,
	updateLimits func(config.GuardConfig),
	configPath string,
	engine *importers.Engine,
) chi.Router {
	return dashboardRouter(database, guardChecker, limits, updateLimits, configPath, engine)
}

func dashboardRouter(
	database *db.Database,
	guardChecker func(string) domain.GuardCheckResult,
	getLimits func() config.GuardConfig,
	updateLimits func(config.GuardConfig),
	configPath string,
	engine *importers.Engine,
) chi.Router {
	r := chi.NewRouter()

	if engine != nil {
		registerImportRoutes(r, database, engine, configPath)
	}
	limitsView := func(v config.GuardConfig) map[string]interface{} {
		limit := func(value float64) interface{} {
			if value == 0 {
				return nil
			}
			return value
		}
		tokens := func(value int) interface{} {
			if value == 0 {
				return nil
			}
			return value
		}
		return map[string]interface{}{
			"daily_cost_limit_usd":   limit(v.DailyCostLimitUSD),
			"monthly_cost_limit_usd": limit(v.MonthlyCostLimitUSD),
			"daily_tokens_limit":     tokens(v.DailyTokenLimit),
			"monthly_tokens_limit":   tokens(v.MonthlyTokenLimit),
			"warning_threshold_pct":  limit(v.WarningThresholdPct),
			"action_on_breach":       v.ActionOnBreach,
			"timezone":               v.Timezone,
		}
	}
	budgetStatus := func(project string) map[string]interface{} {
		status := map[string]interface{}{
			"limits": limitsView(getLimits()),
			"current": map[string]interface{}{
				"daily_cost_usd":   0.0,
				"monthly_cost_usd": 0.0,
				"daily_tokens":     0,
				"monthly_tokens":   0,
			},
			"status":  "disabled",
			"allowed": true,
			"reason":  "",
		}
		if guardChecker == nil {
			return status
		}
		res := guardChecker(project)
		status["limits"] = limitsView(getLimits())
		status["current"] = map[string]interface{}{
			"daily_cost_usd":   res.CurrentDailyCost,
			"monthly_cost_usd": res.CurrentMonthlyCost,
			"daily_tokens":     res.CurrentDailyTokens,
			"monthly_tokens":   res.CurrentMonthTokens,
		}
		status["status"] = res.Status
		status["allowed"] = res.Allowed
		status["reason"] = res.Message
		status["reason_code"] = res.ReasonCode
		status["reason_params"] = res.ReasonParams
		return status
	}

	r.Get("/guard/limits", func(w http.ResponseWriter, r *http.Request) {
		if getLimits == nil {
			writeAPIError(w, http.StatusNotFound, "error.guard.unavailable", "guard unavailable")
			return
		}
		writeJSON(w, http.StatusOK, limitsView(getLimits()))
	})

	r.Get("/guard/status", func(w http.ResponseWriter, r *http.Request) {
		if guardChecker == nil {
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"status": "disabled", "allowed": true, "reason": "",
			})
			return
		}
		res := guardChecker(r.URL.Query().Get("project"))
		events, err := database.RecentGuardEvents(20)
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, "error.guard.history", "guard history unavailable")
			return
		}
		if events == nil {
			events = []db.GuardEvent{}
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"status":        res.Status,
			"allowed":       res.Allowed,
			"reason":        res.Message,
			"reason_code":   res.ReasonCode,
			"reason_params": res.ReasonParams,
			"history":       events,
		})
	})

	r.Patch("/guard/limits", func(w http.ResponseWriter, r *http.Request) {
		if getLimits == nil || updateLimits == nil {
			writeAPIError(w, http.StatusNotFound, "error.guard.unavailable", "guard unavailable")
			return
		}
		var raw map[string]json.RawMessage
		dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&raw); err != nil {
			writeAPIError(w, http.StatusBadRequest, "error.request.invalidJson", "invalid JSON")
			return
		}
		limits := getLimits()
		for key, value := range raw {
			switch key {
			case "daily_cost_limit_usd":
				v, ok := decodeLimitFloat(value)
				if !ok {
					writeAPIErrorField(w, http.StatusBadRequest, "error.guard.fieldNumber", "daily_cost_limit_usd", "daily_cost_limit_usd must be a number or null")
					return
				}
				limits.DailyCostLimitUSD = v
			case "monthly_cost_limit_usd":
				v, ok := decodeLimitFloat(value)
				if !ok {
					writeAPIErrorField(w, http.StatusBadRequest, "error.guard.fieldNumber", "monthly_cost_limit_usd", "monthly_cost_limit_usd must be a number or null")
					return
				}
				limits.MonthlyCostLimitUSD = v
			case "daily_tokens_limit":
				v, ok := decodeLimitInt(value)
				if !ok {
					writeAPIErrorField(w, http.StatusBadRequest, "error.guard.fieldInteger", "daily_tokens_limit", "daily_tokens_limit must be an integer or null")
					return
				}
				limits.DailyTokenLimit = v
			case "monthly_tokens_limit":
				v, ok := decodeLimitInt(value)
				if !ok {
					writeAPIErrorField(w, http.StatusBadRequest, "error.guard.fieldInteger", "monthly_tokens_limit", "monthly_tokens_limit must be an integer or null")
					return
				}
				limits.MonthlyTokenLimit = v
			case "warning_threshold_pct":
				v, ok := decodeLimitFloat(value)
				if !ok {
					writeAPIErrorField(w, http.StatusBadRequest, "error.guard.fieldNumber", "warning_threshold_pct", "warning_threshold_pct must be a number or null")
					return
				}
				limits.WarningThresholdPct = v
			case "action_on_breach":
				v, ok := decodeOptionalString(value)
				if !ok {
					writeAPIError(w, http.StatusBadRequest, "error.guard.action", "action_on_breach must be alert or block")
					return
				}
				limits.ActionOnBreach = v
			case "timezone":
				v, ok := decodeOptionalString(value)
				if !ok {
					writeAPIError(w, http.StatusBadRequest, "error.guard.timezone", "timezone must be an IANA timezone string")
					return
				}
				if v != "" {
					if _, err := time.LoadLocation(v); err != nil {
						writeAPIError(w, http.StatusBadRequest, "error.guard.timezone", "invalid timezone")
						return
					}
				}
				limits.Timezone = v
			default:
				writeAPIError(w, http.StatusBadRequest, "error.guard.unknownField", "unknown guard limit field")
				return
			}
		}
		if limits.ActionOnBreach == "" {
			limits.ActionOnBreach = "block"
		}
		if limits.ActionOnBreach != "alert" && limits.ActionOnBreach != "block" {
			writeAPIError(w, http.StatusBadRequest, "error.guard.action", "action_on_breach must be alert or block")
			return
		}
		if limits.DailyCostLimitUSD < 0 || limits.MonthlyCostLimitUSD < 0 ||
			limits.DailyTokenLimit < 0 || limits.MonthlyTokenLimit < 0 ||
			limits.WarningThresholdPct < 0 || limits.WarningThresholdPct > 100 {
			writeAPIError(w, http.StatusBadRequest, "error.guard.range", "limits must be non-negative and warning threshold must be 0..100")
			return
		}
		if configPath != "" {
			if err := persistGuardLimits(configPath, limits); err != nil {
				writeAPIError(w, http.StatusInternalServerError, "error.guard.persist", fmt.Sprintf("persist limits: %v", err))
				return
			}
		}
		updateLimits(limits)
		writeJSON(w, http.StatusOK, limitsView(limits))
	})

	r.Get("/filters", func(w http.ResponseWriter, r *http.Request) {
		since := resolveSince(r.URL.Query().Get("since"))
		project := r.URL.Query().Get("project")
		source := r.URL.Query().Get("source")
		projects, projectErr := database.GetSummaryByProject(since, source)
		sources, sourceErr := database.GetSummaryBySource(since, project)
		tags, tagErr := database.GetSummaryByTag(since, project, source)
		if projectErr != nil || sourceErr != nil || tagErr != nil {
			writeAPIError(w, http.StatusInternalServerError, "error.filters.unavailable", "filter options unavailable")
			return
		}
		if projects == nil {
			projects = []db.ByProject{}
		}
		if sources == nil {
			sources = []db.BySource{}
		}
		if tags == nil {
			tags = []db.ByTag{}
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"by_project": projects,
			"by_source":  sources,
			"by_tag":     tags,
		})
	})
	r.Get("/stats", func(w http.ResponseWriter, r *http.Request) {
		since := resolveSince(r.URL.Query().Get("since"))
		project := r.URL.Query().Get("project")
		source := r.URL.Query().Get("source")
		tag := r.URL.Query().Get("tag")
		tz := r.URL.Query().Get("tz")

		// A storage failure must be visible. Silently substituting zeroes made
		// the dashboard look healthy while every metric was fabricated.
		summary, err := database.GetSummary(since, project, source, tag)
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, "error.stats.unavailable", "stats unavailable")
			return
		}
		// A failure in any partition must be visible, matching the summary
		// above. A silently dropped group made one pane look empty or stale
		// while the rest of the dashboard asserted current data.
		var partitionErrs []string
		byModel, err := database.GetSummaryByModel(since, project, source)
		if err != nil {
			partitionErrs = append(partitionErrs, "by_model")
		}
		byClient, err := database.GetSummaryByClient(since, project, source)
		if err != nil {
			partitionErrs = append(partitionErrs, "by_client")
		}
		byProject, err := database.GetSummaryByProject(since, source)
		if err != nil {
			partitionErrs = append(partitionErrs, "by_project")
		}
		bySource, err := database.GetSummaryBySource(since, project)
		if err != nil {
			partitionErrs = append(partitionErrs, "by_source")
		}
		byTag, err := database.GetSummaryByTag(since, project, source)
		if err != nil {
			partitionErrs = append(partitionErrs, "by_tag")
		}
		recentEvents, err := database.RecentEventsFiltered(50, since, project, source, tag)
		if err != nil {
			partitionErrs = append(partitionErrs, "recent_events")
		}
		daily, err := database.GetDailyUsage(45, since, source, tz)
		if err != nil {
			partitionErrs = append(partitionErrs, "daily")
		}
		liveSessions, err := database.GetLiveSessions()
		if err != nil {
			partitionErrs = append(partitionErrs, "live_sessions")
		}
		tps, err := database.GetTpsSummary(since, project, source)
		if err != nil {
			partitionErrs = append(partitionErrs, "tps")
		}
		if len(partitionErrs) > 0 {
			writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
				"error":      "stats partitions unavailable",
				"partitions": partitionErrs,
			})
			return
		}
		// Normalize summary to match frontend Summary interface
		summaryMap := map[string]interface{}{
			"total_requests":           summary.TotalRequests,
			"total_prompt_tokens":      summary.PromptTokens,
			"total_completion_tokens":  summary.CompletionTokens,
			"total_tokens":             summary.TotalTokens,
			"total_cache_read_tokens":  summary.CacheReadTokens,
			"total_cache_write_tokens": summary.CacheWriteTokens,
			"total_cost_usd":           summary.TotalCostUSD,
			"avg_latency_ms":           summary.AvgLatencyMs,
			"avg_ttfb_ms":              summary.AvgTTFBMs,
		}

		// Normalize by_model to match ModelRow interface
		modelRows := make([]map[string]interface{}, 0, len(byModel))
		for _, m := range byModel {
			modelRows = append(modelRows, map[string]interface{}{
				"model":               m.Model,
				"provider":            m.Provider,
				"requests":            m.TotalRequests,
				"prompt_tokens":       m.PromptTokens,
				"completion_tokens":   m.CompletionToks,
				"total_tokens":        m.TotalTokens,
				"cache_read_tokens":   m.CacheReadToks,
				"cache_write_tokens":  m.CacheWriteToks,
				"cost_usd":            m.TotalCostUSD,
				"price_missing_count": m.PriceMissing,
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
				"project_id":      p.ProjectID,
				"requests":        p.TotalRequests,
				"total_tokens":    p.TotalTokens,
				"cost_usd":        p.TotalCostUSD,
				"shadow_cost_usd": p.ShadowCost,
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
				"id":                 e.ID,
				"timestamp":          e.Timestamp.Format(time.RFC3339Nano),
				"provider":           e.Provider,
				"model":              e.Model,
				"client_app":         e.ClientApp,
				"status_code":        e.StatusCode,
				"prompt_tokens":      e.PromptTokens,
				"total_tokens":       e.TotalTokens,
				"completion_tokens":  e.CompletionTokens,
				"cache_read_tokens":  e.CacheReadTokens,
				"cache_write_tokens": e.CacheWriteTokens,
				"cost_usd":           e.CostUSD,
				"latency_ms":         e.LatencyMs,
				"ttfb_ms":            e.TTFBMs,
				"streaming_ms":       e.StreamingMs,
				"tags":               e.Tags,
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

		// Throughput covers only rows whose timing was measured. Reporting
		// completion tokens as total_tokens hid prompt and cache usage.
		liveTPS, err := database.GetLiveTPS(10)
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, "error.stats.unavailable", "stats unavailable")
			return
		}
		tpsMap := map[string]interface{}{
			"requests":          tps.TotalReqs,
			"completion_tokens": tps.CompletionTokens,
			"total_tokens":      tps.TotalTokens,
			"total_latency_ms":  tps.AvgLatencyMs * float64(tps.TotalReqs),
			"avg_latency_ms":    tps.AvgLatencyMs,
			"avg_tps":           tps.AvgTPS,
			"peak_tps":          tps.PeakTPS,
			"live_tps":          liveTPS,
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
				"run_id":        s.RunID,
				"cli":           s.CLI,
				"session_id":    s.SessionID,
				"provider":      s.Provider,
				"model":         s.Model,
				"tokens_input":  s.TokensInput,
				"tokens_output": s.TokensOutput,
				"cost_usd":      s.CostUSD,
				"project":       s.Project,
				"title":         "",
				"started_at":    s.StartedAt,
				"last_updated":  s.LastUpdated,
				"elapsed_ms":    s.ElapsedMs,
			})
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"summary":       summaryMap,
			"by_model":      modelRows,
			"by_client":     clientRows,
			"by_project":    projectRows,
			"by_source":     sourceRows,
			"by_tag":        tagRows,
			"recent_events": eventRows,
			"live_sessions": sessionRows,
			"daily":         dailyRows,
			"tps":           tpsMap,
			"savings":       savings,
			"guard":         budgetStatus(project),
		})
	})

	r.Get("/stats/daily-detail", func(w http.ResponseWriter, r *http.Request) {
		day := r.URL.Query().Get("day")
		if day == "" {
			writeAPIError(w, http.StatusBadRequest, "error.stats.dayRequired", "day is required (YYYY-MM-DD)")
			return
		}
		tz := r.URL.Query().Get("tz")
		project := r.URL.Query().Get("project")
		source := r.URL.Query().Get("source")
		tag := r.URL.Query().Get("tag")

		summary, byClient, bySource, err := database.GetDailyDetail(day, tz, project, source, tag)
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, "error.stats.detailUnavailable", fmt.Sprintf("daily detail unavailable: %v", err))
			return
		}

		clientRows := make([]map[string]interface{}, 0, len(byClient))
		for _, c := range byClient {
			clientRows = append(clientRows, map[string]interface{}{
				"client_app":   c.Key,
				"requests":     c.Requests,
				"total_tokens": c.TotalTokens,
				"cost_usd":     c.CostUSD,
			})
		}
		sourceRows := make([]map[string]interface{}, 0, len(bySource))
		for _, s := range bySource {
			sourceRows = append(sourceRows, map[string]interface{}{
				"source":       s.Key,
				"requests":     s.Requests,
				"total_tokens": s.TotalTokens,
				"cost_usd":     s.CostUSD,
			})
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"day": day,
			"summary": map[string]interface{}{
				"total_requests":           summary.TotalRequests,
				"total_prompt_tokens":      summary.PromptTokens,
				"total_completion_tokens":  summary.CompletionTokens,
				"total_tokens":             summary.TotalTokens,
				"total_cache_read_tokens":  summary.CacheReadTokens,
				"total_cache_write_tokens": summary.CacheWriteTokens,
				"total_cost_usd":           summary.TotalCostUSD,
				"avg_latency_ms":           summary.AvgLatencyMs,
				"avg_ttfb_ms":              summary.AvgTTFBMs,
			},
			"by_client": clientRows,
			"by_source": sourceRows,
		})
	})

	r.Get("/stats.csv", func(w http.ResponseWriter, r *http.Request) {
		since := resolveSince(r.URL.Query().Get("since"))
		project := r.URL.Query().Get("project")
		source := r.URL.Query().Get("source")
		tag := r.URL.Query().Get("tag")
		summary, err := database.GetSummary(since, project, source, tag)
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, "error.stats.unavailable", "stats unavailable")
			return
		}
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="odotoken-stats.csv"`)
		csv := fmt.Sprintf("requests,total_tokens,prompt_tokens,completion_tokens,cost_usd,shadow_cost_usd,avg_latency_ms,avg_ttfb_ms\n%d,%d,%d,%d,%.8f,%.8f,%.2f,%.2f\n",
			summary.TotalRequests, summary.TotalTokens, summary.PromptTokens,
			summary.CompletionTokens, summary.TotalCostUSD, summary.TotalShadowCost,
			summary.AvgLatencyMs, summary.AvgTTFBMs)
		_, _ = w.Write([]byte(csv))
	})

	r.Get("/stats.svg", func(w http.ResponseWriter, r *http.Request) {
		since := resolveSince(r.URL.Query().Get("since"))
		project := r.URL.Query().Get("project")
		source := r.URL.Query().Get("source")
		tag := r.URL.Query().Get("tag")
		summary, err := database.GetSummary(since, project, source, tag)
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, "error.stats.unavailable", "stats unavailable")
			return
		}
		w.Header().Set("Content-Type", "image/svg+xml; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="odotoken-stats.svg"`)
		labels := svgLabelsFor(r.URL.Query().Get("lang"))
		svg := fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 800 360" role="img" aria-label="%s">
<rect width="800" height="360" fill="#ffffff"/>
<text x="32" y="48" font-family="system-ui,sans-serif" font-size="24" font-weight="700" fill="#111827">%s</text>
<text x="32" y="76" font-family="system-ui,sans-serif" font-size="12" fill="#6b7280">%s · %s · %s</text>
<g font-family="system-ui,sans-serif" fill="#111827">
<text x="32" y="130" font-size="13" fill="#6b7280">%s</text><text x="32" y="164" font-size="30" font-weight="700">%d</text>
<text x="232" y="130" font-size="13" fill="#6b7280">%s</text><text x="232" y="164" font-size="30" font-weight="700">%d</text>
<text x="432" y="130" font-size="13" fill="#6b7280">%s</text><text x="432" y="164" font-size="30" font-weight="700">$%.2f</text>
<text x="632" y="130" font-size="13" fill="#6b7280">%s</text><text x="632" y="164" font-size="30" font-weight="700">%.0fms</text>
</g>
<line x1="32" y1="206" x2="768" y2="206" stroke="#e5e7eb"/>
<text x="32" y="244" font-family="system-ui,sans-serif" font-size="13" fill="#6b7280">%s <tspan font-weight="700" fill="#111827">%d</tspan> · %s <tspan font-weight="700" fill="#111827">%d</tspan> · %s <tspan font-weight="700" fill="#111827">%d</tspan></text>
<text x="32" y="276" font-family="system-ui,sans-serif" font-size="13" fill="#6b7280">%s <tspan font-weight="700" fill="#111827">%.0fms</tspan> · %s <tspan font-weight="700" fill="#111827">$%.2f</tspan></text>
</svg>`, escapeXML(labels.Title), escapeXML(labels.Title), escapeXML(sinceLabel(r.URL.Query().Get("since"), labels)), escapeXML(project), escapeXML(source),
			labels.Requests, summary.TotalRequests, labels.Tokens, summary.TotalTokens,
			labels.Spent, summary.TotalCostUSD, labels.Latency, summary.AvgLatencyMs,
			labels.Prompt, summary.PromptTokens, labels.Completion, summary.CompletionTokens,
			labels.CacheRead, summary.CacheReadTokens,
			labels.TTFT, summary.AvgTTFBMs, labels.Shadow, summary.TotalShadowCost)
		_, _ = w.Write([]byte(svg))
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
				"id":                 e.ID,
				"timestamp":          e.Timestamp.Format(time.RFC3339Nano),
				"provider":           e.Provider,
				"model":              e.Model,
				"client_app":         e.ClientApp,
				"status_code":        e.StatusCode,
				"prompt_tokens":      e.PromptTokens,
				"total_tokens":       e.TotalTokens,
				"completion_tokens":  e.CompletionTokens,
				"cache_read_tokens":  e.CacheReadTokens,
				"cache_write_tokens": e.CacheWriteTokens,
				"cost_usd":           e.CostUSD,
				"latency_ms":         e.LatencyMs,
				"ttfb_ms":            e.TTFBMs,
				"streaming_ms":       e.StreamingMs,
				"tags":               e.Tags,
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
		liveTPS, _ := database.GetLiveTPS(10)

		// Normalize series to match TpsBucket interface
		seriesRows := make([]map[string]interface{}, 0, len(series))
		for _, s := range series {
			seriesRows = append(seriesRows, map[string]interface{}{
				"bucket":            s.Timestamp,
				"requests":          s.Requests,
				"total_tokens":      s.Tokens,
				"completion_tokens": s.Tokens,
				"total_latency_ms":  0.0,
				"avg_latency_ms":    0.0,
				"avg_tps":           s.AvgTPS,
				"peak_tps":          s.PeakTPS,
			})
		}

		summaryMap := map[string]interface{}{
			"requests":          summary.TotalReqs,
			"completion_tokens": summary.CompletionTokens,
			"total_tokens":      summary.TotalTokens,
			"total_latency_ms":  summary.AvgLatencyMs * float64(summary.TotalReqs),
			"avg_latency_ms":    summary.AvgLatencyMs,
			"avg_tps":           summary.AvgTPS,
			"peak_tps":          summary.PeakTPS,
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
		tz := r.URL.Query().Get("tz")
		daily, _ := database.GetDailyUsage(45, "", tag, tz)
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

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"forecast":  forecast,
			"anomalies": anomalies,
			"daily":     dailyRows,
		})
	})

	r.Get("/sessions/live", func(w http.ResponseWriter, r *http.Request) {
		sessions, err := database.GetLiveSessions()
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, "error.stats.liveRuns", "live runs unavailable")
			return
		}

		// Normalize to match LiveSession interface
		sessionRows := make([]map[string]interface{}, 0, len(sessions))
		for _, s := range sessions {
			sessionRows = append(sessionRows, map[string]interface{}{
				"run_id":        s.RunID,
				"cli":           s.CLI,
				"session_id":    s.SessionID,
				"provider":      s.Provider,
				"model":         s.Model,
				"tokens_input":  s.TokensInput,
				"tokens_output": s.TokensOutput,
				"cost_usd":      s.CostUSD,
				"project":       s.Project,
				"title":         "",
				"started_at":    s.StartedAt,
				"last_updated":  s.LastUpdated,
				"elapsed_ms":    s.ElapsedMs,
			})
		}
		writeJSON(w, http.StatusOK, sessionRows)
	})

	return r
}

// readConfigObject loads the config as raw blocks so handlers can preserve
// fields they do not own.
func readConfigObject(path string) (map[string]json.RawMessage, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		data, err = os.ReadFile("config.json.example")
	}
	if err != nil {
		return nil, err
	}
	var root map[string]json.RawMessage
	if err := json.Unmarshal(data, &root); err != nil {
		return nil, err
	}
	return root, nil
}

func writeConfigObject(path string, root map[string]json.RawMessage) error {
	data, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

// persistGuardLimits updates only the guard block, preserving every other
// config field.
func persistGuardLimits(path string, limits config.GuardConfig) error {
	root, err := readConfigObject(path)
	if err != nil {
		return err
	}
	body, err := json.Marshal(limits)
	if err != nil {
		return err
	}
	root["guard"] = body
	return writeConfigObject(path, root)
}

// persistSessionPath updates one entry of sessions.paths. Empty removes it.
func persistSessionPath(path, cli, root string) error {
	cfg, err := readConfigObject(path)
	if err != nil {
		return err
	}
	sessions := map[string]json.RawMessage{}
	if raw := cfg["sessions"]; len(raw) > 0 {
		if err := json.Unmarshal(raw, &sessions); err != nil {
			return err
		}
	}
	paths := map[string]string{}
	if raw := sessions["paths"]; len(raw) > 0 {
		if err := json.Unmarshal(raw, &paths); err != nil {
			return err
		}
	}
	if root == "" {
		delete(paths, cli)
	} else {
		paths[cli] = root
	}
	pathsBody, err := json.Marshal(paths)
	if err != nil {
		return err
	}
	sessions["paths"] = pathsBody
	sessionsBody, err := json.Marshal(sessions)
	if err != nil {
		return err
	}
	cfg["sessions"] = sessionsBody
	return writeConfigObject(path, cfg)
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

// decodeNumber decodes a JSON number or null. JSON null means the operator
// cleared the field, and zero is the stored representation of an unlimited
// guard limit.
func decodeNumber[T float64 | int](raw json.RawMessage) (T, bool) {
	var zero T
	if string(raw) == "null" {
		return zero, true
	}
	if err := json.Unmarshal(raw, &zero); err != nil {
		return zero, false
	}
	return zero, true
}

// decodeLimitFloat decodes a number-or-null PATCH field, mapping a value below
// one to an inactive limit.
func decodeLimitFloat(raw json.RawMessage) (float64, bool) {
	v, ok := decodeNumber[float64](raw)
	if !ok {
		return 0, false
	}
	if v <= 0 {
		return 0, true
	}
	return v, true
}

// decodeLimitInt decodes an integer-or-null PATCH field, mapping a value below
// one to an inactive limit.
func decodeLimitInt(raw json.RawMessage) (int, bool) {
	v, ok := decodeNumber[int](raw)
	if !ok {
		return 0, false
	}
	if v <= 0 {
		return 0, true
	}
	return v, true
}

// decodeOptionalString decodes a string PATCH field; JSON null maps to the
// caller's default action.
func decodeOptionalString(raw json.RawMessage) (string, bool) {
	if string(raw) == "null" {
		return "", true
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", false
	}
	return value, true
}

// escapeXML keeps user-supplied filter labels safe inside the snapshot SVG.
func escapeXML(value string) string {
	var b strings.Builder
	for _, r := range value {
		switch r {
		case '&':
			b.WriteString("&amp;")
		case '<':
			b.WriteString("&lt;")
		case '>':
			b.WriteString("&gt;")
		case '"':
			b.WriteString("&quot;")
		case '\'':
			b.WriteString("&apos;")
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func sinceLabel(raw string, labels svgLabels) string {
	if raw == "" || raw == "all" {
		return labels.AllTime
	}
	return raw
}
