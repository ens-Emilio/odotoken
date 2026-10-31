// Package analytics derives the cost views that are not part of metering:
// cache savings, cross-model simulation, anomaly attribution and CSV export.
// Everything here reads the token_events rows the proxy already wrote, so no
// number can disagree with the dashboard's totals.
package analytics

import (
	"encoding/csv"
	"fmt"
	"io"
	"math"
	"sort"
	"strconv"

	"github.com/ens-Emilio/odotoken/internal/db"
	"github.com/ens-Emilio/odotoken/internal/pricing"
)

// CacheModelSaving is one model's cache contribution. Uncached is what the
// same cache-read tokens would have cost at the input rate.
type CacheModelSaving struct {
	Model           string  `json:"model"`
	Provider        string  `json:"provider"`
	Requests        int     `json:"requests"`
	CacheRead       int     `json:"cache_read_tokens"`
	CacheWrite      int     `json:"cache_write_tokens"`
	ReadSavedUSD    float64 `json:"read_saved_usd"`
	WritePremiumUSD float64 `json:"write_premium_usd"`
	NetSavedUSD     float64 `json:"net_saved_usd"`
	ActualUSD       float64 `json:"actual_cost_usd"`
	UncachedUSD     float64 `json:"uncached_cost_usd"`
	DiscountPct     float64 `json:"discount_pct"`
	Priced          bool    `json:"priced"`
}

// CacheSavings is the KPI payload. DiscountPct answers "how much cheaper was
// this window because of the cache", not "how many tokens were cached".
type CacheSavings struct {
	Requests        int                `json:"requests"`
	CacheRead       int                `json:"cache_read_tokens"`
	CacheWrite      int                `json:"cache_write_tokens"`
	ActualUSD       float64            `json:"actual_cost_usd"`
	ReadSavedUSD    float64            `json:"read_saved_usd"`
	WritePremiumUSD float64            `json:"write_premium_usd"`
	NetSavedUSD     float64            `json:"net_saved_usd"`
	UncachedUSD     float64            `json:"uncached_cost_usd"`
	DiscountPct     float64            `json:"discount_pct"`
	UnpricedModels  int                `json:"unpriced_models"`
	ByModel         []CacheModelSaving `json:"by_model"`
}

// ComputeCacheSavings prices each row twice: once as recorded, once as if every
// cache-read token had been a normal input token. A model without an explicit
// price is reported unpriced instead of contributing a fabricated saving.
func ComputeCacheSavings(rows []db.CacheTokenRow, calc *pricing.Calculator) CacheSavings {
	out := CacheSavings{ByModel: make([]CacheModelSaving, 0, len(rows))}
	for _, row := range rows {
		s := CacheModelSaving{
			Model: row.Model, Provider: row.Provider, Requests: row.Requests,
			CacheRead: row.CacheReadTokens, CacheWrite: row.CacheWriteTokens,
			ActualUSD: row.ActualCostUSD, Priced: calc.HasExplicitPrice(row.Model),
		}
		out.Requests += row.Requests
		out.CacheRead += row.CacheReadTokens
		out.CacheWrite += row.CacheWriteTokens
		out.ActualUSD += row.ActualCostUSD
		if s.Priced {
			uncached := calc.CalculateCost(row.Model, row.PromptTokens+row.CacheReadTokens,
				row.CompletionTokens, 0, row.CacheWriteTokens)
			s.UncachedUSD = uncached
			s.ReadSavedUSD = math.Max(0, uncached-row.ActualCostUSD)
			// A cache write billed above the input rate is a cost the cache
			// introduced, so it must reduce the reported saving.
			rates := calc.GetModelRates(row.Model)
			if rates.CacheWrite > rates.Input {
				s.WritePremiumUSD = float64(row.CacheWriteTokens) * (rates.CacheWrite - rates.Input) / 1e6
			}
			s.NetSavedUSD = s.ReadSavedUSD - s.WritePremiumUSD
			if s.UncachedUSD > 0 {
				s.DiscountPct = round(s.NetSavedUSD/s.UncachedUSD*100, 2)
			}
			out.ReadSavedUSD += s.ReadSavedUSD
			out.WritePremiumUSD += s.WritePremiumUSD
			out.NetSavedUSD += s.NetSavedUSD
			out.UncachedUSD += s.UncachedUSD
		} else {
			out.UnpricedModels++
		}
		out.ByModel = append(out.ByModel, s)
	}
	if out.UncachedUSD > 0 {
		out.DiscountPct = round(out.NetSavedUSD/out.UncachedUSD*100, 2)
	}
	sort.Slice(out.ByModel, func(i, j int) bool { return out.ByModel[i].NetSavedUSD > out.ByModel[j].NetSavedUSD })
	return out
}

