package cache

import (
	"math"
	"sync"
	"time"
)

// Entry holds a cached response with its embedding vector and expiration.
type Entry struct {
	Response  []byte
	Embedding []float64
	ExpiresAt time.Time
}

// SemanticCache provides similarity-based lookup for LLM responses.
type SemanticCache struct {
	mu        sync.RWMutex
	entries   []Entry
	threshold float64
	ttl       time.Duration
	maxSize   int
}

// NewSemanticCache creates a cache with the given cosine similarity threshold,
// TTL and maximum number of entries.
func NewSemanticCache(threshold float64, ttl time.Duration, maxSize int) *SemanticCache {
	if maxSize <= 0 {
		maxSize = 1000
	}
	return &SemanticCache{
		threshold: threshold,
		ttl:       ttl,
		maxSize:   maxSize,
		entries:   make([]Entry, 0, maxSize),
	}
}

// Get returns the cached response whose embedding has the highest cosine
// similarity to query, provided it exceeds the threshold and has not expired.
func (c *SemanticCache) Get(query []float64) ([]byte, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	now := time.Now()
	bestScore := -1.0
	var bestResponse []byte

	for _, e := range c.entries {
		if now.After(e.ExpiresAt) {
			continue
		}
		score := cosineSimilarity(query, e.Embedding)
		if score > bestScore {
			bestScore = score
			bestResponse = e.Response
		}
	}

	if bestScore >= c.threshold && bestResponse != nil {
		return bestResponse, true
	}
	return nil, false
}

// Set stores a response with its embedding vector. Evicts the oldest entry
// when the cache is full.
func (c *SemanticCache) Set(embedding []float64, response []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry := Entry{
		Response:  append([]byte(nil), response...),
		Embedding: append([]float64(nil), embedding...),
		ExpiresAt: time.Now().Add(c.ttl),
	}

	if len(c.entries) >= c.maxSize {
		// Evict expired first, then oldest
		now := time.Now()
		valid := make([]Entry, 0, len(c.entries))
		for _, e := range c.entries {
			if now.Before(e.ExpiresAt) {
				valid = append(valid, e)
			}
		}
		if len(valid) >= c.maxSize {
			c.entries = valid[1:]
		} else {
			c.entries = valid
		}
	}
	c.entries = append(c.entries, entry)
}

// Len returns the current number of entries (including expired ones).
func (c *SemanticCache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}

func cosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	denom := math.Sqrt(normA) * math.Sqrt(normB)
	if denom == 0 {
		return 0
	}
	return dot / denom
}