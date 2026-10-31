package parser

import (
	"bytes"
	"encoding/json"
	"strings"
	"time"

	"github.com/ens-Emilio/odotoken/internal/domain"
)

// IdentifyProvider detects the LLM provider from request path and headers.
func IdentifyProvider(path string, headers map[string]string) string {
	p := strings.ToLower(path)
	switch {
	case strings.Contains(p, "anthropic") || strings.Contains(p, "/v1/messages"):
		return "anthropic"
	case strings.Contains(p, "gemini") || strings.Contains(p, "generativelanguage"):
		return "gemini"
	case strings.Contains(p, "ollama"):
		return "ollama"
	default:
		return "openai"
	}
}

// IdentifyClientApp uses X-Odo-Client as the canonical attribution when a
// known client key has assigned one. Requests without that header retain the
// existing X-Client-App, then User-Agent precedence.
func IdentifyClientApp(headers map[string]string) string {
	if client := headers["x-odo-client"]; client != "" {
		return client
	}
	ua := strings.ToLower(headers["user-agent"])
	custom := headers["x-client-app"]
	if custom != "" {
		return custom
	}
	switch {
	case strings.Contains(ua, "pi-agent") || strings.Contains(ua, "pi/"):
		return "pi-agent"
	case strings.Contains(ua, "omp"):
		return "omp"
	case strings.Contains(ua, "omo"):
		return "omo"
	case strings.Contains(ua, "opencode"):
		return "opencode"
	case strings.Contains(ua, "crush"):
		return "crush"
	case strings.Contains(ua, "codex"):
		return "codex"
	case strings.Contains(ua, "claude"):
		return "claude-cli"
	default:
		return "unknown"
	}
}

// ParseJSONResponse extracts usage from a non-streaming JSON response.
func ParseJSONResponse(provider string, data map[string]interface{}, fallbackModel string) domain.UsageResult {
	res := domain.UsageResult{Provider: provider}

	if m, ok := data["model"].(string); ok && m != "" {
		res.Model = m
	} else {
		res.Model = fallbackModel
	}

	switch provider {
	case "anthropic":
		if usage, ok := data["usage"].(map[string]interface{}); ok {
			res.PromptTokens = toInt(usage["input_tokens"])
			res.CompletionTokens = toInt(usage["output_tokens"])
			res.CacheReadTokens = toInt(usage["cache_read_input_tokens"])
			res.CacheWriteTokens = toInt(usage["cache_creation_input_tokens"])
		}
	case "openai":
		if usage, ok := data["usage"].(map[string]interface{}); ok {
			res.PromptTokens = toInt(usage["prompt_tokens"])
			res.CompletionTokens = toInt(usage["completion_tokens"])
			res.TotalTokens = toInt(usage["total_tokens"])
			if details, ok := usage["prompt_tokens_details"].(map[string]interface{}); ok {
				res.CacheReadTokens = toInt(details["cached_tokens"])
			}
		}
	case "gemini":
		if usage, ok := data["usageMetadata"].(map[string]interface{}); ok {
			res.PromptTokens = toInt(usage["promptTokenCount"])
			res.CompletionTokens = toInt(usage["candidatesTokenCount"])
			res.TotalTokens = toInt(usage["totalTokenCount"])
			res.CacheReadTokens = toInt(usage["cachedContentTokenCount"])
		}
	case "ollama":
		res.PromptTokens = toInt(data["prompt_eval_count"])
		res.CompletionTokens = toInt(data["eval_count"])
	default:
		// OpenAI-compatible relays speak the OpenAI response shape. Without
		// this fallback their exact usage was replaced by a body-size estimate.
		if usage, ok := data["usage"].(map[string]interface{}); ok {
			res.PromptTokens = toInt(usage["prompt_tokens"])
			res.CompletionTokens = toInt(usage["completion_tokens"])
			res.TotalTokens = toInt(usage["total_tokens"])
			if details, ok := usage["prompt_tokens_details"].(map[string]interface{}); ok {
				res.CacheReadTokens = toInt(details["cached_tokens"])
			}
		}
	}

	if res.TotalTokens == 0 {
		res.TotalTokens = res.PromptTokens + res.CompletionTokens
	}
	return res
}

