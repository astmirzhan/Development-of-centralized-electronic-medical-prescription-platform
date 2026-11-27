package qr

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/xjam-byte/erx-platform/backend/internal/auth"
	"github.com/xjam-byte/erx-platform/backend/internal/httpserver"
)

func TestQRTokenContainsZeroPII(t *testing.T) {
	patientID := uuid.New()
	itemID := uuid.New()
	repo := &stubQRRepo{
		eligibility: &ItemEligibility{
			PrescriptionItemID: itemID,
			PrescriptionID:     uuid.New(),
			PatientID:          patientID,
			PrescriptionStatus: "active",
			ValidUntil:         time.Now().UTC().Add(24 * time.Hour),
			ItemStatus:         "pending",
			QuantityPrescribed: 10,
			QuantityDispensed:  0,
		},
	}

	svc := NewService(repo, 0, 0) // also tests default 60s/90s fallback
	resp, err := svc.IssueToken(context.Background(), itemID, patientID)
	if err != nil {
		t.Fatalf("IssueToken error: %v", err)
	}

	forbiddenPII := []string{
		"981105303419",
		"Sultanov",
		"Amoxicillin",
		"J18.9",
		patientID.String(),
		itemID.String(),
	}
	for _, s := range forbiddenPII {
		if strings.Contains(strings.ToLower(resp.Token), strings.ToLower(s)) {
			t.Fatalf("QR token %q unexpectedly contains PII substring %q", resp.Token, s)
		}
	}
	if resp.RotateAfterSeconds != 60 || resp.AcceptWindowSeconds != 90 {
		t.Fatalf("expected default 60/90s windows, got %d/%d", resp.RotateAfterSeconds, resp.AcceptWindowSeconds)
	}
}

func TestQRHandlerErrorMappings(t *testing.T) {
	patientID := uuid.New()
	itemID := uuid.New()
	repo := &stubQRRepo{
		eligibility: &ItemEligibility{
			PrescriptionItemID: itemID,
			PrescriptionID:     uuid.New(),
			PatientID:          patientID,
			PrescriptionStatus: "cancelled",
			ValidUntil:         time.Now().UTC().Add(24 * time.Hour),
			ItemStatus:         "pending",
			QuantityPrescribed: 10,
			QuantityDispensed:  0,
		},
	}
	h := NewHandler(NewService(repo, 60*time.Second, 90*time.Second), nil)

	// Unauthenticated -> 401
	unauthRouter := httpserver.NewRouter(httpserver.RouterDeps{RegisterRoutes: h.RegisterRoutes})
	unauthRec := httptest.NewRecorder()
	unauthRouter.ServeHTTP(unauthRec, httptest.NewRequest(http.MethodPost, "/api/v1/prescription-items/"+itemID.String()+"/qr", nil))
	if unauthRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", unauthRec.Code)
	}

	var currentActor auth.ActorContext
	router := httpserver.NewRouter(httpserver.RouterDeps{
		RegisterRoutes: func(r chi.Router) {
			r.Use(func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					next.ServeHTTP(w, r.WithContext(auth.WithActor(r.Context(), currentActor)))
				})
			})
			h.RegisterRoutes(r)
		},
	})

	// Invalid UUID -> 400
	currentActor = auth.ActorContext{UserID: patientID, Role: auth.RolePatient}
	badRec := httptest.NewRecorder()
	router.ServeHTTP(badRec, httptest.NewRequest(http.MethodPost, "/api/v1/prescription-items/bad-id/qr", nil))
	if badRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", badRec.Code)
	}

	// Missing item -> 404
	missingRec := httptest.NewRecorder()
	router.ServeHTTP(missingRec, httptest.NewRequest(http.MethodPost, "/api/v1/prescription-items/"+uuid.New().String()+"/qr", nil))
	if missingRec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", missingRec.Code)
	}

	// Wrong owner -> 403
	currentActor = auth.ActorContext{UserID: uuid.New(), Role: auth.RolePatient}
	forbRec := httptest.NewRecorder()
	router.ServeHTTP(forbRec, httptest.NewRequest(http.MethodPost, "/api/v1/prescription-items/"+itemID.String()+"/qr", nil))
	if forbRec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", forbRec.Code)
	}

	// Cancelled prescription -> 409
	currentActor = auth.ActorContext{UserID: patientID, Role: auth.RolePatient}
	confRec := httptest.NewRecorder()
	router.ServeHTTP(confRec, httptest.NewRequest(http.MethodPost, "/api/v1/prescription-items/"+itemID.String()+"/qr", nil))
	if confRec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", confRec.Code)
	}
}
