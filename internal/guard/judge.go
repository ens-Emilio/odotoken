package guard

import (
	"encoding/json"
	"regexp"
	"strconv"
)

var scoreRegex = regexp.MustCompile(`(?i)"score"\s*:\s*([0-9]+(?:\.[0-9]+)?)`)

// JudgeResult holds the extracted feedback score from an LLM response.
type JudgeResult struct {
	Score float64
	Found bool
}

// ExtractJudgeScore parses a JSON or markdown response looking for a numeric
// "score" field. Returns 0 and false when no valid score is present.
func ExtractJudgeScore(response []byte) JudgeResult {
	// Try structured JSON first
	var obj map[string]interface{}
	if err := json.Unmarshal(response, &obj); err == nil {
		if v, ok := obj["score"]; ok {
			switch n := v.(type) {
			case float64:
				return JudgeResult{Score: n, Found: true}
			case string:
				if f, err := strconv.ParseFloat(n, 64); err == nil {
					return JudgeResult{Score: f, Found: true}
				}
			}
		}
	}

	// Fallback: regex extraction for markdown or partial JSON
	matches := scoreRegex.FindSubmatch(response)
	if len(matches) >= 2 {
		if f, err := strconv.ParseFloat(string(matches[1]), 64); err == nil {
			return JudgeResult{Score: f, Found: true}
		}
	}

	return JudgeResult{Score: 0, Found: false}
}