// maxPlausiblePromptTokens is deliberately far above current hosted context
// windows. Relays sometimes report large but legitimate agent histories, so a
// threshold near normal context sizes would discard real usage.
const maxPlausiblePromptTokens = 10_000_000

// SanitizeUsage drops only a usage block that is impossible for any hosted
// model. Valid large prompts, including long agent sessions, are preserved.
func SanitizeUsage(u domain.UsageResult) domain.UsageResult {
	if u.PromptTokens < 0 || u.CompletionTokens < 0 || u.CacheReadTokens < 0 || u.CacheWriteTokens < 0 ||
		u.PromptTokens > maxPlausiblePromptTokens || u.TotalTokens > 2*maxPlausiblePromptTokens {
		u.PromptTokens = 0
		u.CacheReadTokens = 0
		u.CacheWriteTokens = 0
		u.TotalTokens = u.CompletionTokens
		u.Estimated = true
	}
	if u.TotalTokens == 0 {
		u.TotalTokens = u.PromptTokens + u.CompletionTokens
	}
	return u
}

// StreamUsageAccumulator accumulates SSE chunks across streaming responses.
type StreamUsageAccumulator struct {
	provider         string
	model            string
	promptTokens     int
	completionTokens int
	totalTokens      int
	cacheReadTokens  int
	cacheWriteTokens int
	contentChars     int
	chunkCount       int
	firstChunkTime   time.Time
	startTime        time.Time
	hasFirstChunk    bool
	pending          []byte
}

// NewStreamAccumulator creates a new accumulator for a given provider.
func NewStreamAccumulator(provider, defaultModel string) *StreamUsageAccumulator {
	return &StreamUsageAccumulator{
		provider:  provider,
		model:     defaultModel,
		startTime: time.Now(),
	}
}

// Write makes the accumulator an io.Writer so a stream can be counted as it is
// read. Chunk boundaries from the network fall anywhere, including mid-line,
// so a partial trailing line is held until its newline arrives; feeding
// fragments straight to ProcessChunk would drop the usage block of whichever
// chunk happened to split it.
func (s *StreamUsageAccumulator) Write(p []byte) (int, error) {
	s.pending = append(s.pending, p...)
	for {
		i := bytes.IndexByte(s.pending, '\n')
		if i < 0 {
			break
		}
		s.ProcessChunk(s.pending[:i])
		s.pending = s.pending[i+1:]
	}
	return len(p), nil
}

// ProcessChunk parses an SSE chunk and accumulates usage data.
func (s *StreamUsageAccumulator) ProcessChunk(chunk []byte) {
	if !s.hasFirstChunk {
		s.firstChunkTime = time.Now()
		s.hasFirstChunk = true
	}
	s.chunkCount++

	lines := bytes.Split(chunk, []byte("\n"))
	for _, line := range lines {
		line = bytes.TrimSpace(line)
		if len(line) == 0 || !bytes.HasPrefix(line, []byte("data:")) {
			continue
		}
		payload := bytes.TrimSpace(line[5:])
		if bytes.Equal(payload, []byte("[DONE]")) {
			continue
		}

		var data map[string]interface{}
		if json.Unmarshal(payload, &data) != nil {
			continue
		}

		if m, ok := data["model"].(string); ok && m != "" {
			s.model = m
		}

		switch s.provider {
		case "anthropic":
			s.handleAnthropicEvent(data)
		case "gemini":
			s.handleGeminiEvent(data)
		case "ollama":
			s.handleOllamaEvent(data)
		default:
			// Every other provider is OpenAI-compatible.
			s.handleOpenAIEvent(data)
		}
	}
}

