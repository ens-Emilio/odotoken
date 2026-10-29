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
	"sync/atomic"
	"time"

	"golang.org/x/time/rate"

	"github.com/ens-Emilio/odotoken/internal/config"
	"github.com/ens-Emilio/odotoken/internal/db"
	"github.com/ens-Emilio/odotoken/internal/domain"
	"github.com/ens-Emilio/odotoken/internal/guard"
	"github.com/ens-Emilio/odotoken/internal/parser"
	"github.com/ens-Emilio/odotoken/internal/pricing"
	"github.com/google/uuid"
)

// Proxy is a transparent reverse proxy that meters token usage.
type Proxy struct {
	db        *db.Database
	pricing   *pricing.Calculator
	guard     *guard.Guard
	limiter   *rate.Limiter
	cfg       config.Config
	upstreams map[string]*url.URL
	mu        sync.RWMutex
	logger    *slog.Logger
}

func New(database *db.Database, calc *pricing.Calculator, cfg config.Config, logger *slog.Logger) *Proxy {
	p := &Proxy{
		db:      database,
		pricing: calc,
		cfg:     cfg,
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
func (p *Proxy) Handler() http.Handler {
	rp := &httputil.ReverseProxy{
		Director:       p.director,
		ModifyResponse: p.modifyResponse,
		ErrorHandler:   p.errorHandler,
		Transport: &http.Transport{
			Proxy:                 http.ProxyFromEnvironment,
			MaxIdleConns:          64,
			MaxIdleConnsPerHost:   32,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   15 * time.Second,
			ResponseHeaderTimeout: 120 * time.Second,
			ExpectContinueTimeout: time.Second,
			ForceAttemptHTTP2:     true,
		},
	}
	rp.FlushInterval = -1

	var handler http.Handler = rp
	if p.limiter != nil {
		handler = p.rateLimitMiddleware(handler)
	}
	if p.guard != nil {
		handler = p.guardMiddleware(handler)
	}
	// Intercept request body to extract model before proxying
	handler = p.activeRunMiddleware(handler)
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

	// Try to extract provider from path prefix (e.g., /myrelay/chat/completions)
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
				// Strip the provider prefix so the upstream sees its own path
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
		// Reverse lookup: when the openai upstream points at a relay, find the
		// provider name that owns that host
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
	runContextKey      contextKey = "request_run"
)

type requestUsageContext struct {
	model        string
	promptTokens int
	clientApp    string
	projectID    string
	runID        string
	sessionID    string
}

const requestUsageContextKey contextKey = "request_usage"

type requestRunState struct {
	responseStarted atomic.Bool
}

const requestRunStateKey contextKey = "request_run_state"

// extractModelMiddleware reads the request body, extracts the model field,
// and stores it in the request context for use in modifyResponse.
func (p *Proxy) extractModelMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Body == nil {
			next.ServeHTTP(w, r)
			return
		}
		// ponytail: 10 MB cap; LLM request payloads never approach it, so reject instead of spill
		bodyBytes, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 10<<20))
		if err != nil {
			http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
			return
		}
		if len(bodyBytes) == 0 {
			r.Body = io.NopCloser(bytes.NewReader(bodyBytes))
			next.ServeHTTP(w, r)
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(bodyBytes))

		// Try to extract model and prompt size from JSON body.
		var reqBody map[string]interface{}
		usage := requestUsageContext{
			clientApp: parser.IdentifyClientApp(headerMap(r.Header)),
			projectID: r.Header.Get("X-Odo-Project"),
			runID:     uuid.NewString(),
			sessionID: r.Header.Get("X-Odo-Session"),
		}
		if usage.projectID == "" {
			usage.projectID = "default"
		}
		if usage.sessionID == "" {
			usage.sessionID = usage.runID
		}
		if json.Unmarshal(bodyBytes, &reqBody) == nil {
			if model, ok := reqBody["model"].(string); ok && model != "" {
				usage.model = model
				ctx := context.WithValue(r.Context(), modelContextKey, model)
				r = r.WithContext(ctx)
			}
			charsPerToken := p.cfg.TokenCounter.Heuristic.CharsPerToken
			if charsPerToken <= 0 {
				charsPerToken = 4
			}
			usage.promptTokens = int(float64(len(bodyBytes)) / charsPerToken)
		}
		ctx := context.WithValue(r.Context(), requestUsageContextKey, usage)
		ctx = context.WithValue(ctx, requestRunStateKey, &requestRunState{})
		r = r.WithContext(ctx)
		next.ServeHTTP(w, r)
	})
}

