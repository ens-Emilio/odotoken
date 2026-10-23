package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"

	"github.com/ens-Emilio/odotoken/internal/config"
	"github.com/ens-Emilio/odotoken/internal/db"
	"github.com/ens-Emilio/odotoken/internal/domain"
	"github.com/ens-Emilio/odotoken/internal/parser"
	"github.com/ens-Emilio/odotoken/internal/pricing"
	"github.com/ens-Emilio/odotoken/internal/redact"
	"github.com/ens-Emilio/odotoken/internal/guard"
)

// Proxy is a transparent reverse proxy that meters token usage.
type Proxy struct {
	db        *db.Database
	pricing   *pricing.Calculator
	redactor  *redact.Redactor
	guard     *guard.Guard
	limiter   *rate.Limiter
	cfg       config.Config
	upstreams map[string]*url.URL
	mu        sync.RWMutex
	logger    *slog.Logger
}

// New creates a new metering proxy.
func New(database *db.Database, calc *pricing.Calculator, cfg config.Config, logger *slog.Logger) *Proxy {
	p := &Proxy{
		db:       database,
		pricing:  calc,
		redactor: redact.New(),
		cfg:      cfg,
		upstreams: map[string]*url.URL{
			"anthropic": mustParseURL("https://api.anthropic.com"),
			"openai":    mustParseURL("https://api.openai.com"),
			"gemini":    mustParseURL("https://generativelanguage.googleapis.com"),
			"ollama":    mustParseURL("http://localhost:11434"),
		},
		logger: logger,
	}

	// Load custom providers from config
	for name, providerCfg := range cfg.Providers {
		if providerCfg.UpstreamURL != "" {
			p.upstreams[name] = mustParseURL(providerCfg.UpstreamURL)
		}
	}

	if rl := cfg.Limits.RateLimit; rl.Enabled && rl.RPS > 0 {
		burst := rl.BurstSize
		if burst <= 0 {
			burst = int(rl.RPS)
		}
		p.limiter = rate.NewLimiter(rate.Limit(rl.RPS), burst)
	}

	return p
}

// Handler returns the HTTP handler for the proxy.
func (p *Proxy) Handler() http.Handler {
	rp := &httputil.ReverseProxy{
		Director:       p.director,
		ModifyResponse: p.modifyResponse,
		ErrorHandler:   p.errorHandler,
	}

	var handler http.Handler = rp
	if p.limiter != nil {
		handler = p.rateLimitMiddleware(handler)
	}
	if p.guard != nil {
		handler = p.guardMiddleware(handler)
	}
	// Intercept request body to extract model before proxying
	handler = p.extractModelMiddleware(handler)
	return handler
}

func (p *Proxy) director(req *http.Request) {
	// Check for explicit provider header first
	explicitProvider := req.Header.Get("X-Odo-Provider")

	var provider string
	var upstream *url.URL

	if explicitProvider != "" {
		// Use explicit provider if specified
		p.mu.RLock()
		var ok bool
		upstream, ok = p.upstreams[explicitProvider]
		p.mu.RUnlock()
		if ok {
			provider = explicitProvider
		}
	}

	// Try to extract provider from path prefix (e.g., /kodeo/chat/completions → kodeo)
	if upstream == nil {
		pathParts := strings.Split(strings.TrimPrefix(req.URL.Path, "/"), "/")
		if len(pathParts) > 0 {
			pathProvider := pathParts[0]
			p.mu.RLock()
			var ok bool
			upstream, ok = p.upstreams[pathProvider]
			p.mu.RUnlock()
			if ok {
				provider = pathProvider
				// Strip provider prefix from path for upstream (e.g., /kodeo/chat/completions → /chat/completions)
				req.URL.Path = "/" + strings.Join(pathParts[1:], "/")
			}
		}
	}

	if upstream == nil {
		// Fallback: try path-based detection
		detectedProvider := parser.IdentifyProvider(req.URL.Path, headerMap(req.Header))
		p.mu.RLock()
		var ok bool
		upstream, ok = p.upstreams[detectedProvider]
		p.mu.RUnlock()
		if ok {
			provider = detectedProvider
		}
	}

	if upstream == nil {
		// Final fallback: find which provider matches the configured openai upstream
		p.mu.RLock()
		defaultUpstream := p.upstreams["openai"]
		// Reverse lookup: if openai upstream points to a custom host (e.g., kodeo),
		// find the provider name that owns that upstream
		for name, u := range p.upstreams {
			if name != "openai" && u.Host == defaultUpstream.Host && u.Scheme == defaultUpstream.Scheme {
				provider = name
				upstream = u
				break
			}
		}
		if upstream == nil {
			upstream = defaultUpstream
			provider = "openai"
		}
		p.mu.RUnlock()
	}

	// Concatenate upstream base path with request path
	// e.g., upstream.Path="/v1" + req.Path="/chat/completions" → "/v1/chat/completions"
	req.URL.Path = singleJoiningSlash(upstream.Path, req.URL.Path)
	req.URL.Scheme = upstream.Scheme
	req.URL.Host = upstream.Host
	req.Host = upstream.Host

	// Store the actual provider in context for modifyResponse
	ctx := context.WithValue(req.Context(), providerContextKey, provider)
	*req = *req.WithContext(ctx)

	if h := req.Header.Get("Te"); h == "trailers" {
		req.Header.Del("Te")
	}
}

