package guard

import (
	"testing"
	"time"
)

func TestCheckKeyPermissions(t *testing.T) {
	tests := []struct {
		name    string
		scope   KeyScope
		req     Request
		wantErr error
	}{
		{"no restrictions", KeyScope{}, Request{Model: "gpt-4", CostUSD: 1.0, Method: "POST"}, nil},
		{"model allowed", KeyScope{AllowedModels: []string{"gpt-4"}}, Request{Model: "gpt-4", Method: "POST"}, nil},
		{"model denied", KeyScope{AllowedModels: []string{"gpt-3.5"}}, Request{Model: "gpt-4", Method: "POST"}, ErrModelNotAllowed},
		{"cost ok", KeyScope{MaxCostUSD: 1.0}, Request{CostUSD: 0.5, Method: "POST"}, nil},
		{"cost exceeded", KeyScope{MaxCostUSD: 0.5}, Request{CostUSD: 1.0, Method: "POST"}, ErrCostExceeded},
		{"read only get", KeyScope{ReadOnly: true}, Request{Method: "GET"}, nil},
		{"read only post", KeyScope{ReadOnly: true}, Request{Method: "POST"}, ErrReadOnly},
		{"expired", KeyScope{ExpiresAt: time.Now().Add(-time.Hour)}, Request{Method: "GET"}, ErrKeyExpired},
		{"not expired", KeyScope{ExpiresAt: time.Now().Add(time.Hour)}, Request{Method: "GET"}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CheckKeyPermissions(tt.scope, tt.req)
			if err != tt.wantErr {
				t.Errorf("CheckKeyPermissions() = %v, want %v", err, tt.wantErr)
			}
		})
	}
}