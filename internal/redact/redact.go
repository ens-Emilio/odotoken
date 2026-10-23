package redact

import (
	"regexp"
	"strings"
)

var sensitivePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(sk-[a-zA-Z0-9]{20,})`),                          // OpenAI keys
	regexp.MustCompile(`(?i)(sk-ant-[a-zA-Z0-9\-]{20,})`),                    // Anthropic keys
	regexp.MustCompile(`(?i)(AIza[a-zA-Z0-9\-_]{35})`),                       // Google API keys
	regexp.MustCompile(`(?i)(ghp_[a-zA-Z0-9]{36})`),                          // GitHub PAT
	regexp.MustCompile(`(?i)(Bearer\s+[a-zA-Z0-9\-._~+/]+=*)`),              // Bearer tokens
	regexp.MustCompile(`(?i)("api[_-]?key"\s*:\s*"[^"]+")`),                  // JSON api_key fields
	regexp.MustCompile(`(?i)("authorization"\s*:\s*"[^"]+")`),                // JSON auth headers
	regexp.MustCompile(`(?i)(password["\s:=]+[^\s,}"']{4,})`),               // Passwords
}

const replacement = "[REDACTED]"

// Redactor masks sensitive data in text.
type Redactor struct {
	patterns []*regexp.Regexp
}

// New creates a Redactor with default sensitive patterns.
func New() *Redactor {
	return &Redactor{patterns: sensitivePatterns}
}

// Redact replaces all sensitive patterns in text.
func (r *Redactor) Redact(text string) string {
	result := text
	for _, p := range r.patterns {
		result = p.ReplaceAllString(result, replacement)
	}
	return result
}

// RedactHeaders redacts sensitive HTTP header values.
func (r *Redactor) RedactHeaders(headers map[string]string) map[string]string {
	sensitiveKeys := map[string]bool{
		"authorization": true,
		"x-api-key":     true,
		"cookie":        true,
	}
	out := make(map[string]string, len(headers))
	for k, v := range headers {
		if sensitiveKeys[strings.ToLower(k)] {
			out[k] = replacement
		} else {
			out[k] = v
		}
	}
	return out
}