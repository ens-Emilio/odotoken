package proxy

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// fallbackTransport retries a request against the next configured provider
// when the first one answers a transient status. The retry happens before the
// client sees a single byte, so SSE and JSON responses are unaffected.
type fallbackTransport struct {
	base  http.RoundTripper
	proxy *Proxy
}

func (t *fallbackTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.base.RoundTrip(req)
	provider, _ := req.Context().Value(providerContextKey).(string)
	targets := t.proxy.fallbacksFor(provider)
	if len(targets) == 0 || !t.proxy.shouldFallback(provider, resp, err) {
		return resp, err
	}
	if resp != nil {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4<<10))
		resp.Body.Close()
	}

	originalPath, _ := req.Context().Value(originalPathContextKey).(string)
	if originalPath == "" {
		originalPath = req.URL.Path
	}

	var lastErr error = err
	for _, target := range targets {
		retry, cloneErr := retryRequest(req, target, originalPath)
		if cloneErr != nil {
			lastErr = cloneErr
			continue
		}
		t.proxy.logger.Warn("provider fallback",
			"from", provider, "to", target.provider, "reason", fallbackReason(resp, err))
		next, retryErr := t.base.RoundTrip(retry)
		if retryErr != nil {
			lastErr = retryErr
			continue
		}
		if !t.proxy.shouldFallback(target.provider, next, nil) {
			return next, nil
		}
		_, _ = io.Copy(io.Discard, io.LimitReader(next.Body, 4<<10))
		next.Body.Close()
		lastErr = fmt.Errorf("fallback %s answered %d", target.provider, next.StatusCode)
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("all provider fallbacks failed")
	}
	return nil, lastErr
}

// retryRequest rebuilds the outbound request for another provider. The body is
// replayed from GetBody, which only exists because the proxy buffers request
// bodies; without it a retry would send an empty body.
func retryRequest(req *http.Request, target fallbackTarget, originalPath string) (*http.Request, error) {
	ctx := context.WithValue(req.Context(), providerContextKey, target.provider)
	retry := req.Clone(ctx)
	retry.URL = cloneURL(req.URL)
	retry.URL.Scheme = target.url.Scheme
	retry.URL.Host = target.url.Host
	retry.URL.Path = singleJoiningSlash(target.url.Path, originalPath)
	retry.Host = target.url.Host
	if req.Body == nil || req.Body == http.NoBody {
		return retry, nil
	}
	if req.GetBody == nil {
		return nil, fmt.Errorf("cannot replay %s request body for fallback", req.Method)
	}
	body, err := req.GetBody()
	if err != nil {
		return nil, err
	}
	retry.Body = body
	return retry, nil
}

func cloneURL(u *url.URL) *url.URL {
	clone := *u
	return &clone
}

func (p *Proxy) fallbacksFor(provider string) []fallbackTarget {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.fallbacks[provider]
}

// shouldFallback reports whether a transient status (or a transport error on a
// replayable request) justifies trying the next provider.
func (p *Proxy) shouldFallback(provider string, resp *http.Response, err error) bool {
	if err != nil {
		return true
	}
	if resp == nil {
		return false
	}
	p.mu.RLock()
	statuses := p.fallbackStatus[provider]
	p.mu.RUnlock()
	return statuses[resp.StatusCode]
}

func fallbackReason(resp *http.Response, err error) string {
	if err != nil {
		return err.Error()
	}
	if resp != nil {
		return fmt.Sprintf("upstream HTTP %d", resp.StatusCode)
	}
	return "unknown"
}
