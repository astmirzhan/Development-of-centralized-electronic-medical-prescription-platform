package prescriptions

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/xjam-byte/erx-platform/backend/internal/auth"
	"github.com/xjam-byte/erx-platform/backend/internal/httpserver"
	"github.com/xjam-byte/erx-platform/backend/internal/pki"
)

func TestCanonicalTamperDetectionAllFields(t *testing.T) {
	base := PrescriptionInput{
		PrescriptionID:     uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"),
		PrescriptionNumber: "RX-2026-00099",
		DoctorIIN:          "840315301294",
		ClinicBIN:          "240140000101",
		PatientIIN:         "981105303419",
		DiagnosisCode:      "J18.9",
		ValidUntil:         time.Date(2026, 12, 27, 10, 0, 0, 0, time.UTC),
		CreatedAt:          time.Date(2026, 11, 27, 10, 0, 0, 0, time.UTC),
		Items: []PrescriptionItemInput{
			{
				MedicineFormID:     uuid.MustParse("11111111-1111-1111-1111-111111111111"),
				DosageValue:        500,
				DosageUnit:         "mg",
				Frequency:          "2x/day",
				DurationDays:       7,
				QuantityPrescribed: 14,
			},
		},
	}

	_, origHash, err := SerializeCanonical(base)
	if err != nil {
		t.Fatalf("SerializeCanonical error: %v", err)
	}

	mutations := []struct {
		name   string
		mutate func(in *PrescriptionInput)
	}{
		{"prescription_number", func(in *PrescriptionInput) { in.PrescriptionNumber = "RX-2026-00098" }},
		{"doctor_iin", func(in *PrescriptionInput) { in.DoctorIIN = "840315301295" }},
		{"clinic_bin", func(in *PrescriptionInput) { in.ClinicBIN = "240140000102" }},
		{"patient_iin", func(in *PrescriptionInput) { in.PatientIIN = "981105303418" }},
		{"diagnosis_code", func(in *PrescriptionInput) { in.DiagnosisCode = "J06.9" }},
		{"valid_until", func(in *PrescriptionInput) { in.ValidUntil = in.ValidUntil.Add(time.Second) }},
		{"created_at", func(in *PrescriptionInput) { in.CreatedAt = in.CreatedAt.Add(-time.Second) }},
		{"dosage_value", func(in *PrescriptionInput) { in.Items[0].DosageValue = 250 }},
		{"dosage_unit", func(in *PrescriptionInput) { in.Items[0].DosageUnit = "ml" }},
		{"frequency", func(in *PrescriptionInput) { in.Items[0].Frequency = "3x/day" }},
		{"duration_days", func(in *PrescriptionInput) { in.Items[0].DurationDays = 10 }},
		{"quantity_prescribed", func(in *PrescriptionInput) { in.Items[0].QuantityPrescribed = 15 }},
	}

	for _, tc := range mutations {
		t.Run(tc.name, func(t *testing.T) {
			cloned := base
			cloned.Items = append([]PrescriptionItemInput(nil), base.Items...)
			tc.mutate(&cloned)

			ok, err := VerifyCanonicalHash(cloned, origHash)
			if err != nil {
				t.Fatalf("VerifyCanonicalHash error: %v", err)
			}
			if ok {
				t.Fatalf("mutating %s must fail hash verification", tc.name)
			}
		})
	}
}

