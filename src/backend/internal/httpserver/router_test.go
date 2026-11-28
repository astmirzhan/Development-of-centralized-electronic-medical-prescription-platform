package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/xjam-byte/erx-platform/backend/internal/auth"
)

type stubPinger struct {
	err error
}

func (s stubPinger) Ping(_ context.Context) error {
	return s.err
}

func TestHealthEndpoint(t *testing.T) {
	t.Run("healthy database returns 200", func(t *testing.T) {
		router := NewRouter(RouterDeps{
			DB:                 stubPinger{err: nil},
			CORSAllowedOrigins: []string{"http://localhost:3000"},
		})

		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		req.Header.Set("Origin", "http://localhost:3000")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", rec.Code)
		}
		if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:3000" {
			t.Fatalf("expected CORS header for localhost:3000, got %q", got)
		}

		var body map[string]string
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("unmarshal response: %v", err)
		}
		if body["status"] != "ok" || body["database"] != "connected" || body["timestamp"] == "" {
			t.Fatalf("unexpected health body: %#v", body)
		}
	})

	t.Run("unreachable database returns 503", func(t *testing.T) {
		router := NewRouter(RouterDeps{
			DB: stubPinger{err: errors.New("connection refused")},
		})

		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("expected 503 Service Unavailable, got %d", rec.Code)
		}

		var body map[string]string
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("unmarshal response: %v", err)
		}
		if body["status"] != "degraded" || body["database"] != "disconnected" {
			t.Fatalf("unexpected degraded health body: %#v", body)
		}
	})
}

func TestCORSPolicyAndAuthMiddleware(t *testing.T) {
	t.Run("wildcard CORS emits literal * without Allow-Credentials", func(t *testing.T) {
		router := NewRouter(RouterDeps{
			DB:                 stubPinger{},
			CORSAllowedOrigins: []string{"*"},
		})

		req := httptest.NewRequest(http.MethodOptions, "/health", nil)
		req.Header.Set("Origin", "https://evil.example")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusNoContent {
			t.Fatalf("expected 204, got %d", rec.Code)
		}
		if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "*" {
			t.Fatalf("expected Access-Control-Allow-Origin=*, got %q", got)
		}
		if got := rec.Header().Get("Access-Control-Allow-Credentials"); got != "" {
			t.Fatalf("expected no Allow-Credentials with wildcard, got %q", got)
		}
	})

	t.Run("disallowed origin preflight returns 403", func(t *testing.T) {
		router := NewRouter(RouterDeps{
			DB:                 stubPinger{},
			CORSAllowedOrigins: []string{"http://localhost:3000"},
		})

		req := httptest.NewRequest(http.MethodOptions, "/health", nil)
		req.Header.Set("Origin", "https://evil.example")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403 for disallowed preflight origin, got %d", rec.Code)
		}
		if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
			t.Fatalf("expected no Access-Control-Allow-Origin header, got %q", got)
		}
	})

	t.Run("/api/v1 enforces AuthMiddleware while /health stays public", func(t *testing.T) {
		router := NewRouter(RouterDeps{
			DB:             stubPinger{},
			AuthMiddleware: auth.JWTMiddleware("test-signing-key-32-chars-minimum"),
			RegisterRoutes: func(r chi.Router) {
				r.Get("/ping", func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusOK)
				})
			},
		})

		healthRec := httptest.NewRecorder()
		router.ServeHTTP(healthRec, httptest.NewRequest(http.MethodGet, "/health", nil))
		if healthRec.Code != http.StatusOK {
			t.Fatalf("expected public /health 200, got %d", healthRec.Code)
		}

		apiRec := httptest.NewRecorder()
		router.ServeHTTP(apiRec, httptest.NewRequest(http.MethodGet, "/api/v1/ping", nil))
		if apiRec.Code != http.StatusUnauthorized {
			t.Fatalf("expected unauthenticated /api/v1/ping 401, got %d", apiRec.Code)
		}
	})
}
