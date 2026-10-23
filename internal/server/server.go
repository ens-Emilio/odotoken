package server

import (
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/user/odotoken/internal/api"
	"github.com/user/odotoken/internal/config"
	"github.com/user/odotoken/internal/db"
	"github.com/user/odotoken/internal/guard"
	"github.com/user/odotoken/internal/pricing"
	"github.com/user/odotoken/internal/proxy"
)

// Server holds all wired components.
type Server struct {
	DB      *db.Database
	Config  config.Config
	Pricing *pricing.Calculator
	Guard   *guard.Guard
	Proxy   *proxy.Proxy
	Router  chi.Router
	Logger  *slog.Logger
}

// New wires all components together.
func New(database *db.Database, cfg config.Config, logger *slog.Logger) *Server {
	calc := pricing.NewCalculator("prices.json")
	g := guard.New(database, cfg.Guard, cfg.Alerts, logger)
	p := proxy.New(database, calc, cfg, logger)
	p.SetGuard(g)

	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Odo-Project, X-Odo-Tags")
			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
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

	// API routes
	r.Mount("/v1", api.Router(cfg, calc))
	r.Mount("/api", api.DashboardRouter(database))

	// Favicon — serve from disk
	r.Get("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		data, err := os.ReadFile("src/odotoken/static/favicon.svg")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "image/svg+xml")
		w.Write(data)
	})

	// Root redirect → /app/
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/app/", http.StatusTemporaryRedirect)
	})

	// SPA: /app and /app/* serve index.html or static assets from disk
	// This allows live reload during development without rebuilding the Go binary
	staticDir := "src/odotoken/static"

	r.Get("/app", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/app/", http.StatusTemporaryRedirect)
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
		Proxy:   p,
		Router:  r,
		Logger:  logger,
	}
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