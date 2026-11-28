package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestJWTSignParseAndMiddleware(t *testing.T) {
	const secret = "test-jwt-signing-secret-32-chars-min"
	actor := ActorContext{
		UserID: uuid.New(),
		IIN:    "840315301294",
		Role:   RoleDoctor,
	}

	token, err := SignAccessToken(secret, actor, 15*time.Minute)
	if err != nil {
		t.Fatalf("SignAccessToken error: %v", err)
	}

	parsed, err := ParseAccessToken(secret, token, time.Now().UTC())
	if err != nil {
		t.Fatalf("ParseAccessToken error: %v", err)
	}
	if parsed != actor {
		t.Fatalf("expected %#v, got %#v", actor, parsed)
	}

	// Wrong secret must fail.
	if _, err := ParseAccessToken("wrong-secret", token, time.Now().UTC()); err == nil {
		t.Fatal("expected error with wrong signing key")
	}

	// Expired token must fail.
	if _, err := ParseAccessToken(secret, token, time.Now().UTC().Add(16*time.Minute)); err == nil {
		t.Fatal("expected error for expired token")
	}

	protected := JWTMiddleware(secret)(RequireRole(RoleDoctor, RoleAdmin)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, ok := ActorFromContext(r.Context())
		if !ok || got.UserID != actor.UserID {
			t.Fatalf("expected actor in context, got %#v", got)
		}
		w.WriteHeader(http.StatusOK)
	})))

	// Missing Authorization header -> 401.
	rec401 := httptest.NewRecorder()
	protected.ServeHTTP(rec401, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec401.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec401.Code)
	}

	// Valid doctor token -> 200.
	reqOK := httptest.NewRequest(http.MethodGet, "/", nil)
	reqOK.Header.Set("Authorization", "Bearer "+token)
	recOK := httptest.NewRecorder()
	protected.ServeHTTP(recOK, reqOK)
	if recOK.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recOK.Code)
	}

	// Patient token against doctor/admin route -> 403.
	patientToken, err := SignAccessToken(secret, ActorContext{UserID: uuid.New(), IIN: "981105303419", Role: RolePatient}, 15*time.Minute)
	if err != nil {
		t.Fatalf("SignAccessToken patient: %v", err)
	}
	req403 := httptest.NewRequest(http.MethodGet, "/", nil)
	req403.Header.Set("Authorization", "Bearer "+patientToken)
	rec403 := httptest.NewRecorder()
	protected.ServeHTTP(rec403, req403)
	if rec403.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec403.Code)
	}
}