func (s *StreamUsageAccumulator) handleAnthropicEvent(data map[string]interface{}) {
	eventType, _ := data["type"].(string)
	if eventType == "message_delta" {
		if usage, ok := data["usage"].(map[string]interface{}); ok {
			s.completionTokens = toInt(usage["output_tokens"])
		}
	}
	if eventType == "message_start" {
		if msg, ok := data["message"].(map[string]interface{}); ok {
			if usage, ok := msg["usage"].(map[string]interface{}); ok {
				s.promptTokens = toInt(usage["input_tokens"])
				s.cacheReadTokens = toInt(usage["cache_read_input_tokens"])
				s.cacheWriteTokens = toInt(usage["cache_creation_input_tokens"])
			}
		}
	}
}

func (s *StreamUsageAccumulator) handleOpenAIEvent(data map[string]interface{}) {
	if usage, ok := data["usage"].(map[string]interface{}); ok {
		s.promptTokens = toInt(usage["prompt_tokens"])
		s.completionTokens = toInt(usage["completion_tokens"])
		s.totalTokens = toInt(usage["total_tokens"])
		if details, ok := usage["prompt_tokens_details"].(map[string]interface{}); ok {
			s.cacheReadTokens = toInt(details["cached_tokens"])
		}
	}
	if s.completionTokens == 0 {
		s.contentChars += openAIContentChars(data)
	}
}

func openAIContentChars(data map[string]interface{}) int {
	choices, ok := data["choices"].([]interface{})
	if !ok {
		return 0
	}
	total := 0
	for _, rawChoice := range choices {
		choice, ok := rawChoice.(map[string]interface{})
		if !ok {
			continue
		}
		delta, ok := choice["delta"].(map[string]interface{})
		if !ok {
			continue
		}
		content, _ := delta["content"].(string)
		total += len(content)
	}
	return total
}

func (s *StreamUsageAccumulator) handleGeminiEvent(data map[string]interface{}) {
	if usage, ok := data["usageMetadata"].(map[string]interface{}); ok {
		s.promptTokens = toInt(usage["promptTokenCount"])
		s.completionTokens = toInt(usage["candidatesTokenCount"])
		s.totalTokens = toInt(usage["totalTokenCount"])
		s.cacheReadTokens = toInt(usage["cachedContentTokenCount"])
	}
}

func (s *StreamUsageAccumulator) handleOllamaEvent(data map[string]interface{}) {
	s.promptTokens = toInt(data["prompt_eval_count"])
	s.completionTokens = toInt(data["eval_count"])
}

// Finalize returns the accumulated usage result.
func (s *StreamUsageAccumulator) Finalize() domain.UsageResult {
	if s.completionTokens == 0 && s.contentChars > 0 {
		s.completionTokens = (s.contentChars + 3) / 4
		s.totalTokens = s.promptTokens + s.completionTokens
	}
	if s.totalTokens == 0 {
		s.totalTokens = s.promptTokens + s.completionTokens
	}
	return domain.UsageResult{
		Provider:         s.provider,
		Model:            s.model,
		PromptTokens:     s.promptTokens,
		CompletionTokens: s.completionTokens,
		TotalTokens:      s.totalTokens,
		CacheReadTokens:  s.cacheReadTokens,
		CacheWriteTokens: s.cacheWriteTokens,
		Estimated:        s.contentChars > 0,
	}
}

// TTFBMs returns time-to-first-byte in milliseconds.
func (s *StreamUsageAccumulator) TTFBMs() float64 {
	if !s.hasFirstChunk {
		return 0
	}
	return float64(s.firstChunkTime.Sub(s.startTime).Milliseconds())
}

// StreamingMs returns total streaming duration in milliseconds.
func (s *StreamUsageAccumulator) StreamingMs() float64 {
	return float64(time.Since(s.startTime).Milliseconds())
}

func toInt(v interface{}) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case int64:
		return int(n)
	case json.Number:
		i, _ := n.Int64()
		return int(i)
	default:
		return 0
	}
}
