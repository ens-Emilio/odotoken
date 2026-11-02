package tracing

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
)

// SpanContext holds the parsed W3C Trace Context fields.
type SpanContext struct {
	TraceID string
	SpanID  string
	Flags   string
}

// Valid reports whether the span context has non-zero trace and span IDs.
func (s SpanContext) Valid() bool {
	return s.TraceID != "" && s.SpanID != ""
}

// ParseTraceparent extracts trace-id, span-id and flags from a W3C
// traceparent header value. Returns an invalid SpanContext on malformed input.
func ParseTraceparent(header string) SpanContext {
	parts := strings.Split(strings.TrimSpace(header), "-")
	if len(parts) < 4 {
		return SpanContext{}
	}
	traceID := parts[1]
	spanID := parts[2]
	flags := parts[3]

	if len(traceID) != 32 || len(spanID) != 16 {
		return SpanContext{}
	}
	// All-zero trace-id or span-id is invalid per spec
	if traceID == "00000000000000000000000000000000" || spanID == "0000000000000000" {
		return SpanContext{}
	}
	return SpanContext{TraceID: traceID, SpanID: spanID, Flags: flags}
}

// GenerateSpanID returns a random 16-hex-char span identifier.
func GenerateSpanID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// FormatTraceparent builds a W3C traceparent header value from a span context.
func FormatTraceparent(sc SpanContext) string {
	if !sc.Valid() {
		return ""
	}
	flags := sc.Flags
	if flags == "" {
		flags = "01"
	}
	return fmt.Sprintf("00-%s-%s-%s", sc.TraceID, sc.SpanID, flags)
}