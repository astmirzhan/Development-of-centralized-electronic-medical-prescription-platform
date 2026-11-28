package httpserver

import (
	"context"
	"log/slog"
	"net/http"
	"slices"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// DBPinger checks PostgreSQL health.
type DBPinger interface {
	Ping(ctx context.Context) error
}

// RouterDeps holds dependencies wired into the HTTP router.
type RouterDeps struct {
	Logger             *slog.Logger
	DB                 DBPinger
	CORSAllowedOrigins []string
	AuthMiddleware     func(http.Handler) http.Handler
	RegisterRoutes     func(r chi.Router)
}

// NewRouter constructs the chi router with standard middlewares and /health endpoint.
func NewRouter(deps RouterDeps) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(corsMiddleware(deps.CORSAllowedOrigins))
	r.Use(slogMiddleware(deps.Logger))
	r.Use(middleware.Recoverer)

	r.Get("/health", healthHandler(deps.DB))

	if deps.RegisterRoutes != nil {
		r.Route("/api/v1", func(api chi.Router) {
			if deps.AuthMiddleware != nil {
				api.Use(deps.AuthMiddleware)
			}
			deps.RegisterRoutes(api)
		})
	}

	return r
}

func healthHandler(db DBPinger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		now := time.Now().UTC().Format(time.RFC3339)
		if db == nil {
			JSON(w, http.StatusServiceUnavailable, map[string]string{
				"status":    "degraded",
				"database":  "disconnected",
				"timestamp": now,
			})
			return
		}

		pingCtx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		if err := db.Ping(pingCtx); err != nil {
			JSON(w, http.StatusServiceUnavailable, map[string]string{
				"status":    "degraded",
				"database":  "disconnected",
				"timestamp": now,
			})
			return
		}

		JSON(w, http.StatusOK, map[string]string{
			"status":    "ok",
			"database":  "connected",
			"timestamp": now,
		})
	}
}

func slogMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r)

			logger.Info("http_request",
				"request_id", middleware.GetReqID(r.Context()),
				"method", r.Method,
				"path", r.URL.Path,
				"status", ww.Status(),
				"bytes", ww.BytesWritten(),
				"duration_ms", time.Since(start).Milliseconds(),
				"remote_ip", r.RemoteAddr,
			)
		})
	}
}

func corsMiddleware(allowedOrigins []string) func(http.Handler) http.Handler {
	wildcard := slices.Contains(allowedOrigins, "*")
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			allowed := false

			if wildcard {
				// Wildcard origin must never be combined with Allow-Credentials.
				w.Header().Set("Access-Control-Allow-Origin", "*")
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Accept, Authorization, Content-Type, X-Request-ID")
				allowed = true
			} else if origin != "" && slices.Contains(allowedOrigins, origin) {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Vary", "Origin")
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Accept, Authorization, Content-Type, X-Request-ID")
				w.Header().Set("Access-Control-Allow-Credentials", "true")
				allowed = true
			}

			if r.Method == http.MethodOptions {
				if !allowed {
					w.WriteHeader(http.StatusForbidden)
					return
				}
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
