package prescriptions_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/xjam-byte/erx-platform/backend/internal/audit"
	"github.com/xjam-byte/erx-platform/backend/internal/auth"
	"github.com/xjam-byte/erx-platform/backend/internal/pki"
	"github.com/xjam-byte/erx-platform/backend/internal/prescriptions"
	"github.com/xjam-byte/erx-platform/backend/internal/seed"
	"github.com/xjam-byte/erx-platform/backend/internal/testutil"
)

func TestPrescriptionCanonicalAndSignPG16(t *testing.T) {
	pool := testutil.SetupSeededDB(t)
	ctx := context.Background()

	var diagID int
	if err := pool.QueryRow(ctx, `SELECT id FROM icd10_codes WHERE code = 'J18.9'`).Scan(&diagID); err != nil {
		t.Fatalf("lookup J18.9: %v", err)
	}
	paracetamolForm := uuid.MustParse("20000000-0000-0000-0000-000000000015")

	rxID := uuid.New()
	if _, err := pool.Exec(ctx, `
		INSERT INTO prescriptions (id, prescription_number, doctor_id, clinic_id, patient_id, diagnosis_id, status, valid_until)
		VALUES ($1, 'RX-SIGN-PG-001', $2, $3, $4, $5, 'draft', $6)
	`, rxID, seed.DoctorUserID, seed.ClinicOrgID, seed.PatientUserID, diagID, time.Now().UTC().Add(30*24*time.Hour)); err != nil {
		t.Fatalf("insert draft prescription: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO prescription_items (id, prescription_id, medicine_form_id, dosage_value, dosage_unit, frequency, duration_days, quantity_prescribed)
		VALUES ($1, $2, $3, 500.00, 'mg', '2x/day', 5, 10)
	`, uuid.New(), rxID, paracetamolForm); err != nil {
		t.Fatalf("insert draft item: %v", err)
	}

	rxRepo := prescriptions.NewPGRepository(pool)
	sigRepo := pki.NewPGRepository()
	verifier, err := pki.NewMockVerifier("development", true, "pg-test-secret")
	if err != nil {
		t.Fatalf("NewMockVerifier: %v", err)
	}
	svc := prescriptions.NewService(rxRepo, rxRepo, sigRepo, verifier, audit.NewPGLogger(), nil)

	doctor := auth.ActorContext{
		UserID: seed.DoctorUserID,
		IIN:    "840315301294",
		Role:   auth.RoleDoctor,
	}

	canonicalResp, err := svc.GetCanonical(ctx, rxID, doctor)
	if err != nil {
		t.Fatalf("GetCanonical: %v", err)
	}

	cms, err := pki.CreateTestEnvelope("pg-test-secret", doctor.IIN, "CERT-PG-01", canonicalResp.CanonicalJSON)
	if err != nil {
		t.Fatalf("CreateTestEnvelope: %v", err)
	}

	signRes, err := svc.Sign(ctx, rxID, doctor, cms)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	if signRes.Status != "active" || signRes.SignerIIN != doctor.IIN {
		t.Fatalf("unexpected SignResult: %#v", signRes)
	}

	var dbStatus string
	if err := pool.QueryRow(ctx, `SELECT status::text FROM prescriptions WHERE id = $1`, rxID).Scan(&dbStatus); err != nil {
		t.Fatalf("query prescription status: %v", err)
	}
	if dbStatus != "active" {
		t.Fatalf("expected active status in DB, got %s", dbStatus)
	}

	var sigCount, auditCount int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM signatures WHERE entity_id = $1`, rxID).Scan(&sigCount)
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM audit_logs WHERE entity_id = $1 AND action = 'prescription:sign'`, rxID).Scan(&auditCount)
	if sigCount != 1 || auditCount != 1 {
		t.Fatalf("expected 1 signature and 1 audit log row, got sig=%d audit=%d", sigCount, auditCount)
	}
}