// activeRunMiddleware opens a row before the upstream is called. The response
// path closes it; the deferred close covers transport errors and panics.
func (p *Proxy) activeRunMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		usage, ok := r.Context().Value(requestUsageContextKey).(requestUsageContext)
		if !ok || r.Method != http.MethodPost {
			next.ServeHTTP(w, r)
			return
		}
		provider := r.Header.Get("X-Odo-Provider")
		if provider == "" {
			provider = strings.Split(strings.TrimPrefix(r.URL.Path, "/"), "/")[0]
		}
		if err := p.db.StartActiveRun(usage.runID, usage.sessionID, usage.clientApp, provider, usage.model, usage.projectID); err != nil {
			p.logger.Warn("could not start active run", "error", err)
		}
		state, _ := r.Context().Value(requestRunStateKey).(*requestRunState)
		defer func() {
			if rec := recover(); rec != nil {
				_ = p.db.FinishActiveRun(usage.runID, "failed", fmt.Sprint(rec), 0, 0, 0)
				panic(rec)
			}
			if state == nil || !state.responseStarted.Load() {
				_ = p.db.FinishActiveRun(usage.runID, "failed", "request ended without a response", 0, 0, 0)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func (p *Proxy) modifyResponse(resp *http.Response) error {
	startTime := time.Now()
	if state, ok := resp.Request.Context().Value(requestRunStateKey).(*requestRunState); ok {
		state.responseStarted.Store(true)
	}

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

	contentType := resp.Header.Get("Content-Type")
	isStream := strings.Contains(contentType, "text/event-stream")

	if isStream {
		// Keep the upstream body lazy. Reading it here would block the client
		// until the model finished and break streaming semantics.
		acc := parser.NewStreamAccumulator(provider, "")
		countingBody := newCountingReadCloser(resp.Body, acc)
		if requestUsage, ok := resp.Request.Context().Value(requestUsageContextKey).(requestUsageContext); ok {
			countingBody.onRead = func() { _ = p.db.TouchActiveRun(requestUsage.runID) }
		}
		resp.Body = countingBody
		resp.ContentLength = -1
		resp.Header.Del("Content-Length")

		go func() {
			<-countingBody.done
			usage := parser.SanitizeUsage(acc.Finalize())
			if requestUsage, ok := resp.Request.Context().Value(requestUsageContextKey).(requestUsageContext); ok {
				if usage.Model == "" {
					usage.Model = requestUsage.model
				}
				if usage.PromptTokens == 0 && requestUsage.promptTokens > 0 {
					usage.PromptTokens = requestUsage.promptTokens
					usage.Estimated = true
				}
			}
			if !usage.Estimated && usage.CompletionTokens == 0 && resp.StatusCode == 200 {
				usage.CompletionTokens = int(atomic.LoadInt64(&countingBody.bytes) / 4)
				usage.Estimated = true
			}
			usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
			p.recordEvent(resp, provider, clientApp, projectID, usage, acc, startTime)
			if requestUsage, ok := resp.Request.Context().Value(requestUsageContextKey).(requestUsageContext); ok {
				cost := p.pricing.CalculateCost(usage.Model, usage.PromptTokens, usage.CompletionTokens, usage.CacheReadTokens, usage.CacheWriteTokens)
				status := "ok"
				errorText := ""
				if resp.StatusCode >= http.StatusBadRequest {
					status = "failed"
					errorText = fmt.Sprintf("upstream HTTP %d", resp.StatusCode)
				}
				_ = p.db.FinishActiveRun(requestUsage.runID, status, errorText,
					usage.PromptTokens, usage.CompletionTokens, cost)
			}
		}()
		return nil
	}

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, resp.Body); err != nil {
		resp.Body.Close()
		return fmt.Errorf("read response body: %w", err)
	}
	resp.Body.Close()
	body := buf.Bytes()
	var usage domain.UsageResult
	var data map[string]interface{}
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	if dec.Decode(&data) == nil {
		usage = parser.SanitizeUsage(parser.ParseJSONResponse(provider, data, ""))
	}
	if usage.Model == "" {
		if reqModel, ok := resp.Request.Context().Value(modelContextKey).(string); ok && reqModel != "" {
			usage.Model = reqModel
		}
	}
	if usage.TotalTokens == 0 && resp.StatusCode == 200 {
		estimatedCompletion := len(body) / 4
		if estimatedCompletion < 10 {
			estimatedCompletion = 10
		}
		usage.CompletionTokens = estimatedCompletion
		usage.PromptTokens = estimatedCompletion * 2
		usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
		usage.Estimated = true
	}

	if provider == "openai" {
		p.mu.RLock()
		openaiUpstream := p.upstreams["openai"]
		if openaiUpstream != nil && openaiUpstream.Host != "api.openai.com" {
			// The openai upstream points at a relay; report it under the name
			// the operator gave that provider instead of a generic "openai".
			for name, u := range p.upstreams {
				if name != "openai" && u.Host == openaiUpstream.Host {
					provider = name
					break
				}
			}
		}
		p.mu.RUnlock()
	}

	latencyMs := float64(time.Since(startTime).Milliseconds())
	p.recordEventNow(resp, provider, clientApp, projectID, usage, 0, 0, latencyMs)
	if requestUsage, ok := resp.Request.Context().Value(requestUsageContextKey).(requestUsageContext); ok {
		cost := p.pricing.CalculateCost(usage.Model, usage.PromptTokens, usage.CompletionTokens, usage.CacheReadTokens, usage.CacheWriteTokens)
		status := "ok"
		errorText := ""
		if resp.StatusCode >= http.StatusBadRequest {
			status = "failed"
			errorText = fmt.Sprintf("upstream HTTP %d", resp.StatusCode)
		}
		_ = p.db.FinishActiveRun(requestUsage.runID, status, errorText,
			usage.PromptTokens, usage.CompletionTokens, cost)
	}
	resp.Body = io.NopCloser(bytes.NewReader(body))
	resp.ContentLength = int64(len(body))
	resp.Header.Set("Content-Length", fmt.Sprintf("%d", len(body)))
	return nil
}

type countingReadCloser struct {
	io.ReadCloser
	acc       *parser.StreamUsageAccumulator
	bytes     int64
	once      sync.Once
	done      chan struct{}
	onRead    func()
	lastTouch atomic.Int64
}

func newCountingReadCloser(body io.ReadCloser, acc *parser.StreamUsageAccumulator) *countingReadCloser {
	return &countingReadCloser{ReadCloser: body, acc: acc, done: make(chan struct{})}
}

func (c *countingReadCloser) Read(p []byte) (int, error) {
	n, err := c.ReadCloser.Read(p)
	if n > 0 {
		atomic.AddInt64(&c.bytes, int64(n))
		_, _ = c.acc.Write(p[:n])
		if c.onRead != nil {
			now := time.Now().UnixMilli()
			previous := c.lastTouch.Load()
			if now-previous >= 2_000 && c.lastTouch.CompareAndSwap(previous, now) {
				c.onRead()
			}
		}
	}
	if err != nil {
		c.signalDone()
	}
	return n, err
}

func (c *countingReadCloser) Close() error {
	c.signalDone()
	return c.ReadCloser.Close()
}

func (c *countingReadCloser) signalDone() {
	c.once.Do(func() { close(c.done) })
}

func (p *Proxy) recordEvent(resp *http.Response, provider, clientApp, projectID string, usage domain.UsageResult, acc *parser.StreamUsageAccumulator, start time.Time) {
	p.recordEventNow(resp, provider, clientApp, projectID, usage,
		acc.TTFBMs(), acc.StreamingMs(), float64(time.Since(start).Milliseconds()))
}

func (p *Proxy) recordEventNow(resp *http.Response, provider, clientApp, projectID string, usage domain.UsageResult, ttfbMs, streamingMs, latencyMs float64) {
	costUSD := p.pricing.CalculateCost(usage.Model, usage.PromptTokens, usage.CompletionTokens, usage.CacheReadTokens, usage.CacheWriteTokens)
	shadowCost := p.pricing.CalculateShadowCost(usage.Model, usage.PromptTokens, usage.CompletionTokens, usage.CacheReadTokens, usage.CacheWriteTokens)
	metadata := map[string]bool{}
	if usage.Estimated {
		metadata["estimated"] = true
	}
	if usage.Model != "" && !p.pricing.HasExplicitPrice(usage.Model) {
		metadata["price_missing"] = true
	}
	metadataJSON, _ := json.Marshal(metadata)
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
		Metadata:         string(metadataJSON),
		Source:           "proxy",
	}
	go func() {
		if _, insertErr := p.db.InsertEvent(event); insertErr != nil {
			p.logger.Error("failed to insert token event", "error", insertErr)
		}
	}()
}

func (p *Proxy) errorHandler(w http.ResponseWriter, r *http.Request, err error) {
	p.logger.Error("proxy error", "error", err)
	if usage, ok := r.Context().Value(requestUsageContextKey).(requestUsageContext); ok {
		_ = p.db.FinishActiveRun(usage.runID, "failed", err.Error(), 0, 0, 0)
	}
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
		if !result.Allowed && result.ActionOnBreach == "block" {
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
