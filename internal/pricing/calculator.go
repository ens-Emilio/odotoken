package pricing

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// ModelRates holds per-million-token rates for a model.
type ModelRates struct {
	Input      float64 `json:"input"`
	Output     float64 `json:"output"`
	CacheRead  float64 `json:"cache_read"`
	CacheWrite float64 `json:"cache_write"`
}

var defaultPrices = map[string]ModelRates{
	"claude-3-7-sonnet": {Input: 3.0, Output: 15.0, CacheRead: 0.30, CacheWrite: 3.75},
	"claude-3-5-sonnet": {Input: 3.0, Output: 15.0, CacheRead: 0.30, CacheWrite: 3.75},
	"claude-3-5-haiku":  {Input: 0.8, Output: 4.0, CacheRead: 0.08, CacheWrite: 1.0},
	"claude-3-opus":     {Input: 15.0, Output: 75.0, CacheRead: 1.5, CacheWrite: 18.75},
	"gpt-4o":            {Input: 2.5, Output: 10.0, CacheRead: 1.25, CacheWrite: 0.0},
	"gpt-4o-mini":       {Input: 0.15, Output: 0.60, CacheRead: 0.075, CacheWrite: 0.0},
	"o1":                {Input: 15.0, Output: 60.0, CacheRead: 7.5, CacheWrite: 0.0},
	"o3-mini":           {Input: 1.10, Output: 4.40, CacheRead: 0.55, CacheWrite: 0.0},
	"gemini-2.0-flash":  {Input: 0.10, Output: 0.40, CacheRead: 0.025, CacheWrite: 0.0},
	"gemini-1.5-flash":  {Input: 0.075, Output: 0.30, CacheRead: 0.01875, CacheWrite: 0.0},
	"gemini-1.5-pro":    {Input: 1.25, Output: 5.0, CacheRead: 0.3125, CacheWrite: 0.0},
	"default":           {Input: 0.0, Output: 0.0, CacheRead: 0.0, CacheWrite: 0.0},
}

// Calculator computes token costs.
type Calculator struct {
	prices map[string]ModelRates
}

// NewCalculator creates a calculator, loading overrides from priceFile if it
// exists. A relative path that is missing from the working directory is looked
// up next to the executable: a service started by systemd or a dashboard run
// from another directory must price identically to one started from the repo,
// and silently falling back to the built-in table turned every imported cost
// into $0.
func NewCalculator(priceFile string) *Calculator {
	c := &Calculator{prices: make(map[string]ModelRates, len(defaultPrices))}
	for k, v := range defaultPrices {
		c.prices[k] = v
	}
	if priceFile == "" {
		return c
	}
	data, err := os.ReadFile(priceFile)
	if err != nil && !filepath.IsAbs(priceFile) {
		if exe, exeErr := os.Executable(); exeErr == nil {
			candidates := []string{
				filepath.Join(filepath.Dir(exe), priceFile),
				filepath.Join(filepath.Dir(exe), "..", priceFile),
			}
			for _, candidate := range candidates {
				if data, err = os.ReadFile(candidate); err == nil {
					break
				}
			}
		}
	}
	if err != nil {
		return c
	}
	var overrides map[string]ModelRates
	if json.Unmarshal(data, &overrides) != nil {
		return c
	}
	for k, v := range overrides {
		c.prices[k] = v
	}
	return c
}

// HasExplicitPrice reports whether a model has a real catalog price. Falling
// back to default ($0) is not a valid cost signal.
func (c *Calculator) HasExplicitPrice(model string) bool {
	if _, ok := c.prices[model]; ok {
		return true
	}
	for prefix := range c.prices {
		if prefix != "default" && strings.HasPrefix(model, prefix) {
			return true
		}
	}
	return false
}

// GetModelRates finds the best matching rates for a model name.
func (c *Calculator) GetModelRates(model string) ModelRates {
	if rates, ok := c.prices[model]; ok {
		return rates
	}
	for prefix, rates := range c.prices {
		if prefix != "default" && strings.HasPrefix(model, prefix) {
			return rates
		}
	}
	return c.prices["default"]
}

// CalculateCost computes USD cost for token usage (rates are per 1M tokens).
func (c *Calculator) CalculateCost(model string, promptTokens, completionTokens, cacheRead, cacheWrite int) float64 {
	r := c.GetModelRates(model)
	cost := float64(promptTokens)*r.Input/1_000_000 +
		float64(completionTokens)*r.Output/1_000_000 +
		float64(cacheRead)*r.CacheRead/1_000_000 +
		float64(cacheWrite)*r.CacheWrite/1_000_000
	return roundTo(cost, 7)
}

// CalculateShadowCost computes theoretical cloud cost for local/Ollama models.
func (c *Calculator) CalculateShadowCost(model string, promptTokens, completionTokens, cacheRead, cacheWrite int) float64 {
	if !isLocalModel(model) {
		return 0
	}
	equivalent := getCloudEquivalent(model)
	return c.CalculateCost(equivalent, promptTokens, completionTokens, cacheRead, cacheWrite)
}

func isLocalModel(model string) bool {
	prefixes := []string{"ollama/", "llama", "mistral", "qwen", "gemma"}
	lower := strings.ToLower(model)
	for _, p := range prefixes {
		if strings.HasPrefix(lower, p) || strings.Contains(lower, "/"+p) {
			return true
		}
	}
	return false
}

func getCloudEquivalent(model string) string {
	lower := strings.ToLower(model)
	switch {
	case strings.Contains(lower, "llama-3"):
		return "gpt-4o-mini"
	case strings.Contains(lower, "llama-2"):
		return "gpt-3.5-turbo"
	case strings.Contains(lower, "mistral"):
		return "gpt-4o-mini"
	case strings.Contains(lower, "qwen"):
		return "gpt-4o-mini"
	case strings.Contains(lower, "gemma"):
		return "gemini-1.5-flash"
	default:
		return "default"
	}
}

func roundTo(val float64, places int) float64 {
	mul := 1.0
	for range places {
		mul *= 10
	}
	return float64(int(val*mul+0.5)) / mul
}
