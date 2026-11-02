package guard

import (
	"regexp"
)

var piiPatterns = []*regexp.Regexp{
	regexp.MustCompile(`\b\d{3}\.\d{3}\.\d{3}-\d{2}\b`),           // CPF
	regexp.MustCompile(`\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}\b`), // email
	regexp.MustCompile(`\bsk-[A-Za-z0-9]{20,}\b`),                 // OpenAI-style key
	regexp.MustCompile(`\bodt_[A-Za-z0-9]{16,}\b`),                // odotoken client key
}

// RedactPII replaces CPFs, emails and API tokens with [REDACTED].
func RedactPII(input []byte) []byte {
	out := input
	for _, re := range piiPatterns {
		out = re.ReplaceAll(out, []byte("[REDACTED]"))
	}
	return out
}