func TestPrescriptionHandlerErrorPaths(t *testing.T) {
	doctorID := uuid.New()
	otherDoctorID := uuid.New()
	rxID := uuid.New()

	repo := &stubRxRepo{
		details: &PrescriptionDetails{
			DoctorID: doctorID,
			Status:   "draft",
			Input: PrescriptionInput{
				PrescriptionID:     rxID,
				PrescriptionNumber: "RX-2026-00055",
				DoctorIIN:          "840315301294",
				ClinicBIN:          "240140000101",
				PatientIIN:         "981105303419",
				DiagnosisCode:      "J18.9",
				ValidUntil:         time.Now().UTC().Add(24 * time.Hour),
				CreatedAt:          time.Now().UTC(),
				Items: []PrescriptionItemInput{
					{MedicineFormID: uuid.New(), DosageValue: 500, DosageUnit: "mg", Frequency: "1x/day", DurationDays: 5, QuantityPrescribed: 5},
				},
			},
		},
	}

	verifier, _ := pki.NewMockVerifier("development", true, "secret")
	svc := NewService(repo, repo, &stubSigRepo{}, verifier, &stubAuditLogger{}, nil)
	h := NewHandler(svc, nil)

	t.Run("unauthenticated returns 401", func(t *testing.T) {
		router := httpserver.NewRouter(httpserver.RouterDeps{RegisterRoutes: h.RegisterRoutes})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/prescriptions/"+rxID.String()+"/canonical", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("forbidden doctor returns 403 and invalid signature returns 422", func(t *testing.T) {
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

		// Forbidden actor -> 403
		currentActor = auth.ActorContext{UserID: otherDoctorID, IIN: "840315301299", Role: auth.RoleDoctor}
		req := httptest.NewRequest(http.MethodPost, "/api/v1/prescriptions/"+rxID.String()+"/canonical", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403, got %d", rec.Code)
		}

		// Invalid UUID -> 400
		currentActor = auth.ActorContext{UserID: doctorID, IIN: "840315301294", Role: auth.RoleDoctor}
		badIDReq := httptest.NewRequest(http.MethodPost, "/api/v1/prescriptions/not-a-uuid/sign", bytes.NewBufferString(`{"cms_signature":"abc"}`))
		badIDRec := httptest.NewRecorder()
		router.ServeHTTP(badIDRec, badIDReq)
		if badIDRec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", badIDRec.Code)
		}

		// Not found -> 404
		missingReq := httptest.NewRequest(http.MethodPost, "/api/v1/prescriptions/"+uuid.New().String()+"/canonical", nil)
		missingRec := httptest.NewRecorder()
		router.ServeHTTP(missingRec, missingReq)
		if missingRec.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", missingRec.Code)
		}

		// Invalid CMS signature -> 422
		body, _ := json.Marshal(signRequest{CMSSignature: "invalid-base64-cms"})
		badSigReq := httptest.NewRequest(http.MethodPost, "/api/v1/prescriptions/"+rxID.String()+"/sign", bytes.NewReader(body))
		badSigRec := httptest.NewRecorder()
		router.ServeHTTP(badSigRec, badSigReq)
		if badSigRec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("expected 422, got %d: %s", badSigRec.Code, badSigRec.Body.String())
		}
	})

	t.Run("unavailable verifier returns 503", func(t *testing.T) {
		unavailVerifier := unavailablePKI{}
		unavailSvc := NewService(repo, repo, &stubSigRepo{}, unavailVerifier, nil, nil)
		unavailH := NewHandler(unavailSvc, nil)
		actor := auth.ActorContext{UserID: doctorID, IIN: "840315301294", Role: auth.RoleDoctor}
		router := httpserver.NewRouter(httpserver.RouterDeps{
			RegisterRoutes: func(r chi.Router) {
				r.Use(func(next http.Handler) http.Handler {
					return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						next.ServeHTTP(w, r.WithContext(auth.WithActor(r.Context(), actor)))
					})
				})
				unavailH.RegisterRoutes(r)
			},
		})
		body, _ := json.Marshal(signRequest{CMSSignature: "ZmFrZQ=="})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/prescriptions/"+rxID.String()+"/sign", bytes.NewReader(body))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("expected 503, got %d", rec.Code)
		}
	})
}

type unavailablePKI struct{}

func (unavailablePKI) VerifyCMS(_ context.Context, _ string, _ []byte, _ string) (*pki.VerificationResult, error) {
	return nil, errors.Join(pki.ErrVerifierUnavailable, errors.New("connection refused"))
}
