package guard

import (
	"sync"
	"time"
)

// IdleTracker monitors last activity per session/client to support auto-pause.
type IdleTracker struct {
	mu       sync.RWMutex
	lastSeen map[string]time.Time
	timeout  time.Duration
}

// NewIdleTracker creates a tracker with the given idle threshold.
func NewIdleTracker(timeout time.Duration) *IdleTracker {
	return &IdleTracker{
		lastSeen: make(map[string]time.Time),
		timeout:  timeout,
	}
}

// RecordActivity updates the last seen timestamp for an entity.
func (t *IdleTracker) RecordActivity(entityID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.lastSeen[entityID] = time.Now()
}

// IsIdle returns true when the entity has no activity within the configured timeout.
// Unknown entities are considered idle.
func (t *IdleTracker) IsIdle(entityID string) bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	last, ok := t.lastSeen[entityID]
	if !ok {
		return true
	}
	return time.Since(last) > t.timeout
}

// Prune removes entries older than the timeout to bound memory growth.
func (t *IdleTracker) Prune() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	cutoff := time.Now().Add(-t.timeout)
	pruned := 0
	for id, ts := range t.lastSeen {
		if ts.Before(cutoff) {
			delete(t.lastSeen, id)
			pruned++
		}
	}
	return pruned
}