type contextKey string

const (
	modelContextKey    contextKey = "request_model"
	providerContextKey contextKey = "request_provider"
)

// extractModelMiddleware reads the request body, extracts the model field,
// and stores it in the request context for use in modifyResponse.
func (p *Proxy) extractModelMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Body == nil {
			next.ServeHTTP(w, r)
			return
		}
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil || len(bodyBytes) == 0 {
			r.Body = io.NopCloser(bytes.NewReader(bodyBytes))
			next.ServeHTTP(w, r)
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(bodyBytes))

		// Try to extract model from JSON body
		var reqBody map[string]interface{}
		if json.Unmarshal(bodyBytes, &reqBody) == nil {
			if model, ok := reqBody["model"].(string); ok && model != "" {
				ctx := context.WithValue(r.Context(), modelContextKey, model)
				r = r.WithContext(ctx)
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (p *Proxy) modifyResponse(resp *http.Response) error {
	startTime := time.Now()
	
	// Get provider from context (set by director) or fallback to path detection
	var provider string
	if ctxProvider, ok := resp.Request.Context().Value(providerContextKey).(string); ok && ctxProvider != "" {
		provider = ctxProvider
	} else {
		provider = parser.IdentifyProvider(resp.Request.URL.Path, headerMap(resp.Request.Header))
	}
	
	clientApp := parser.IdentifyClientApp(headerMap(resp.Request.Header))
	projectID := resp.Request.Header.Get("X-Odo-Project")
	if projectID == "" {
		projectID = "default"
	}

	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return fmt.Errorf("read response body: %w", err)
	}

	contentType := resp.Header.Get("Content-Type")
	isStream := strings.Contains(contentType, "text/event-stream")

	var usage domain.UsageResult
	var ttfbMs, streamingMs float64

	if isStream {
		acc := parser.NewStreamAccumulator(provider, "")
		lines := bytes.Split(body, []byte("\n"))
		for _, line := range lines {
			acc.ProcessChunk(line)
		}
		usage = acc.Finalize()
		ttfbMs = acc.TTFBMs()
		streamingMs = acc.StreamingMs()
	} else {
		var data map[string]interface{}
		dec := json.NewDecoder(bytes.NewReader(body))
		dec.UseNumber()
		if dec.Decode(&data) == nil {
			usage = parser.ParseJSONResponse(provider, data, "")
		}
	}
	// Fallback: use model from request context if response didn't provide it
	if usage.Model == "" {
		if reqModel, ok := resp.Request.Context().Value(modelContextKey).(string); ok && reqModel != "" {
			usage.Model = reqModel
		}
	}

	// Heuristic fallback: if tokens are still 0 but we have a valid response, estimate based on body size
	// This handles relays (Kodeo, Pegasus) that don't return usage blocks in streams
	if usage.TotalTokens == 0 && resp.StatusCode == 200 {
		// Estimate completion tokens from response body (approx 4 chars per token)
		estimatedCompletion := len(body) / 4
		if estimatedCompletion < 10 {
			estimatedCompletion = 10 // Minimum floor for valid responses
		}

		usage.CompletionTokens = estimatedCompletion
		// Default assumption: prompt is roughly 2x completion for coding tasks
		usage.PromptTokens = estimatedCompletion * 2
		usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
	}

	// If provider is still "openai" (default), check if openai upstream is a custom relay
	// If so, use kodeo as the default custom provider (not random map iteration)
	if provider == "openai" {
		p.mu.RLock()
		openaiUpstream := p.upstreams["openai"]
		// If openai upstream is not api.openai.com, it's a custom relay
		if openaiUpstream != nil && openaiUpstream.Host != "api.openai.com" {
			// Prefer kodeo if configured, otherwise use first custom provider
			if _, ok := p.cfg.Providers["kodeo"]; ok {
				provider = "kodeo"
			} else {
				for name := range p.cfg.Providers {
					if name != "openai" && name != "anthropic" && name != "gemini" && name != "ollama" {
						provider = name
						break
					}
				}
			}
		}
		p.mu.RUnlock()
	}

	latencyMs := float64(time.Since(startTime).Milliseconds())
	costUSD := p.pricing.CalculateCost(usage.Model, usage.PromptTokens, usage.CompletionTokens, usage.CacheReadTokens, usage.CacheWriteTokens)
	shadowCost := p.pricing.CalculateShadowCost(usage.Model, usage.PromptTokens, usage.CompletionTokens, usage.CacheReadTokens, usage.CacheWriteTokens)

	event := domain.TokenEvent{
		Timestamp:        time.Now().UTC(),
		Provider:         provider,
		Model:            usage.Model,
		ClientApp:        clientApp,
		PromptTokens:     usage.PromptTokens,
		CompletionTokens: usage.CompletionTokens,
		TotalTokens:      usage.TotalTokens,
		CacheReadTokens:  usage.CacheReadTokens,
		CacheWriteTokens: usage.CacheWriteTokens,
		CostUSD:          costUSD,
		ShadowCost:       shadowCost,
		LatencyMs:        latencyMs,
		TTFBMs:           ttfbMs,
		StreamingMs:      streamingMs,
		StatusCode:       resp.StatusCode,
		ProjectID:        projectID,
		Tags:             resp.Request.Header.Get("X-Odo-Tags"),
		Metadata:         "{}",
		Source:           "proxy",
	}

	go func() {
		if _, insertErr := p.db.InsertEvent(event); insertErr != nil {
			p.logger.Error("failed to insert token event", "error", insertErr)
		}
	}()

	resp.Body = io.NopCloser(bytes.NewReader(body))
	resp.ContentLength = int64(len(body))
	resp.Header.Set("Content-Length", fmt.Sprintf("%d", len(body)))
	return nil
}

func (p *Proxy) errorHandler(w http.ResponseWriter, _ *http.Request, err error) {
	p.logger.Error("proxy error", "error", err)
	http.Error(w, "Bad Gateway", http.StatusBadGateway)
}

func (p *Proxy) rateLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !p.limiter.Allow() {
			http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// SetGuard attaches a budget guard to the proxy.
func (p *Proxy) SetGuard(g *guard.Guard) {
	p.guard = g
}

// guardMiddleware checks budget limits before forwarding requests.
func (p *Proxy) guardMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		projectID := r.Header.Get("X-Odo-Project")
		if projectID == "" {
			projectID = "default"
		}
		result := p.guard.Check(projectID)
		if !result.Allowed {
			http.Error(w, result.Message, http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// SetUpstream overrides the upstream URL for a provider.
func (p *Proxy) SetUpstream(provider string, rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return err
	}
	p.mu.Lock()
	p.upstreams[provider] = u
	p.mu.Unlock()
	return nil
}

func headerMap(h http.Header) map[string]string {
	m := make(map[string]string, len(h))
	for k := range h {
		m[strings.ToLower(k)] = h.Get(k)
	}
	return m
}

func mustParseURL(raw string) *url.URL {
	u, err := url.Parse(raw)
	if err != nil {
		panic(err)
	}
	return u
}

// singleJoiningSlash joins two URL path segments with a single slash.
func singleJoiningSlash(a, b string) string {
	aslash := strings.HasSuffix(a, "/")
	bslash := strings.HasPrefix(b, "/")
	switch {
	case aslash && bslash:
		return a + b[1:]
	case !aslash && !bslash:
		return a + "/" + b
	}
	return a + b
}