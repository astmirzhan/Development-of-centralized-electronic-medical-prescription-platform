package qr

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"github.com/google/uuid"
)

// Typed domain errors for QR token issuance and redemption locking.
var (
	ErrTokenNotFound            = errors.New("QR token not found")
	ErrTokenExpired             = errors.New("QR token has expired")
	ErrTokenAlreadyConsumed     = errors.New("QR token has already been consumed")
	ErrPrescriptionItemNotFound = errors.New("prescription item not found")
	ErrItemNotOwnedByPatient    = errors.New("prescription item does not belong to requesting patient")
	ErrItemNotEligibleForQR     = errors.New("prescription item is not eligible for QR issuance")
)

// QRToken represents a stored row in qr_tokens (never stores raw token).
type QRToken struct {
	ID                 uuid.UUID  `json:"id"`
	PrescriptionItemID uuid.UUID  `json:"prescription_item_id"`
	TokenHash          string     `json:"token_hash"`
	Nonce              string     `json:"nonce"`
	ExpiresAt          time.Time  `json:"expires_at"`
	IsConsumed         bool       `json:"is_consumed"`
	ConsumedByUserID   *uuid.UUID `json:"consumed_by_user_id,omitempty"`
	ConsumedAt         *time.Time `json:"consumed_at,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
}

// IssuedTokenResponse is returned to the patient client upon QR token generation.
type IssuedTokenResponse struct {
	Token               string    `json:"token"`
	ExpiresAt           time.Time `json:"expires_at"`
	RotateAfterSeconds  int       `json:"rotate_after_seconds"`
	AcceptWindowSeconds int       `json:"accept_window_seconds"`
}

// ItemEligibility carries parent prescription and item status needed before issuing a QR token.
type ItemEligibility struct {
	PrescriptionItemID uuid.UUID
	PrescriptionID     uuid.UUID
	PatientID          uuid.UUID
	PrescriptionStatus string
	ValidUntil         time.Time
	ItemStatus         string
	QuantityPrescribed int
	QuantityDispensed  int
}

// HashToken returns the SHA-256 hex digest of an opaque QR token.
func HashToken(rawToken string) string {
	sum := sha256.Sum256([]byte(rawToken))
	return hex.EncodeToString(sum[:])
}
