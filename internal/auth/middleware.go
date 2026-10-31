package auth

import (
	"context"
	"net/http"
	"strings"
)

// contextKey is private so nothing outside this package can forge the identity.
type contextKey string

const identityContextKey contextKey = "odo_identity"

// Identity is the resolved attribution for one request.
type Identity struct {
	Client  string
	Project string
	// Key is the client key's config when the request authenticated with one;
	// nil means the request used the shared key, a provider token, or nothing.
	Key *Entry
}

// FromRequest returns the identity attached by Attribute, if any.
func FromRequest(r *http.Request) (Identity, bool) {
	id, ok := r.Context().Value(identityContextKey).(Identity)
	return id, ok
}

// Attribute resolves a client key from Authorization: Bearer or X-Api-Key and
// attaches its identity for quota and attribution.
//
// It never rejects. The proxy forwards the caller's own provider credentials in
// Authorization, so an unknown credential is the normal case, not an error;
// /api keeps its own requireAPIKey gate. A request that presents a known client
// key always has its attribution headers overwritten from the key's config: the
// key is the authority, so a caller cannot book spend under another project.
// Unknown credentials — usually provider tokens — pass through untouched.
func Attribute(store *Store, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if store == nil {
			next.ServeHTTP(w, r)
			return
		}
		presented := r.Header.Get("X-Api-Key")
		if presented == "" {
			presented = strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		}
		cfg, ok := store.Resolve(presented)
		if !ok {
			next.ServeHTTP(w, r)
			return
		}
		entry := Entry{
			Key:                 Mask(presented),
			Client:              cfg.Client,
			Project:             cfg.Project,
			DailyCostLimitUSD:   cfg.DailyCostLimitUSD,
			MonthlyCostLimitUSD: cfg.MonthlyCostLimitUSD,
			DailyTokenLimit:     cfg.DailyTokenLimit,
			MonthlyTokenLimit:   cfg.MonthlyTokenLimit,
		}
		if cfg.Client != "" {
			r.Header.Set("X-Odo-Client", cfg.Client)
		}
		if cfg.Project != "" {
			r.Header.Set("X-Odo-Project", cfg.Project)
		}
		ctx := context.WithValue(r.Context(), identityContextKey, Identity{
			Client: cfg.Client, Project: cfg.Project, Key: &entry,
		})
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
