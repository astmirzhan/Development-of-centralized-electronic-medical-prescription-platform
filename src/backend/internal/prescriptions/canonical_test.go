package prescriptions

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/xjam-byte/erx-platform/backend/internal/audit"
	"github.com/xjam-byte/erx-platform/backend/internal/auth"
	"github.com/xjam-byte/erx-platform/backend/internal/httpserver"
	"github.com/xjam-byte/erx-platform/backend/internal/pki"
)

func TestCanonicalSerializationDeterminism(t *testing.T) {
	rxID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	formA := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	formB := uuid.MustParse("22222222-2222-2222-2222-222222222222")

	loc := time.FixedZone("ALMT", 5*3600)
	created := time.Date(2026, 11, 27, 15, 0, 0, 0, loc)
	validUntil := time.Date(2026, 12, 27, 15, 0, 0, 0, loc)

	in1 := PrescriptionInput{
		PrescriptionID:     rxID,
		PrescriptionNumber: "RX-2026-00001",
		DoctorIIN:          "840315301294",
		ClinicBIN:          "240140000101",
		PatientIIN:         "981105303419",
		DiagnosisCode:      "J18.9",
		ValidUntil:         validUntil,
		CreatedAt:          created,
		Items: []PrescriptionItemInput{
			{MedicineFormID: formB, DosageValue: 500.0, DosageUnit: "mg", Frequency: "2x/day", DurationDays: 7, QuantityPrescribed: 14},
			{MedicineFormID: formA, DosageValue: 2.5, DosageUnit: "mg", Frequency: "1x/day", DurationDays: 30, QuantityPrescribed: 30},
		},
	}

	in2 := in1
	in2.ValidUntil = validUntil.UTC()
	in2.CreatedAt = created.UTC()
	in2.Items = []PrescriptionItemInput{in1.Items[1], in1.Items[0]}

	raw1, hash1, err := SerializeCanonical(in1)
	if err != nil {
		t.Fatalf("SerializeCanonical(in1) error: %v", err)
	}
	raw2, hash2, err := SerializeCanonical(in2)
	if err != nil {
		t.Fatalf("SerializeCanonical(in2) error: %v", err)
	}
	if string(raw1) != string(raw2) || hash1 != hash2 {
		t.Fatalf("expected deterministic output regardless of input item order or timezone")
	}

	// Single-field tampering must alter the hash.
	tampered := in1
	tampered.Items = append([]PrescriptionItemInput(nil), in1.Items...)
	tampered.Items[0].DosageValue = 500.01
	ok, err := VerifyCanonicalHash(tampered, hash1)
	if err != nil {
		t.Fatalf("VerifyCanonicalHash error: %v", err)
	}
	if ok {
		t.Fatal("expected VerifyCanonicalHash to reject tampered dosage")
	}
}

type stubRxRepo struct {
	details   *PrescriptionDetails
	activated bool
}

func (s *stubRxRepo) GetDetails(_ context.Context, id uuid.UUID) (*PrescriptionDetails, error) {
	if s.details == nil || s.details.Input.PrescriptionID != id {
		return nil, ErrPrescriptionNotFound
	}
	return s.details, nil
}

func (s *stubRxRepo) LockDetailsForSignTx(_ context.Context, _ pgx.Tx, id uuid.UUID) (*PrescriptionDetails, error) {
	return s.GetDetails(context.Background(), id)
}

func (s *stubRxRepo) ActivateTx(_ context.Context, _ pgx.Tx, _ uuid.UUID) error {
	s.activated = true
	s.details.Status = "active"
	return nil
}

func (s *stubRxRepo) WithTx(_ context.Context, fn func(pgx.Tx) error) error {
	return fn(nil)
}

type stubSigRepo struct {
	saved []pki.SignatureRecord
}

func (s *stubSigRepo) SaveTx(_ context.Context, _ pgx.Tx, rec pki.SignatureRecord) (uuid.UUID, error) {
	s.saved = append(s.saved, rec)
	return uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"), nil
}

type stubAuditLogger struct {
	entries []audit.Entry
}

