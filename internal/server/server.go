package server

import (
	"crypto/subtle"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/ens-Emilio/odotoken/internal/api"
	"github.com/ens-Emilio/odotoken/internal/auth"
	"github.com/ens-Emilio/odotoken/internal/config"
	"github.com/ens-Emilio/odotoken/internal/db"
	"github.com/ens-Emilio/odotoken/internal/guard"
	"github.com/ens-Emilio/odotoken/internal/importers"
	"github.com/ens-Emilio/odotoken/internal/otel"
	"github.com/ens-Emilio/odotoken/internal/pricing"
	"github.com/ens-Emilio/odotoken/internal/proxy"
)

// Server holds all wired components.
type Server struct {
	DB      *db.Database
	Config  config.Config
	Pricing *pricing.Calculator
	Guard   *guard.Guard
	Otel    *otel.Exporter
	Proxy   *proxy.Proxy
	Router  chi.Router
	Logger  *slog.Logger
}

// New wires all components together.
func New(database *db.Database, cfg config.Config, logger *slog.Logger) *Server {
	return NewWithConfigPath(database, cfg, logger, "config.json")
}

// NewWithConfigPath builds the server and lets the guard API persist changes
// to the exact config file this daemon loaded.
func NewWithConfigPath(database *db.Database, cfg config.Config, logger *slog.Logger, configPath string) *Server {
	calc := pricing.NewCalculator("prices.json")
	g := guard.New(database, cfg.Guard, cfg.Alerts, logger)
	// Nil when otel is disabled or has no endpoint, so callers can skip the
	// worker instead of polling for nothing.
	exporter := otel.New(database, cfg.Otel, logger)
	keys := auth.NewStore(cfg.Server.ClientKeys, configPath, logger)
	p := proxy.New(database, calc, cfg, logger)
	p.SetGuard(g)
	p.SetKeyStore(keys)

	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	// CORS: only origins explicitly allowlisted in config get headers. Empty
	// allowlist means no cross-origin access at all (the SPA is same-origin).
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if origin := r.Header.Get("Origin"); origin != "" && originAllowed(origin, cfg.Server.CORSAllowlist) {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Vary", "Origin")
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Api-Key, X-Odo-Project, X-Odo-Session, X-Odo-Tags")
			}
			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	})

	// Security headers (matching Python middleware)
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("X-Frame-Options", "DENY")
			w.Header().Set("X-XSS-Protection", "1; mode=block")
			next.ServeHTTP(w, r)
		})
	})

	// API auth: when server.api_key is set, /api and /v1 require it. The proxy
	// catch-all is exempt — it forwards the caller's own provider credentials.
	authed := func(h http.Handler) http.Handler {
		if cfg.Server.APIKey == "" {
			return h
		}
		return requireAPIKey(cfg.Server.APIKey, h)
	}

	// API routes
	r.Mount("/v1", authed(api.Router(cfg, calc)))
	dashboard := api.DashboardRouter(database, g.Check, g.Limits, g.UpdateLimits, configPath,
		importers.NewEngine(importers.Paths(cfg.Sessions.Paths), calc, database))
	api.RegisterClientKeyRoutes(dashboard, keys)
	api.RegisterAnalyticsRoutes(dashboard, database, calc, "prices.json")
	r.Mount("/metrics", authed(api.MetricsRouter(database)))
	r.Mount("/api", authed(dashboard))

	// Liveness of odotoken itself. Deliberately unauthenticated and outside the
	// proxy: a probe that needs a secret, spends the rate-limit burst or records
	// token usage starts failing exactly when the operator needs it most.
	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}` + "\n"))
	})

	// Favicon — serve from disk
	r.Get("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		data, err := os.ReadFile("internal/server/static/favicon.svg")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "image/svg+xml")
		w.Write(data)
	})

	// Root redirect → /app/. The query string is forwarded: `?lang=xx` is a
	// shareable link and a redirect that drops it silently loses the language.
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/app/"+querySuffix(r), http.StatusTemporaryRedirect)
	})

	// SPA: /app and /app/* serve index.html or static assets from disk
	// This allows live reload during development without rebuilding the Go binary
	staticDir := "internal/server/static"

	r.Get("/app", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/app/"+querySuffix(r), http.StatusTemporaryRedirect)
	})

	r.Get("/app/*", func(w http.ResponseWriter, r *http.Request) {
		// Strip /app/ prefix to get the asset path within static/
		assetPath := strings.TrimPrefix(r.URL.Path, "/app/")
		assetPath = strings.TrimPrefix(assetPath, "/")

		// Try serving the exact file first
		if assetPath != "" && !strings.HasSuffix(assetPath, "/") {
			fullPath := filepath.Join(staticDir, assetPath)
			// Security: ensure we don't escape staticDir
			cleanFull := filepath.Clean(fullPath)
			cleanStatic := filepath.Clean(staticDir)
			if strings.HasPrefix(cleanFull, cleanStatic) {
				if data, err := os.ReadFile(cleanFull); err == nil {
					ct := guessContentType(assetPath)
					if ct == "" {
						ct = "application/octet-stream"
					}
					w.Header().Set("Content-Type", ct)
					// Cache hashed assets aggressively, but not HTML
					if strings.HasSuffix(assetPath, ".html") {
						w.Header().Set("Cache-Control", "no-cache")
					} else {
						w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
					}
					w.Write(data)
					return
				}
			}
		}

		// Fallback to index.html for SPA client-side routing
		indexHTMLPath := filepath.Join(staticDir, "index.html")
		if data, err := os.ReadFile(indexHTMLPath); err == nil {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Header().Set("Cache-Control", "no-cache")
			w.Write(data)
		} else {
			http.Error(w, "index.html not found. Run 'cd frontend && npm run build' first.", http.StatusNotFound)
		}
	})

	// Proxy catch-all (must be last)
	r.Handle("/*", p.Handler())

	return &Server{
		DB:      database,
		Config:  cfg,
		Pricing: calc,
		Guard:   g,
		Otel:    exporter,
		Proxy:   p,
		Router:  r,
		Logger:  logger,
	}
}

// originAllowed reports whether origin is in the explicit allowlist.
// querySuffix keeps the request's query string across a redirect, prefixed
// with `?` when present.
func querySuffix(r *http.Request) string {
	if r.URL.RawQuery == "" {
		return ""
	}
	return "?" + r.URL.RawQuery
}

func originAllowed(origin string, allowlist []string) bool {
	for _, a := range allowlist {
		if a == "*" || strings.EqualFold(a, origin) {
			return true
		}
	}
	return false
}

// requireAPIKey gates h behind a shared key accepted from either the
// Authorization: Bearer header or X-Api-Key. Compared in constant time.
func requireAPIKey(key string, h http.Handler) http.Handler {
	want := []byte(key)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got := r.Header.Get("X-Api-Key")
		if got == "" {
			got = strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		}
		if subtle.ConstantTimeCompare([]byte(got), want) != 1 {
			w.Header().Set("WWW-Authenticate", `Bearer realm="odotoken"`)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		h.ServeHTTP(w, r)
	})
}

func guessContentType(name string) string {
	switch {
	case strings.HasSuffix(name, ".js"):
		return "application/javascript; charset=utf-8"
	case strings.HasSuffix(name, ".css"):
		return "text/css; charset=utf-8"
	case strings.HasSuffix(name, ".svg"):
		return "image/svg+xml"
	case strings.HasSuffix(name, ".png"):
		return "image/png"
	case strings.HasSuffix(name, ".ico"):
		return "image/x-icon"
	case strings.HasSuffix(name, ".json"):
		return "application/json"
	case strings.HasSuffix(name, ".html"):
		return "text/html; charset=utf-8"
	default:
		return ""
	}
}
