package prescriptions

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/xjam-byte/erx-platform/backend/internal/audit"
	"github.com/xjam-byte/erx-platform/backend/internal/auth"
	"github.com/xjam-byte/erx-platform/backend/internal/pki"
)

// CanonicalPayload is returned to the doctor's browser for NCALayer EDS signing.
type CanonicalPayload struct {
	PrescriptionID     uuid.UUID       `json:"prescription_id"`
	PrescriptionNumber string          `json:"prescription_number"`
	Status             string          `json:"status"`
	CanonicalJSON      json.RawMessage `json:"canonical_json"`
	PayloadHash        string          `json:"payload_hash"`
}

// SignResult summarizes the completed prescription EDS verification and activation.
type SignResult struct {
	PrescriptionID   uuid.UUID `json:"prescription_id"`
	Status           string    `json:"status"`
	SignatureID      uuid.UUID `json:"signature_id"`
	SignerIIN        string    `json:"signer_iin"`
	CertSerialNumber string    `json:"cert_serial_number"`
	OCSPStatus       string    `json:"ocsp_status"`
	PayloadHash      string    `json:"payload_hash"`
	SignedAt         time.Time `json:"signed_at"`
}

// Service coordinates canonicalization and EDS signing for prescriptions.
type Service struct {
	repo        Repository
	txRunner    TxRunner
	sigRepo     pki.SignatureRepository
	verifier    pki.Verifier
	auditLogger audit.Logger
	logger      *slog.Logger
}

// NewService creates a prescription signing service.
func NewService(
	repo Repository,
	txRunner TxRunner,
	sigRepo pki.SignatureRepository,
	verifier pki.Verifier,
	auditLogger audit.Logger,
	logger *slog.Logger,
) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{
		repo:        repo,
		txRunner:    txRunner,
		sigRepo:     sigRepo,
		verifier:    verifier,
		auditLogger: auditLogger,
		logger:      logger,
	}
}

// GetCanonical builds the deterministic JSON payload and SHA-256 hash for a prescription.
func (s *Service) GetCanonical(ctx context.Context, prescriptionID uuid.UUID, actor auth.ActorContext) (*CanonicalPayload, error) {
	details, err := s.repo.GetDetails(ctx, prescriptionID)
	if err != nil {
		return nil, err
	}
	if actor.Role != auth.RoleAdmin && details.DoctorID != actor.UserID {
		return nil, ErrForbiddenActor
	}

	raw, hashHex, err := SerializeCanonical(details.Input)
	if err != nil {
		return nil, err
	}

	return &CanonicalPayload{
		PrescriptionID:     details.Input.PrescriptionID,
		PrescriptionNumber: details.Input.PrescriptionNumber,
		Status:             details.Status,
		CanonicalJSON:      json.RawMessage(raw),
		PayloadHash:        hashHex,
	}, nil
}

// Sign verifies the doctor's CMS EDS signature, persists it, activates the prescription,
// and appends an audit log entry inside one PostgreSQL transaction.
func (s *Service) Sign(ctx context.Context, prescriptionID uuid.UUID, actor auth.ActorContext, cmsBase64 string) (*SignResult, error) {
	if strings.TrimSpace(cmsBase64) == "" {
		return nil, ErrInvalidSigningRequest
	}

	var res *SignResult
	err := s.txRunner.WithTx(ctx, func(tx pgx.Tx) error {
		details, err := s.repo.LockDetailsForSignTx(ctx, tx, prescriptionID)
		if err != nil {
			return err
		}
		if details.DoctorID != actor.UserID {
			return ErrForbiddenActor
		}
		if details.Status != "draft" {
			return ErrInvalidStatusTransition
		}

		canonicalBytes, hashHex, err := SerializeCanonical(details.Input)
		if err != nil {
			return err
		}

		verified, err := s.verifier.VerifyCMS(ctx, cmsBase64, canonicalBytes, actor.IIN)
		if err != nil {
			return err
		}

		signedAt := time.Now().UTC()
		sigID, err := s.sigRepo.SaveTx(ctx, tx, pki.SignatureRecord{
			EntityType:         pki.EntityPrescription,
			EntityID:           prescriptionID,
			SignerUserID:       actor.UserID,
			SignerIIN:          verified.SignerIIN,
			CertSerialNumber:   verified.CertSerialNumber,
			RawSignature:       verified.RawSignature,
			SignatureHash:      hashHex,
			SignedAt:           signedAt,
			OCSPStatus:         verified.OCSPStatus,
			VerificationStatus: "verified",
		})
		if err != nil {
			return err
		}

		if err := s.repo.ActivateTx(ctx, tx, prescriptionID); err != nil {
			return err
		}

		if s.auditLogger != nil {
			actorID := actor.UserID
			entityID := prescriptionID
			if err := s.auditLogger.AppendTx(ctx, tx, audit.Entry{
				ActorID:    &actorID,
				Action:     "prescription:sign",
				EntityType: "prescription",
				EntityID:   &entityID,
				Payload: map[string]any{
					"signature_id":       sigID,
					"signer_iin":         verified.SignerIIN,
					"cert_serial_number": verified.CertSerialNumber,
					"payload_hash":       hashHex,
				},
			}); err != nil {
				return err
			}
		}

		res = &SignResult{
			PrescriptionID:   prescriptionID,
			Status:           "active",
			SignatureID:      sigID,
			SignerIIN:        verified.SignerIIN,
			CertSerialNumber: verified.CertSerialNumber,
			OCSPStatus:       verified.OCSPStatus,
			PayloadHash:      hashHex,
			SignedAt:         signedAt,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	s.logger.Info("prescription_signed",
		"prescription_id", res.PrescriptionID,
		"signature_id", res.SignatureID,
		"signer_iin", res.SignerIIN,
	)
	return res, nil
}