func (s *stubAuditLogger) AppendTx(_ context.Context, _ pgx.Tx, entry audit.Entry) error {
	s.entries = append(s.entries, entry)
	return nil
}

func TestSigningServiceAndHandler(t *testing.T) {
	doctorID := uuid.New()
	rxID := uuid.New()
	repo := &stubRxRepo{
		details: &PrescriptionDetails{
			DoctorID: doctorID,
			Status:   "draft",
			Input: PrescriptionInput{
				PrescriptionID:     rxID,
				PrescriptionNumber: "RX-2026-00042",
				DoctorIIN:          "840315301294",
				ClinicBIN:          "240140000101",
				PatientIIN:         "981105303419",
				DiagnosisCode:      "J18.9",
				ValidUntil:         time.Now().UTC().Add(30 * 24 * time.Hour),
				CreatedAt:          time.Now().UTC(),
				Items: []PrescriptionItemInput{
					{MedicineFormID: uuid.New(), DosageValue: 500, DosageUnit: "mg", Frequency: "2x/day", DurationDays: 7, QuantityPrescribed: 14},
				},
			},
		},
	}

	sigRepo := &stubSigRepo{}
	auditLog := &stubAuditLogger{}
	verifier, err := pki.NewMockVerifier("development", true, "test-secret")
	if err != nil {
		t.Fatalf("NewMockVerifier error: %v", err)
	}

	svc := NewService(repo, repo, sigRepo, verifier, auditLog, nil)
	h := NewHandler(svc, nil)
	actor := auth.ActorContext{UserID: doctorID, IIN: "840315301294", Role: auth.RoleDoctor}

	router := httpserver.NewRouter(httpserver.RouterDeps{
		RegisterRoutes: func(r chi.Router) {
			r.Use(func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					next.ServeHTTP(w, r.WithContext(auth.WithActor(r.Context(), actor)))
				})
			})
			h.RegisterRoutes(r)
		},
	})

	// 1. Canonical endpoint
	canReq := httptest.NewRequest(http.MethodPost, "/api/v1/prescriptions/"+rxID.String()+"/canonical", nil)
	canRec := httptest.NewRecorder()
	router.ServeHTTP(canRec, canReq)
	if canRec.Code != http.StatusOK {
		t.Fatalf("expected 200 on canonical, got %d: %s", canRec.Code, canRec.Body.String())
	}

	var canResp CanonicalPayload
	if err := json.Unmarshal(canRec.Body.Bytes(), &canResp); err != nil {
		t.Fatalf("unmarshal canonical response: %v", err)
	}

	// 2. Sign with valid test envelope
	cms, err := pki.CreateTestEnvelope("test-secret", actor.IIN, "CERT-12345", canResp.CanonicalJSON)
	if err != nil {
		t.Fatalf("CreateTestEnvelope error: %v", err)
	}

	body, _ := json.Marshal(signRequest{CMSSignature: cms})
	signReq := httptest.NewRequest(http.MethodPost, "/api/v1/prescriptions/"+rxID.String()+"/sign", bytes.NewReader(body))
	signRec := httptest.NewRecorder()
	router.ServeHTTP(signRec, signReq)
	if signRec.Code != http.StatusOK {
		t.Fatalf("expected 200 on sign, got %d: %s", signRec.Code, signRec.Body.String())
	}

	if !repo.activated || len(sigRepo.saved) != 1 || len(auditLog.entries) != 1 {
		t.Fatalf("expected prescription activated, 1 signature saved, 1 audit entry written")
	}

	// 3. Re-signing an active prescription must return 409 Conflict
	reSignReq := httptest.NewRequest(http.MethodPost, "/api/v1/prescriptions/"+rxID.String()+"/sign", bytes.NewReader(body))
	reSignRec := httptest.NewRecorder()
	router.ServeHTTP(reSignRec, reSignReq)
	if reSignRec.Code != http.StatusConflict {
		t.Fatalf("expected 409 Conflict when re-signing active prescription, got %d", reSignRec.Code)
	}
}
