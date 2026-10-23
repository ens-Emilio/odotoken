package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/pkoukk/tiktoken-go"

	"github.com/user/odotoken/internal/config"
	"github.com/user/odotoken/internal/domain"
	"github.com/user/odotoken/internal/pricing"
)

// CountTextRequest is the JSON body for /v1/count with text.
type CountTextRequest struct {
	Model    string `json:"model"`
	Text     string `json:"text"`
	Provider string `json:"provider,omitempty"`
}

// CountMessagesRequest is the JSON body for /v1/count with messages.
type CountMessagesRequest struct {
	Model    string                   `json:"model"`
	Messages []map[string]interface{} `json:"messages"`
	Tools    []map[string]interface{} `json:"tools,omitempty"`
}

// CountResponse is the response for /v1/count.
type CountResponse struct {
	Model          string  `json:"model"`
	PromptTokens   int     `json:"prompt_tokens"`
	TotalTokens    int     `json:"total_tokens"`
	CostUSD        float64 `json:"cost_usd"`
	Method         string  `json:"method"`
	DurationMs     float64 `json:"duration_ms"`
}

// BatchCountRequest wraps multiple count requests.
type BatchCountRequest struct {
	Items []json.RawMessage `json:"items"`
}

// BatchCountResponse wraps multiple count responses.
type BatchCountResponse struct {
	TotalItems int             `json:"total_items"`
	Results    []CountResponse `json:"results"`
	Errors     []BatchError    `json:"errors"`
}

// BatchError describes a failed item in batch.
type BatchError struct {
	Index int    `json:"index"`
	Error string `json:"error"`
}

// ModelInfo describes an available model.
type ModelInfo struct {
	ID       string  `json:"id"`
	Provider string  `json:"provider"`
	InputRate float64 `json:"input_rate_per_1m"`
	OutputRate float64 `json:"output_rate_per_1m"`
}

// Router creates the API router for /v1 endpoints.
func Router(cfg config.Config, calc *pricing.Calculator) chi.Router {
	r := chi.NewRouter()

	r.Post("/count", func(w http.ResponseWriter, r *http.Request) {
		handleCount(w, r, cfg, calc)
	})
	r.Post("/count/batch", func(w http.ResponseWriter, r *http.Request) {
		handleBatchCount(w, r, cfg, calc)
	})
	r.Get("/models", func(w http.ResponseWriter, _ *http.Request) {
		handleModels(w, calc)
	})
	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	return r
}

func handleCount(w http.ResponseWriter, r *http.Request, cfg config.Config, calc *pricing.Calculator) {
	var raw map[string]interface{}
	dec := json.NewDecoder(r.Body)
	dec.UseNumber()
	if err := dec.Decode(&raw); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	model, _ := raw["model"].(string)
	if model == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "model is required"})
		return
	}

	text, hasText := raw["text"].(string)
	messages, hasMessages := raw["messages"]

	var tokenCount int
	method := "heuristic"

	if hasText && text != "" {
		tokenCount = countTokens(model, text, cfg)
		if enc, err := tiktoken.EncodingForModel(model); err == nil {
			tokenCount = len(enc.Encode(text, nil, nil))
			method = "tiktoken"
		}
	} else if hasMessages {
		// Serialize messages to estimate tokens
		msgBytes, _ := json.Marshal(messages)
		tokenCount = countTokens(model, string(msgBytes), cfg)
		method = "heuristic"
	}

	costUSD := calc.CalculateCost(model, tokenCount, 0, 0, 0)

	resp := CountResponse{
		Model:        model,
		PromptTokens: tokenCount,
		TotalTokens:  tokenCount,
		CostUSD:      costUSD,
		Method:       method,
	}
	writeJSON(w, http.StatusOK, resp)
}

func handleBatchCount(w http.ResponseWriter, r *http.Request, cfg config.Config, calc *pricing.Calculator) {
	var req BatchCountRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	resp := BatchCountResponse{TotalItems: len(req.Items)}
	for i, item := range req.Items {
		var raw map[string]interface{}
		if err := json.Unmarshal(item, &raw); err != nil {
			resp.Errors = append(resp.Errors, BatchError{Index: i, Error: err.Error()})
			continue
		}
		model, _ := raw["model"].(string)
		text, _ := raw["text"].(string)
		if model == "" || text == "" {
			resp.Errors = append(resp.Errors, BatchError{Index: i, Error: "model and text required"})
			continue
		}
		tc := countTokens(model, text, cfg)
		costUSD := calc.CalculateCost(model, tc, 0, 0, 0)
		resp.Results = append(resp.Results, CountResponse{
			Model: model, PromptTokens: tc, TotalTokens: tc, CostUSD: costUSD, Method: "heuristic",
		})
	}
	writeJSON(w, http.StatusOK, resp)
}

func handleModels(w http.ResponseWriter, calc *pricing.Calculator) {
	models := []ModelInfo{
		{ID: "claude-3-7-sonnet", Provider: "anthropic", InputRate: 3.0, OutputRate: 15.0},
		{ID: "claude-3-5-sonnet", Provider: "anthropic", InputRate: 3.0, OutputRate: 15.0},
		{ID: "claude-3-5-haiku", Provider: "anthropic", InputRate: 0.8, OutputRate: 4.0},
		{ID: "gpt-4o", Provider: "openai", InputRate: 2.5, OutputRate: 10.0},
		{ID: "gpt-4o-mini", Provider: "openai", InputRate: 0.15, OutputRate: 0.60},
		{ID: "o1", Provider: "openai", InputRate: 15.0, OutputRate: 60.0},
		{ID: "gemini-2.0-flash", Provider: "gemini", InputRate: 0.10, OutputRate: 0.40},
		{ID: "gemini-1.5-pro", Provider: "gemini", InputRate: 1.25, OutputRate: 5.0},
	}
	writeJSON(w, http.StatusOK, models)
}

func countTokens(model, text string, cfg config.Config) int {
	h := cfg.TokenCounter.Heuristic
	if h.CharsPerToken <= 0 {
		h.CharsPerToken = 4.0
	}
	lower := strings.ToLower(model)
	if isCJK(text) {
		return int(float64(len([]rune(text))) / 1.5)
	}
	_ = lower
	return int(float64(len(text)) / h.CharsPerToken)
}

func isCJK(s string) bool {
	for _, r := range s {
		if (r >= 0x4E00 && r <= 0x9FFF) || (r >= 0x3040 && r <= 0x309F) || (r >= 0xAC00 && r <= 0xD7AF) {
			return true
		}
	}
	return false
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// Ensure domain package is used.
var _ = domain.UsageResult{}