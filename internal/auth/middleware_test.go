package auth

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ens-Emilio/odotoken/internal/config"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	return NewStore(map[string]config.ClientKeyConfig{
		"odt_known_key_123456": {Client: "key-client", Project: "key-project"},
	}, "", slog.New(slog.NewTextHandler(io.Discard, nil)))
}

// A known client key is the authority on attribution: headers the caller sends
// must not let it book spend against another project or client.
func TestAttributeOverridesClientIdentityHeaders(t *testing.T) {
	store := testStore(t)
	var seen http.Header
	h := Attribute(store, http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		seen = r.Header.Clone()
	}))

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	req.Header.Set("Authorization", "Bearer odt_known_key_123456")
	req.Header.Set("X-Odo-Client", "other-client")
	req.Header.Set("X-Odo-Project", "other-project")
	h.ServeHTTP(httptest.NewRecorder(), req)

	if got := seen.Get("X-Odo-Client"); got != "key-client" {
		t.Fatalf("X-Odo-Client = %q, want the key's client", got)
	}
	if got := seen.Get("X-Odo-Project"); got != "key-project" {
		t.Fatalf("X-Odo-Project = %q, want the key's project", got)
	}
}

// An unknown credential is the normal case — the caller's own provider token —
// so it is neither rejected nor attributed, and its headers pass untouched.
func TestAttributeLeavesUnknownCredentialAlone(t *testing.T) {
	store := testStore(t)
	var seen http.Header
	h := Attribute(store, http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		seen = r.Header.Clone()
	}))

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	req.Header.Set("Authorization", "Bearer sk-provider-token")
	req.Header.Set("X-Odo-Client", "other-client")
	req.Header.Set("X-Odo-Project", "other-project")
	h.ServeHTTP(httptest.NewRecorder(), req)

	if got := seen.Get("X-Odo-Client"); got != "other-client" {
		t.Fatalf("X-Odo-Client = %q, want it left untouched", got)
	}
	if got := seen.Get("X-Odo-Project"); got != "other-project" {
		t.Fatalf("X-Odo-Project = %q, want it left untouched", got)
	}
}
