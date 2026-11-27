package cds_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/xjam-byte/erx-platform/backend/internal/cds"
	"github.com/xjam-byte/erx-platform/backend/internal/seed"
	"github.com/xjam-byte/erx-platform/backend/internal/testutil"
)

func TestCDSIntegrationAndPerformancePG16(t *testing.T) {
	pool := testutil.SetupSeededDB(t)
	ctx := context.Background()

	var diagViralID, diagPneumoniaID int
	if err := pool.QueryRow(ctx, `SELECT id FROM icd10_codes WHERE code = 'J06.9'`).Scan(&diagViralID); err != nil {
		t.Fatalf("lookup J06.9: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT id FROM icd10_codes WHERE code = 'J18.9'`).Scan(&diagPneumoniaID); err != nil {
		t.Fatalf("lookup J18.9: %v", err)
	}

	amoxicillinForm := uuid.MustParse("20000000-0000-0000-0000-000000000001")
	azithromycinForm := uuid.MustParse("20000000-0000-0000-0000-000000000002")
	ciprofloxacinForm := uuid.MustParse("20000000-0000-0000-0000-000000000003")
	warfarinForm := uuid.MustParse("20000000-0000-0000-0000-000000000004")
	aspirinForm := uuid.MustParse("20000000-0000-0000-0000-000000000005")
	paracetamolForm := uuid.MustParse("20000000-0000-0000-0000-000000000015")

	// Seed one active Warfarin prescription for adult patient to exercise cross-prescription DDI.
	rxID := uuid.New()
	if _, err := pool.Exec(ctx, `
		INSERT INTO prescriptions (id, prescription_number, doctor_id, clinic_id, patient_id, diagnosis_id, status, valid_until)
		VALUES ($1, 'RX-ACTIVE-WARF', $2, $3, $4, $5, 'active', $6)
	`, rxID, seed.DoctorUserID, seed.ClinicOrgID, seed.PatientUserID, diagPneumoniaID, time.Now().UTC().Add(30*24*time.Hour)); err != nil {
		t.Fatalf("insert active warfarin prescription: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO prescription_items (id, prescription_id, medicine_form_id, dosage_value, dosage_unit, frequency, duration_days, quantity_prescribed)
		VALUES ($1, $2, $3, 2.50, 'mg', '1x/day', 30, 30)
	`, uuid.New(), rxID, warfarinForm); err != nil {
		t.Fatalf("insert active warfarin item: %v", err)
	}

	svc := cds.NewService(cds.NewPGRepository(pool), nil)

	// Warm pgxpool connections (matching production NewPool MinConns warm state)
	// before asserting <150ms CDS evaluation latency.
	if _, err := svc.Evaluate(ctx, cds.CheckRequest{
		PatientID:       seed.ChildUserID,
		DiagnosisID:     diagViralID,
		MedicineFormIDs: []uuid.UUID{paracetamolForm},
	}); err != nil {
		t.Fatalf("warmup Evaluate: %v", err)
	}

	// 1. Seeded Amoxicillin allergy for Sultanov Y.M. (981105303419) blocks issuance.
	allergyRes, err := svc.Evaluate(ctx, cds.CheckRequest{
		PatientID:       seed.PatientUserID,
		DiagnosisID:     diagPneumoniaID,
		MedicineFormIDs: []uuid.UUID{amoxicillinForm},
	})
	if err != nil {
		t.Fatalf("Evaluate allergy check: %v", err)
	}
	if allergyRes.Allowed || len(allergyRes.Warnings) == 0 {
		t.Fatalf("expected allergy block, got %#v", allergyRes)
	}
	if allergyRes.DurationMs >= 150 {
		t.Fatalf("CDS latency exceeded 150ms: %dms", allergyRes.DurationMs)
	}

	// 2. Cross-prescription critical DDI (active Warfarin + new Aspirin) blocks issuance.
	ddiRes, err := svc.Evaluate(ctx, cds.CheckRequest{
		PatientID:       seed.PatientUserID,
		DiagnosisID:     diagPneumoniaID,
		MedicineFormIDs: []uuid.UUID{aspirinForm},
	})
	if err != nil {
		t.Fatalf("Evaluate DDI check: %v", err)
	}
	if ddiRes.Allowed {
		t.Fatalf("expected critical DDI block with active Warfarin, got %#v", ddiRes)
	}

	// 3. Pediatric patient + Ciprofloxacin blocks issuance (< 18).
	childRes, err := svc.Evaluate(ctx, cds.CheckRequest{
		PatientID:       seed.ChildUserID,
		DiagnosisID:     diagPneumoniaID,
		MedicineFormIDs: []uuid.UUID{ciprofloxacinForm},
	})
	if err != nil {
		t.Fatalf("Evaluate pediatric check: %v", err)
	}
	if childRes.Allowed {
		t.Fatalf("expected pediatric fluoroquinolone block, got %#v", childRes)
	}

	// 4. Antibiotic (Azithromycin) for J06.9 viral URI requires justification, passes with justification, and finishes < 150ms.
	stewardRes, err := svc.Evaluate(ctx, cds.CheckRequest{
		PatientID:       seed.ChildUserID,
		DiagnosisID:     diagViralID,
		MedicineFormIDs: []uuid.UUID{azithromycinForm},
	})
	if err != nil {
		t.Fatalf("Evaluate stewardship check: %v", err)
	}
	if stewardRes.Allowed || !stewardRes.RequiresJustification {
		t.Fatalf("expected RequiresJustification=true, got %#v", stewardRes)
	}
	if stewardRes.DurationMs >= 150 {
		t.Fatalf("CDS latency exceeded 150ms: %dms", stewardRes.DurationMs)
	}

	// 5. Safe Paracetamol passes cleanly in < 150ms.
	safeRes, err := svc.Evaluate(ctx, cds.CheckRequest{
		PatientID:       seed.ChildUserID,
		DiagnosisID:     diagViralID,
		MedicineFormIDs: []uuid.UUID{paracetamolForm},
	})
	if err != nil {
		t.Fatalf("Evaluate safe check: %v", err)
	}
	if !safeRes.Allowed || len(safeRes.Warnings) != 0 {
		t.Fatalf("expected clean pass, got %#v", safeRes)
	}
	if safeRes.DurationMs >= 150 {
		t.Fatalf("CDS latency exceeded 150ms: %dms", safeRes.DurationMs)
	}
}
