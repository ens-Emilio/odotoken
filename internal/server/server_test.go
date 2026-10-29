package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// A `?lang=xx` link is how a shared dashboard opens in a specific language, so
// the root redirect must not drop the query on the floor.
func TestRootRedirectKeepsQuery(t *testing.T) {
	cases := []struct {
		name   string
		target string
		want   string
	}{
		{"no query", "/", "/app/"},
		{"language", "/?lang=ar", "/app/?lang=ar"},
		{"language and view", "/?lang=de&view=guard", "/app/?lang=de&view=guard"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, tc.target, nil)
			http.Redirect(rec, req, "/app/"+querySuffix(req), http.StatusTemporaryRedirect)
			if got := rec.Header().Get("Location"); got != tc.want {
				t.Fatalf("Location = %q, want %q", got, tc.want)
			}
		})
	}
}
