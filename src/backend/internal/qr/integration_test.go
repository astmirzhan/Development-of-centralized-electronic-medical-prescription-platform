package qr_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/xjam-byte/erx-platform/backend/internal/database"
	"github.com/xjam-byte/erx-platform/backend/internal/qr"
	"github.com/xjam-byte/erx-platform/backend/internal/seed"
	"github.com/xjam-byte/erx-platform/backend/internal/testutil"
)

func TestQRIssueAndLockRedemptionPG16(t *testing.T) {
	pool := testutil.SetupSeededDB(t)
	ctx := context.Background()

	var diagID int
	if err := pool.QueryRow(ctx, `SELECT id FROM icd10_codes WHERE code = 'J18.9'`).Scan(&diagID); err != nil {
		t.Fatalf("lookup J18.9: %v", err)
	}
	paracetamolForm := uuid.MustParse("20000000-0000-0000-0000-000000000015")

	rxID := uuid.New()
	itemID := uuid.New()
	if _, err := pool.Exec(ctx, `
		INSERT INTO prescriptions (id, prescription_number, doctor_id, clinic_id, patient_id, diagnosis_id, status, valid_until)
		VALUES ($1, 'RX-QR-PG-001', $2, $3, $4, $5, 'active', $6)
	`, rxID, seed.DoctorUserID, seed.ClinicOrgID, seed.PatientUserID, diagID, time.Now().UTC().Add(30*24*time.Hour)); err != nil {
		t.Fatalf("insert active prescription: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO prescription_items (id, prescription_id, medicine_form_id, dosage_value, dosage_unit, frequency, duration_days, quantity_prescribed)
		VALUES ($1, $2, $3, 500.00, 'mg', '2x/day', 5, 10)
	`, itemID, rxID, paracetamolForm); err != nil {
		t.Fatalf("insert item: %v", err)
	}

	repo := qr.NewPGRepository(pool)
	svc := qr.NewService(repo, 60*time.Second, 90*time.Second)

	issued, err := svc.IssueToken(ctx, itemID, seed.PatientUserID)
	if err != nil {
		t.Fatalf("IssueToken: %v", err)
	}

	// Verify DB never stores raw token, only SHA-256 hash.
	var storedHash string
	if err := pool.QueryRow(ctx, `SELECT token_hash FROM qr_tokens WHERE prescription_item_id = $1`, itemID).Scan(&storedHash); err != nil {
		t.Fatalf("query stored token_hash: %v", err)
	}
	if storedHash == issued.Token || storedHash != qr.HashToken(issued.Token) {
		t.Fatalf("stored token_hash mismatch: got %q", storedHash)
	}

	// Lock and consume inside a transaction.
	err = database.WithTx(ctx, pool, func(tx pgx.Tx) error {
		locked, err := repo.FindAndLockForRedemption(ctx, tx, issued.Token)
		if err != nil {
			return err
		}
		return repo.MarkConsumedTx(ctx, tx, locked.ID, seed.PharmacistUserID)
	})
	if err != nil {
		t.Fatalf("consume QR token tx: %v", err)
	}

	// Second redemption attempt on consumed token must return ErrTokenAlreadyConsumed.
	err = database.WithTx(ctx, pool, func(tx pgx.Tx) error {
		_, err := repo.FindAndLockForRedemption(ctx, tx, issued.Token)
		return err
	})
	if !errors.Is(err, qr.ErrTokenAlreadyConsumed) {
		t.Fatalf("expected ErrTokenAlreadyConsumed, got %v", err)
	}

	// Expired token (>90s) must return ErrTokenExpired.
	expiredRaw := "erx_expired_test_token_value"
	if _, err := repo.InsertToken(ctx, itemID, qr.HashToken(expiredRaw), "deadbeef", time.Now().UTC().Add(-91*time.Second)); err != nil {
		t.Fatalf("insert expired token: %v", err)
	}
	err = database.WithTx(ctx, pool, func(tx pgx.Tx) error {
		_, err := repo.FindAndLockForRedemption(ctx, tx, expiredRaw)
		return err
	})
	if !errors.Is(err, qr.ErrTokenExpired) {
		t.Fatalf("expected ErrTokenExpired, got %v", err)
	}
}
