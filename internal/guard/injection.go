package guard

import (
	"strings"
)

var injectionPatterns = []string{
	"ignore previous instructions",
	"ignore all prior",
	"disregard previous",
	"forget your rules",
	"you are now",
	"new instructions:",
	"system prompt:",
	"<|im_start|>",
	"[inst]",
	"### instruction",
}

// DetectPromptInjection returns true if the input contains known jailbreak patterns.
func DetectPromptInjection(input string) bool {
	lower := strings.ToLower(input)
	for _, p := range injectionPatterns {
		if strings.Contains(lower, p) {
			return true
		}
	}
	return false
}