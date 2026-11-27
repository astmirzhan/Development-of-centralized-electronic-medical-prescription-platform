package pki_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/xjam-byte/erx-platform/backend/internal/database"
	"github.com/xjam-byte/erx-platform/backend/internal/pki"
	"github.com/xjam-byte/erx-platform/backend/internal/seed"
	"github.com/xjam-byte/erx-platform/backend/internal/testutil"
)

func TestSignatureRepositorySaveTxPG16(t *testing.T) {
	pool := testutil.SetupSeededDB(t)
	ctx := context.Background()
	repo := pki.NewPGRepository()

	var sigID uuid.UUID
	entityID := uuid.New()
	err := database.WithTx(ctx, pool, func(tx pgx.Tx) error {
		var err error
		sigID, err = repo.SaveTx(ctx, tx, pki.SignatureRecord{
			EntityType:         pki.EntityPrescription,
			EntityID:           entityID,
			SignerUserID:       seed.DoctorUserID,
			SignerIIN:          "840315301294",
			CertSerialNumber:   "CERT-PKI-REPO-1",
			RawSignature:       "ZmFrZUNNUw==",
			SignatureHash:      pki.SHA256Hex([]byte("canonical-test")),
			SignedAt:           time.Now().UTC(),
			OCSPStatus:         "GOOD",
			VerificationStatus: "verified",
		})
		return err
	})
	if err != nil {
		t.Fatalf("SaveTx error: %v", err)
	}
	if sigID == uuid.Nil {
		t.Fatal("expected non-nil signature UUID")
	}

	var storedIIN string
	if err := pool.QueryRow(ctx, `SELECT signer_iin FROM signatures WHERE id = $1`, sigID).Scan(&storedIIN); err != nil {
		t.Fatalf("query saved signature: %v", err)
	}
	if storedIIN != "840315301294" {
		t.Fatalf("expected signer_iin 840315301294, got %s", storedIIN)
	}
}
