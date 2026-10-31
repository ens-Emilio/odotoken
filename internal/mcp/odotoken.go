package mcp

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/ens-Emilio/odotoken/internal/analytics"
	"github.com/ens-Emilio/odotoken/internal/db"
	"github.com/ens-Emilio/odotoken/internal/domain"
	"github.com/ens-Emilio/odotoken/internal/pricing"
)

// NewOdotokenServer wires the three read-only tools. The MCP client asks for
// usage, budget state, and the forecast; every answer comes from the same
// SQLite rows the dashboard reads, so the numbers cannot disagree.
func NewOdotokenServer(database *db.Database, guardCheck func(string) domain.GuardCheckResult, calc *pricing.Calculator) *Server {
	tools := []Tool{
		{
			Name:        "odotoken_stats",
			Description: "Token usage, cost, and per-model/per-project breakdown for a window.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"since":   map[string]interface{}{"type": "string", "description": "RFC3339 timestamp or relative window such as 7d"},
					"project": map[string]interface{}{"type": "string"},
					"source":  map[string]interface{}{"type": "string"},
					"tag":     map[string]interface{}{"type": "string"},
				},
			},
		},
		{
			Name:        "odotoken_guard",
			Description: "Current budget state and limits for a project.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"project": map[string]interface{}{"type": "string", "description": "project id, default 'default'"},
				},
			},
		},
		{
			Name:        "odotoken_forecast",
			Description: "Linear cost forecast for the next seven days plus anomalous days attributed to models and projects.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"tz":      map[string]interface{}{"type": "string", "description": "IANA timezone for day boundaries"},
					"project": map[string]interface{}{"type": "string"},
					"source":  map[string]interface{}{"type": "string"},
					"tag":     map[string]interface{}{"type": "string"},
				},
			},
		},
	}

	calls := map[string]CallFunc{
		"odotoken_stats": func(args map[string]interface{}) (interface{}, error) {
			if database == nil {
				return nil, fmt.Errorf("database is not configured")
			}
			since := StringArg(args, "since")
			project := StringArg(args, "project")
			source := StringArg(args, "source")
			tag := StringArg(args, "tag")
			summary, err := database.GetSummary(since, project, source, tag)
			if err != nil {
				return nil, fmt.Errorf("summary: %w", err)
			}
			byModel, err := database.GetSummaryByModel(since, project, source)
			if err != nil {
				return nil, fmt.Errorf("by model: %w", err)
			}
			byProject, err := database.GetSummaryByProject(since, source)
			if err != nil {
				return nil, fmt.Errorf("by project: %w", err)
			}
			return map[string]interface{}{
				"summary": summary, "by_model": byModel, "by_project": byProject,
			}, nil
		},
		"odotoken_guard": func(args map[string]interface{}) (interface{}, error) {
			if guardCheck == nil {
				return nil, fmt.Errorf("guard is not configured")
			}
			project := StringArg(args, "project")
			if project == "" {
				project = "default"
			}
			return guardCheck(project), nil
		},
		"odotoken_forecast": func(args map[string]interface{}) (interface{}, error) {
			if database == nil {
				return nil, fmt.Errorf("database is not configured")
			}
			tz := StringArg(args, "tz")
			daily, err := database.GetDailyUsage(45, "", StringArg(args, "source"), tz)
			if err != nil {
				return nil, fmt.Errorf("daily usage: %w", err)
			}
			byModel, err := database.DailyModelCosts(StringArg(args, "project"), StringArg(args, "source"), StringArg(args, "tag"), tz)
			if err != nil {
				return nil, fmt.Errorf("model attribution: %w", err)
			}
			byProject, err := database.DailyProjectCosts(StringArg(args, "source"), StringArg(args, "tag"), tz)
			if err != nil {
				return nil, fmt.Errorf("project attribution: %w", err)
			}
			return map[string]interface{}{
				"forecast":     analytics.Forecast(daily, 7),
				"anomalies":    analytics.DetectAnomalies(daily, byModel, byProject),
				"generated_at": time.Now().UTC().Format(time.RFC3339),
			}, nil
		},
	}
	_ = calc
	return NewServer(tools, calls)
}

// MarshalToolResult is used by tests and callers that want the MCP content
// envelope without going through the transport.
func MarshalToolResult(value interface{}) (json.RawMessage, error) {
	text, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return json.Marshal(ToolResult{Content: []Content{{Type: "text", Text: string(text)}}})
}