// SimulationInput is the token shape a caller wants priced across models.
type SimulationInput struct {
	Requests         int `json:"requests"`
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	CacheReadTokens  int `json:"cache_read_tokens"`
	CacheWriteTokens int `json:"cache_write_tokens"`
}

// SimulationRow is one model's price for that shape.
type SimulationRow struct {
	Model              string  `json:"model"`
	Input              float64 `json:"input_rate_per_1m"`
	Output             float64 `json:"output_rate_per_1m"`
	CacheRead          float64 `json:"cache_read_rate_per_1m"`
	CacheWrite         float64 `json:"cache_write_rate_per_1m"`
	PerRequestUSD      float64 `json:"per_request_usd"`
	TotalUSD           float64 `json:"total_usd"`
	DeltaVsBaselineUSD float64 `json:"delta_vs_baseline_usd"`
	Priced             bool    `json:"priced"`
}

// Simulate prices one workload across the catalog. baseline is the model the
// caller is comparing against; an empty baseline uses the cheapest priced row.
func Simulate(calc *pricing.Calculator, in SimulationInput, models []string, baseline string, limit int) ([]SimulationRow, string) {
	if in.Requests <= 0 {
		in.Requests = 1
	}
	if len(models) == 0 {
		for model := range calc.Prices() {
			if model == "default" {
				continue
			}
			models = append(models, model)
		}
	}
	sort.Strings(models)
	rows := make([]SimulationRow, 0, len(models))
	for _, model := range models {
		rates := calc.GetModelRates(model)
		perRequest := calc.CalculateCost(model, in.PromptTokens, in.CompletionTokens,
			in.CacheReadTokens, in.CacheWriteTokens)
		rows = append(rows, SimulationRow{
			Model: model, Input: rates.Input, Output: rates.Output,
			CacheRead: rates.CacheRead, CacheWrite: rates.CacheWrite,
			PerRequestUSD: perRequest, TotalUSD: round(perRequest*float64(in.Requests), 6),
			Priced: calc.HasExplicitPrice(model),
		})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].TotalUSD < rows[j].TotalUSD })
	if baseline == "" && len(rows) > 0 {
		baseline = rows[0].Model
	}
	base := 0.0
	for _, row := range rows {
		if row.Model == baseline {
			base = row.TotalUSD
		}
	}
	for i := range rows {
		rows[i].DeltaVsBaselineUSD = round(rows[i].TotalUSD-base, 6)
	}
	if limit > 0 && len(rows) > limit {
		rows = rows[:limit]
	}
	return rows, baseline
}

// AnomalyAttribution explains one expensive day: which models and projects
// moved away from their own baseline, in dollars.
type Attribution struct {
	Key         string  `json:"key"`
	CostUSD     float64 `json:"cost_usd"`
	BaselineUSD float64 `json:"baseline_usd"`
	DeltaUSD    float64 `json:"delta_usd"`
	SharePct    float64 `json:"share_pct"`
}

// Anomaly is a day whose spend exceeds mean+2σ of the window.
type Anomaly struct {
	Day       string        `json:"day"`
	CostUSD   float64       `json:"cost_usd"`
	Threshold float64       `json:"threshold"`
	MeanUSD   float64       `json:"mean_usd"`
	SigmaUSD  float64       `json:"sigma_usd"`
	ZScore    float64       `json:"z_score"`
	ByModel   []Attribution `json:"by_model"`
	ByProject []Attribution `json:"by_project"`
}

