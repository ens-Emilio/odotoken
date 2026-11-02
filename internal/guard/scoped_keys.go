package guard

import (
	"errors"
	"time"
)

// KeyScope defines granular permissions for a client key.
type KeyScope struct {
	AllowedModels []string  `json:"allowed_models,omitempty"`
	MaxCostUSD    float64   `json:"max_cost_usd,omitempty"`
	ExpiresAt     time.Time `json:"expires_at,omitempty"`
	ReadOnly      bool      `json:"read_only,omitempty"`
}

// Request represents an incoming proxy request for permission checking.
type Request struct {
	Model    string
	CostUSD  float64
	Method   string
}

var (
	ErrKeyExpired     = errors.New("key expired")
	ErrModelNotAllowed = errors.New("model not allowed")
	ErrCostExceeded   = errors.New("cost limit exceeded")
	ErrReadOnly       = errors.New("key is read-only")
)

// CheckKeyPermissions validates a request against the key's scope.
func CheckKeyPermissions(scope KeyScope, req Request) error {
	if !scope.ExpiresAt.IsZero() && time.Now().After(scope.ExpiresAt) {
		return ErrKeyExpired
	}
	if scope.ReadOnly && req.Method != "GET" {
		return ErrReadOnly
	}
	if len(scope.AllowedModels) > 0 {
		allowed := false
		for _, m := range scope.AllowedModels {
			if m == req.Model {
				allowed = true
				break
			}
		}
		if !allowed {
			return ErrModelNotAllowed
		}
	}
	if scope.MaxCostUSD > 0 && req.CostUSD > scope.MaxCostUSD {
		return ErrCostExceeded
	}
	return nil
}