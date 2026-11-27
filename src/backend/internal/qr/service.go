package qr

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"time"

	"github.com/google/uuid"
)

const (
	tokenPrefix   = "erx_"
	tokenByteLen  = 32
	nonceByteLen  = 16
	defaultRotate = 60 * time.Second
	defaultAccept = 90 * time.Second
)

// Service issues rotating, opaque, single-use QR tokens per prescription item.
type Service struct {
	repo         Repository
	rotation     time.Duration
	acceptWindow time.Duration
	randReader   io.Reader
	nowFn        func() time.Time
}

// NewService constructs a QR token service.
func NewService(repo Repository, rotation, acceptWindow time.Duration) *Service {
	if rotation <= 0 {
		rotation = defaultRotate
	}
	if acceptWindow <= 0 {
		acceptWindow = defaultAccept
	}
	return &Service{
		repo:         repo,
		rotation:     rotation,
		acceptWindow: acceptWindow,
		randReader:   rand.Reader,
		nowFn:        func() time.Time { return time.Now().UTC() },
	}
}

// IssueToken validates prescription item ownership and eligibility, then creates an opaque
// single-use token whose SHA-256 hash is stored in PostgreSQL.
func (s *Service) IssueToken(ctx context.Context, prescriptionItemID, patientUserID uuid.UUID) (*IssuedTokenResponse, error) {
	if prescriptionItemID == uuid.Nil || patientUserID == uuid.Nil {
		return nil, ErrItemNotEligibleForQR
	}

	eligibility, err := s.repo.GetItemEligibility(ctx, prescriptionItemID)
	if err != nil {
		return nil, err
	}

	if eligibility.PatientID != patientUserID {
		return nil, ErrItemNotOwnedByPatient
	}

	now := s.nowFn()
	if !isPrescriptionEligible(eligibility.PrescriptionStatus, eligibility.ValidUntil, now) {
		return nil, ErrItemNotEligibleForQR
	}
	if !isItemEligible(eligibility.ItemStatus, eligibility.QuantityDispensed, eligibility.QuantityPrescribed) {
		return nil, ErrItemNotEligibleForQR
	}

	rawToken, nonce, err := s.generateOpaqueToken()
	if err != nil {
		return nil, err
	}

	tokenHash := HashToken(rawToken)
	expiresAt := now.Add(s.acceptWindow)

	if _, err := s.repo.InsertToken(ctx, prescriptionItemID, tokenHash, nonce, expiresAt); err != nil {
		return nil, err
	}

	return &IssuedTokenResponse{
		Token:               rawToken,
		ExpiresAt:           expiresAt,
		RotateAfterSeconds:  int(s.rotation.Seconds()),
		AcceptWindowSeconds: int(s.acceptWindow.Seconds()),
	}, nil
}

func (s *Service) generateOpaqueToken() (rawToken string, nonce string, err error) {
	tokenBuf := make([]byte, tokenByteLen)
	if _, err := io.ReadFull(s.randReader, tokenBuf); err != nil {
		return "", "", fmt.Errorf("generate random QR token: %w", err)
	}
	nonceBuf := make([]byte, nonceByteLen)
	if _, err := io.ReadFull(s.randReader, nonceBuf); err != nil {
		return "", "", fmt.Errorf("generate random QR nonce: %w", err)
	}
	rawToken = tokenPrefix + base64.RawURLEncoding.EncodeToString(tokenBuf)
	nonce = hex.EncodeToString(nonceBuf)
	return rawToken, nonce, nil
}

func isPrescriptionEligible(status string, validUntil, now time.Time) bool {
	if status != "active" && status != "partially_dispensed" {
		return false
	}
	return validUntil.After(now)
}

func isItemEligible(status string, dispensed, prescribed int) bool {
	if status != "pending" && status != "partially_dispensed" {
		return false
	}
	return dispensed < prescribed
}
