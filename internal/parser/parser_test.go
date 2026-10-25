package parser

import (
	"testing"
	"time"

	"github.com/ens-Emilio/odotoken/internal/domain"
)

// TestWriteMeasuresStreamDuration feeds an SSE stream through Write one chunk at
// a time with real delays and asserts the measured span matches the wall clock.
func TestWriteMeasuresStreamDuration(t *testing.T) {
	acc := NewStreamAccumulator("openai", "")

	chunks := []string{
		"data: {\"model\":\"gpt-test\",\"choices\":[{\"delta\":{\"content\":\"a\"}}]}\n",
		"data: {\"model\":\"gpt-test\",\"choices\":[{\"delta\":{\"content\":\"b\"}}]}\n",
		"data: {\"model\":\"gpt-test\",\"usage\":{\"prompt_tokens\":10,\"completion_tokens\":20,\"total_tokens\":30}}\n",
		"data: [DONE]\n",
	}
	const gap = 100 * time.Millisecond
	start := time.Now()
	for _, c := range chunks {
		if _, err := acc.Write([]byte(c)); err != nil {
			t.Fatalf("write: %v", err)
		}
		time.Sleep(gap)
	}
	elapsed := time.Since(start)

	streamingMs := acc.StreamingMs()
	if streamingMs < 200 {
		t.Errorf("streaming_ms = %.0f, want >= 200 (real elapsed %.0f)", streamingMs, float64(elapsed.Milliseconds()))
	}
	if streamingMs > float64(elapsed.Milliseconds())+200 {
		t.Errorf("streaming_ms = %.0f exceeds real elapsed %.0f", streamingMs, float64(elapsed.Milliseconds()))
	}

	ttfb := acc.TTFBMs()
	if ttfb < 0 || ttfb > streamingMs {
		t.Errorf("ttfb_ms = %.0f, want 0..%.0f", ttfb, streamingMs)
	}

	usage := acc.Finalize()
	if usage.CompletionTokens != 20 {
		t.Errorf("completion_tokens = %d, want 20", usage.CompletionTokens)
	}
	if usage.PromptTokens != 10 {
		t.Errorf("prompt_tokens = %d, want 10", usage.PromptTokens)
	}
}

// TestWriteReassemblesSplitLines covers the reason Write buffers: the network
// splits where it likes, and a usage block cut mid-line must still parse.
func TestWriteReassemblesSplitLines(t *testing.T) {
	acc := NewStreamAccumulator("openai", "")

	full := "data: {\"model\":\"gpt-test\",\"usage\":{\"prompt_tokens\":7,\"completion_tokens\":13,\"total_tokens\":20}}\n"
	for _, part := range []string{full[:20], full[20:45], full[45:]} {
		if _, err := acc.Write([]byte(part)); err != nil {
			t.Fatalf("write: %v", err)
		}
	}

	usage := acc.Finalize()
	if usage.PromptTokens != 7 || usage.CompletionTokens != 13 {
		t.Errorf("got prompt=%d completion=%d, want 7/13", usage.PromptTokens, usage.CompletionTokens)
	}
}

// TestSanitizeUsagePreservesLargeRealPrompts covers the failure mode that made
// the dashboard undercount: long-running agent sessions legitimately report
// hundreds of thousands of input tokens, and a lower ceiling would erase them.
func TestSanitizeUsagePreservesLargeRealPrompts(t *testing.T) {
	in := domain.UsageResult{PromptTokens: 600_000, CompletionTokens: 90, TotalTokens: 600_090}
	got := SanitizeUsage(in)
	if got.PromptTokens != 600_000 || got.CompletionTokens != 90 || got.Estimated {
		t.Fatalf("SanitizeUsage changed a plausible usage block: %+v", got)
	}

	impossible := domain.UsageResult{PromptTokens: 20_000_000, CompletionTokens: 10, TotalTokens: 20_000_010}
	got = SanitizeUsage(impossible)
	if got.PromptTokens != 0 || got.TotalTokens != 10 || !got.Estimated {
		t.Fatalf("SanitizeUsage kept an impossible usage block: %+v", got)
	}
}

func TestParseJSONResponseReadsOpenAIUsageFromCustomProvider(t *testing.T) {
	data := map[string]interface{}{
		"model": "claude-sonnet-5",
		"usage": map[string]interface{}{
			"prompt_tokens":     float64(13),
			"completion_tokens": float64(1),
			"total_tokens":      float64(14),
			"prompt_tokens_details": map[string]interface{}{
				"cached_tokens": float64(3),
			},
		},
	}
	got := ParseJSONResponse("ghostcli", data, "")
	if got.PromptTokens != 13 || got.CompletionTokens != 1 || got.TotalTokens != 14 || got.CacheReadTokens != 3 {
		t.Fatalf("ghostcli usage = %+v", got)
	}
}

func TestFinalizeEstimatesCompletionFromContentDeltas(t *testing.T) {
	acc := NewStreamAccumulator("ghostcli", "")
	chunks := []string{
		"data: {\"model\":\"claude-sonnet-5\",\"choices\":[{\"delta\":{\"content\":\"ok\"}}]}\n",
		"data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}]}\n",
		"data: [DONE]\n",
	}
	for _, chunk := range chunks {
		if _, err := acc.Write([]byte(chunk)); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	got := acc.Finalize()
	if got.CompletionTokens != 1 || got.TotalTokens != 1 || !got.Estimated {
		t.Fatalf("stream usage = %+v", got)
	}
}