// DetectAnomalies needs at least three days before it will call anything
// anomalous; two points always look like a trend and never like an outlier.
func DetectAnomalies(daily []db.DailyUsage, byModel, byProject []db.DailyDimensionCost) []Anomaly {
	if len(daily) < 3 {
		return []Anomaly{}
	}
	costs := make([]float64, len(daily))
	mean := 0.0
	for i, d := range daily {
		costs[i] = d.CostUSD
		mean += d.CostUSD
	}
	mean /= float64(len(daily))
	variance := 0.0
	for _, c := range costs {
		variance += (c - mean) * (c - mean)
	}
	sigma := math.Sqrt(variance / float64(len(daily)))
	threshold := mean + 2*sigma
	if sigma == 0 {
		return []Anomaly{}
	}
	out := make([]Anomaly, 0, 2)
	for _, d := range daily {
		if d.CostUSD <= threshold {
			continue
		}
		out = append(out, Anomaly{
			Day: d.Date, CostUSD: round(d.CostUSD, 6), Threshold: round(threshold, 6),
			MeanUSD: round(mean, 6), SigmaUSD: round(sigma, 6),
			ZScore:    round((d.CostUSD-mean)/sigma, 2),
			ByModel:   attribute(d.Date, byModel, d.CostUSD),
			ByProject: attribute(d.Date, byProject, d.CostUSD),
		})
	}
	return out
}

// attribute compares one day against the mean of the other days. The share is
// of the anomalous day's spend, so the numbers add up to the bill the reader
// is looking at.
func attribute(day string, rows []db.DailyDimensionCost, dayTotal float64) []Attribution {
	dayCost := map[string]float64{}
	baseline := map[string][]float64{}
	for _, r := range rows {
		if r.Day == day {
			dayCost[r.Key] += r.CostUSD
			continue
		}
		baseline[r.Key] = append(baseline[r.Key], r.CostUSD)
	}
	out := make([]Attribution, 0, len(dayCost))
	for key, cost := range dayCost {
		mean := 0.0
		if values := baseline[key]; len(values) > 0 {
			for _, v := range values {
				mean += v
			}
			mean /= float64(len(values))
		}
		share := 0.0
		if dayTotal > 0 {
			share = cost / dayTotal * 100
		}
		out = append(out, Attribution{
			Key: key, CostUSD: round(cost, 6), BaselineUSD: round(mean, 6),
			DeltaUSD: round(cost-mean, 6), SharePct: round(share, 2),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].DeltaUSD > out[j].DeltaUSD })
	if len(out) > 5 {
		out = out[:5]
	}
	return out
}

// EventColumns is the CSV contract. It is ordered for a spreadsheet reader:
// identity, attribution, tokens, money, timing.
var EventColumns = []string{
	"id", "timestamp", "provider", "model", "client_app", "project_id",
	"session_id", "run_id", "source", "status_code", "prompt_tokens", "completion_tokens",
	"total_tokens", "cache_read_tokens", "cache_write_tokens", "cost_usd",
	"shadow_cost", "latency_ms", "ttfb_ms", "streaming_ms", "tags",
}

// WriteEventsCSV streams rows with encoding/csv, which quotes and escapes
// commas, quotes and newlines in tags and metadata.
func WriteEventsCSV(w io.Writer, rows []map[string]interface{}) error {
	cw := csv.NewWriter(w)
	if err := cw.Write(EventColumns); err != nil {
		return err
	}
	record := make([]string, len(EventColumns))
	for _, row := range rows {
		for i, column := range EventColumns {
			record[i] = cell(row[column])
		}
		if err := cw.Write(record); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

func cell(v interface{}) string {
	switch value := v.(type) {
	case nil:
		return ""
	case string:
		return value
	case float64:
		return strconv.FormatFloat(value, 'f', -1, 64)
	case int:
		return strconv.Itoa(value)
	case int64:
		return strconv.FormatInt(value, 10)
	default:
		return fmt.Sprint(value)
	}
}

func round(v float64, places int) float64 {
	mul := math.Pow(10, float64(places))
	return math.Round(v*mul) / mul
}
