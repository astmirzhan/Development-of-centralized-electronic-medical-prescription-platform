package qr

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository manages persistence and pessimistic locking for qr_tokens.
type Repository interface {
	GetItemEligibility(ctx context.Context, prescriptionItemID uuid.UUID) (*ItemEligibility, error)
	InsertToken(ctx context.Context, itemID uuid.UUID, tokenHash, nonce string, expiresAt time.Time) (*QRToken, error)
	FindAndLockForRedemption(ctx context.Context, tx pgx.Tx, rawToken string) (*QRToken, error)
	MarkConsumedTx(ctx context.Context, tx pgx.Tx, tokenID, consumedByUserID uuid.UUID) error
}

// PGRepository implements Repository using PostgreSQL 16.
type PGRepository struct {
	pool  *pgxpool.Pool
	nowFn func() time.Time
}

// NewPGRepository creates a PostgreSQL QR token repository.
func NewPGRepository(pool *pgxpool.Pool) *PGRepository {
	return &PGRepository{
		pool:  pool,
		nowFn: func() time.Time { return time.Now().UTC() },
	}
}

// GetItemEligibility loads the prescription item and parent prescription status/ownership.
func (r *PGRepository) GetItemEligibility(ctx context.Context, prescriptionItemID uuid.UUID) (*ItemEligibility, error) {
	var e ItemEligibility
	err := r.pool.QueryRow(ctx, `
		SELECT
			pi.id,
			p.id,
			p.patient_id,
			p.status::text,
			p.valid_until,
			pi.status::text,
			pi.quantity_prescribed,
			pi.quantity_dispensed
		FROM prescription_items pi
		JOIN prescriptions p ON p.id = pi.prescription_id
		WHERE pi.id = $1
	`, prescriptionItemID).Scan(
		&e.PrescriptionItemID,
		&e.PrescriptionID,
		&e.PatientID,
		&e.PrescriptionStatus,
		&e.ValidUntil,
		&e.ItemStatus,
		&e.QuantityPrescribed,
		&e.QuantityDispensed,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrPrescriptionItemNotFound
		}
		return nil, fmt.Errorf("get prescription item eligibility: %w", err)
	}
	return &e, nil
}

// InsertToken stores a hashed single-use QR token in qr_tokens (never stores raw token).
func (r *PGRepository) InsertToken(ctx context.Context, itemID uuid.UUID, tokenHash, nonce string, expiresAt time.Time) (*QRToken, error) {
	var tok QRToken
	err := r.pool.QueryRow(ctx, `
		INSERT INTO qr_tokens (
			prescription_item_id, token_hash, nonce, expires_at, is_consumed
		) VALUES ($1, $2, $3, $4, false)
		RETURNING id, prescription_item_id, token_hash, nonce, expires_at, is_consumed, created_at
	`, itemID, tokenHash, nonce, expiresAt.UTC()).Scan(
		&tok.ID,
		&tok.PrescriptionItemID,
		&tok.TokenHash,
		&tok.Nonce,
		&tok.ExpiresAt,
		&tok.IsConsumed,
		&tok.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert qr token: %w", err)
	}
	return &tok, nil
}

// FindAndLockForRedemption locks the matching qr_tokens row FOR UPDATE and validates
// consumption and expiration inside an active transaction (used by DIP-18 dispensing).
func (r *PGRepository) FindAndLockForRedemption(ctx context.Context, tx pgx.Tx, rawToken string) (*QRToken, error) {
	tokenHash := HashToken(rawToken)

	var tok QRToken
	err := tx.QueryRow(ctx, `
		SELECT id, prescription_item_id, token_hash, nonce, expires_at, is_consumed, consumed_by_user_id, consumed_at
		FROM qr_tokens
		WHERE token_hash = $1
		FOR UPDATE
	`, tokenHash).Scan(
		&tok.ID,
		&tok.PrescriptionItemID,
		&tok.TokenHash,
		&tok.Nonce,
		&tok.ExpiresAt,
		&tok.IsConsumed,
		&tok.ConsumedByUserID,
		&tok.ConsumedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrTokenNotFound
		}
		return nil, fmt.Errorf("lock qr token for redemption: %w", err)
	}

	if tok.IsConsumed {
		return nil, ErrTokenAlreadyConsumed
	}
	if !tok.ExpiresAt.After(r.nowFn()) {
		return nil, ErrTokenExpired
	}

	return &tok, nil
}

// MarkConsumedTx marks a locked QR token as consumed by the dispensing pharmacist.
func (r *PGRepository) MarkConsumedTx(ctx context.Context, tx pgx.Tx, tokenID, consumedByUserID uuid.UUID) error {
	tag, err := tx.Exec(ctx, `
		UPDATE qr_tokens
		SET is_consumed = true,
			consumed_by_user_id = $2,
			consumed_at = NOW()
		WHERE id = $1 AND is_consumed = false
	`, tokenID, consumedByUserID)
	if err != nil {
		return fmt.Errorf("mark qr token consumed: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return ErrTokenAlreadyConsumed
	}
	return nil
}
