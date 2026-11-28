package pki

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Entity types matching PostgreSQL signature_entity_enum.
const (
	EntityPrescription = "prescription"
	EntityDispensing   = "dispensing"
	EntityCancellation = "cancellation"
)

// SignatureRecord maps to a row in the signatures table.
type SignatureRecord struct {
	EntityType         string
	EntityID           uuid.UUID
	SignerUserID       uuid.UUID
	SignerIIN          string
	CertSerialNumber   string
	RawSignature       string
	SignatureHash      string
	SignedAt           time.Time
	OCSPStatus         string
	VerificationStatus string
}

// SignatureRepository persists verified EDS signatures inside a DB transaction.
type SignatureRepository interface {
	SaveTx(ctx context.Context, tx pgx.Tx, rec SignatureRecord) (uuid.UUID, error)
}

// PGRepository implements SignatureRepository using PostgreSQL 16.
type PGRepository struct{}

// NewPGRepository creates a PostgreSQL signature repository.
func NewPGRepository() *PGRepository {
	return &PGRepository{}
}

// SaveTx inserts a verified signature record into signatures and returns its UUID.
func (r *PGRepository) SaveTx(ctx context.Context, tx pgx.Tx, rec SignatureRecord) (uuid.UUID, error) {
	var id uuid.UUID
	err := tx.QueryRow(ctx, `
		INSERT INTO signatures (
			entity_type, entity_id, signer_user_id, signer_iin, cert_serial_number,
			raw_signature, signature_hash, signed_at, ocsp_status, verification_status
		) VALUES (
			$1::signature_entity_enum, $2, $3, $4, $5, $6, $7, $8, $9, $10
		)
		RETURNING id
	`,
		rec.EntityType,
		rec.EntityID,
		rec.SignerUserID,
		rec.SignerIIN,
		rec.CertSerialNumber,
		rec.RawSignature,
		rec.SignatureHash,
		rec.SignedAt.UTC(),
		rec.OCSPStatus,
		rec.VerificationStatus,
	).Scan(&id)
	if err != nil {
		return uuid.Nil, fmt.Errorf("insert signature: %w", err)
	}
	return id, nil
}
