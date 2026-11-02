package proxy

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
)

type traceContextKey struct{}

// traceMiddleware injects W3C traceparent header if absent and stores trace_id in context.
func traceMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		traceparent := r.Header.Get("traceparent")
		var traceID string
		
		if traceparent == "" {
			// Generate W3C traceparent: 00-{trace_id}-{span_id}-01
			traceBytes := make([]byte, 16)
			spanBytes := make([]byte, 8)
			_, _ = rand.Read(traceBytes)
			_, _ = rand.Read(spanBytes)
			
			traceID = hex.EncodeToString(traceBytes)
			spanID := hex.EncodeToString(spanBytes)
			traceparent = "00-" + traceID + "-" + spanID + "-01"
			
			r.Header.Set("traceparent", traceparent)
		} else {
			// Extract trace_id from existing traceparent
			// Format: 00-{trace_id}-{span_id}-{flags}
			if len(traceparent) >= 35 && traceparent[2] == '-' {
				traceID = traceparent[3:35]
			}
		}
		
		ctx := context.WithValue(r.Context(), traceContextKey{}, traceID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// TraceIDFromContext extracts the trace_id stored in the request context.
func TraceIDFromContext(ctx context.Context) string {
	if traceID, ok := ctx.Value(traceContextKey{}).(string); ok {
		return traceID
	}
	return ""
}