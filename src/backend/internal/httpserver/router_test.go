package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
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
