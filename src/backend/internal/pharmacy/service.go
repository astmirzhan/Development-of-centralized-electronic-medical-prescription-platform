package pharmacy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/xjam-byte/erx-platform/backend/internal/audit"
	"github.com/xjam-byte/erx-platform/backend/internal/auth"
	"github.com/xjam-byte/erx-platform/backend/internal/database"
	"github.com/xjam-byte/erx-platform/backend/internal/pki"
	"github.com/xjam-byte/erx-platform/backend/internal/qr"
)

// Domain errors for atomic dispensing.
var (
	ErrInvalidDispenseRequest = errors.New("invalid dispensing request")
	ErrForbiddenPharmacist    = errors.New("actor is not a verified pharmacist")
	ErrPrescriptionNotActive  = errors.New("prescription is not active or has expired")
	ErrItemAlreadyDispensed   = errors.New("prescription item is already fully dispensed or cancelled")
	ErrInsufficientQuantity   = errors.New("requested quantity exceeds remaining prescribed quantity")
	ErrBrandMismatch          = errors.New("medicine brand does not match prescribed medicine form")
)

// DispenseRequest represents a pharmacist's request to fulfill an item using a single-use QR token.
type DispenseRequest struct {
	QRToken         string    `json:"qr_token"`
	MedicineBrandID uuid.UUID `json:"medicine_brand_id"`
	Quantity        int       `json:"quantity"`
	Notes           string    `json:"notes,omitempty"`
	CMSSignature    string    `json:"cms_signature"`
}

// DispenseResult summarizes the committed atomic dispensing transaction.
type DispenseResult struct {
	DispensingEventID  uuid.UUID `json:"dispensing_event_id"`
	PrescriptionID     uuid.UUID `json:"prescription_id"`
	PrescriptionItemID uuid.UUID `json:"prescription_item_id"`
	QuantityDispensed  int       `json:"quantity_dispensed"`
	TotalDispensed     int       `json:"total_dispensed"`
	RemainingQuantity  int       `json:"remaining_quantity"`
	ItemStatus         string    `json:"item_status"`
	PrescriptionStatus string    `json:"prescription_status"`
	SignatureID        uuid.UUID `json:"signature_id"`
	DispensedAt        time.Time `json:"dispensed_at"`
}

// BuildDispensingCanonical produces the deterministic JSON payload signed by the pharmacist's EDS.
func BuildDispensingCanonical(qrToken, pharmacistIIN string, medicineBrandID uuid.UUID, quantity int) ([]byte, error) {
	payload := struct {
		QRTokenHash     string `json:"qr_token_hash"`
		PharmacistIIN   string `json:"pharmacist_iin"`
		MedicineBrandID string `json:"medicine_brand_id"`
		Quantity        int    `json:"quantity"`
	}{
		QRTokenHash:     qr.HashToken(strings.TrimSpace(qrToken)),
		PharmacistIIN:   strings.TrimSpace(pharmacistIIN),
		MedicineBrandID: medicineBrandID.String(),
		Quantity:        quantity,
	}
	return json.Marshal(payload)
}

// Service coordinates atomic QR token consumption, EDS verification, and quantity updates.
type Service struct {
	pool        *pgxpool.Pool
	qrRepo      qr.Repository
	sigRepo     pki.SignatureRepository
	verifier    pki.Verifier
	auditLogger audit.Logger
	logger      *slog.Logger
}

// NewService constructs an atomic pharmacy dispensing service.
func NewService(
	pool *pgxpool.Pool,
	qrRepo qr.Repository,
	sigRepo pki.SignatureRepository,
	verifier pki.Verifier,
	auditLogger audit.Logger,
	logger *slog.Logger,
) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{
		pool:        pool,
		qrRepo:      qrRepo,
		sigRepo:     sigRepo,
		verifier:    verifier,
		auditLogger: auditLogger,
		logger:      logger,
	}
}

// Dispense atomically validates the single-use QR token, locks the prescription item,
// verifies the pharmacist's EDS signature, records dispensing_events, and updates statuses.
func (s *Service) Dispense(ctx context.Context, actor auth.ActorContext, req DispenseRequest) (*DispenseResult, error) {
	if strings.TrimSpace(req.QRToken) == "" || req.MedicineBrandID == uuid.Nil || req.Quantity <= 0 {
		return nil, ErrInvalidDispenseRequest
	}
	if s.verifier != nil && strings.TrimSpace(req.CMSSignature) == "" {
		return nil, ErrInvalidDispenseRequest
	}
	if actor.Role != auth.RolePharmacist {
		return nil, ErrForbiddenPharmacist
	}

	var result *DispenseResult
	err := database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		var pharmacyID uuid.UUID
		var pharmacistVerified bool
		err := tx.QueryRow(ctx, `
			SELECT pharmacy_id, is_verified
			FROM pharmacist_profiles
			WHERE user_id = $1
		`, actor.UserID).Scan(&pharmacyID, &pharmacistVerified)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrForbiddenPharmacist
			}
			return fmt.Errorf("lookup pharmacist profile: %w", err)
		}
		if !pharmacistVerified {
			return ErrForbiddenPharmacist
		}

		// Layer 1: Single-use QR token row lock (FOR UPDATE).
		tok, err := s.qrRepo.FindAndLockForRedemption(ctx, tx, req.QRToken)
		if err != nil {
			return err
		}

		// Layer 2: Lock prescription_item and parent prescription (FOR UPDATE).
		var (
			prescriptionID     uuid.UUID
			medicineFormID     uuid.UUID
			qtyPrescribed      int
			qtyDispensed       int
			itemStatus         string
			prescriptionStatus string
			validUntil         time.Time
		)
		err = tx.QueryRow(ctx, `
			SELECT
				pi.prescription_id,
				pi.medicine_form_id,
				pi.quantity_prescribed,
				pi.quantity_dispensed,
				pi.status::text,
				p.status::text,
				p.valid_until
			FROM prescription_items pi
			JOIN prescriptions p ON p.id = pi.prescription_id
			WHERE pi.id = $1
			FOR UPDATE OF pi, p
		`, tok.PrescriptionItemID).Scan(
			&prescriptionID,
			&medicineFormID,
			&qtyPrescribed,
			&qtyDispensed,
			&itemStatus,
			&prescriptionStatus,
			&validUntil,
		)
		if err != nil {
			return fmt.Errorf("lock prescription item: %w", err)
		}

		now := time.Now().UTC()
		if (prescriptionStatus != "active" && prescriptionStatus != "partially_dispensed") || !validUntil.After(now) {
			return ErrPrescriptionNotActive
		}
		if itemStatus == "dispensed" || itemStatus == "cancelled" {
			return ErrItemAlreadyDispensed
		}
		remaining := qtyPrescribed - qtyDispensed
		if req.Quantity > remaining {
			return ErrInsufficientQuantity
		}

		// Verify chosen trade brand matches the prescribed medicine form.
		var brandFormID uuid.UUID
		err = tx.QueryRow(ctx, `
			SELECT medicine_form_id FROM medicine_brands WHERE id = $1
		`, req.MedicineBrandID).Scan(&brandFormID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrBrandMismatch
			}
			return fmt.Errorf("lookup medicine brand: %w", err)
		}
		if brandFormID != medicineFormID {
			return ErrBrandMismatch
		}

		// Verify pharmacist EDS signature and persist in signatures.
		var sigID uuid.UUID
		if s.verifier != nil {
			canonicalBytes, err := BuildDispensingCanonical(req.QRToken, actor.IIN, req.MedicineBrandID, req.Quantity)
			if err != nil {
				return err
			}
			verified, err := s.verifier.VerifyCMS(ctx, req.CMSSignature, canonicalBytes, actor.IIN)
			if err != nil {
				return err
			}
			eventID := uuid.New()
			sigID, err = s.sigRepo.SaveTx(ctx, tx, pki.SignatureRecord{
				EntityType:         pki.EntityDispensing,
				EntityID:           eventID,
				SignerUserID:       actor.UserID,
				SignerIIN:          verified.SignerIIN,
				CertSerialNumber:   verified.CertSerialNumber,
				RawSignature:       verified.RawSignature,
				SignatureHash:      verified.SignedDataHash,
				SignedAt:           now,
				OCSPStatus:         verified.OCSPStatus,
				VerificationStatus: "verified",
			})
			if err != nil {
				return err
			}
		}

		// Consume the single-use QR token.
		if err := s.qrRepo.MarkConsumedTx(ctx, tx, tok.ID, actor.UserID); err != nil {
			return err
		}

		// Layer 3: Update item quantity (protected by PostgreSQL CHECK constraint as well).
		newTotal := qtyDispensed + req.Quantity
		newItemStatus := "partially_dispensed"
		if newTotal == qtyPrescribed {
			newItemStatus = "dispensed"
		}
		if _, err := tx.Exec(ctx, `
			UPDATE prescription_items
			SET quantity_dispensed = $2,
				status = $3::prescription_item_status_enum
			WHERE id = $1
		`, tok.PrescriptionItemID, newTotal, newItemStatus); err != nil {
			return fmt.Errorf("update prescription item quantity: %w", err)
		}

		var sigPtr *uuid.UUID
		if sigID != uuid.Nil {
			sigPtr = &sigID
		}
		var eventID uuid.UUID
		if err := tx.QueryRow(ctx, `
			INSERT INTO dispensing_events (
				prescription_item_id, pharmacist_id, pharmacy_id, medicine_brand_id,
				quantity_dispensed, dispensed_at, notes, signature_id
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			RETURNING id
		`,
			tok.PrescriptionItemID,
			actor.UserID,
			pharmacyID,
			req.MedicineBrandID,
			req.Quantity,
			now,
			nullIfEmpty(req.Notes),
			sigPtr,
		).Scan(&eventID); err != nil {
			return fmt.Errorf("insert dispensing event: %w", err)
		}

		// Update parent prescription status based on remaining open items.
		var openItems int
		if err := tx.QueryRow(ctx, `
			SELECT count(*)
			FROM prescription_items
			WHERE prescription_id = $1
			  AND status NOT IN ('dispensed', 'cancelled')
		`, prescriptionID).Scan(&openItems); err != nil {
			return fmt.Errorf("check remaining prescription items: %w", err)
		}

		newRxStatus := "partially_dispensed"
		if openItems == 0 {
			newRxStatus = "fully_dispensed"
		}
		if _, err := tx.Exec(ctx, `
			UPDATE prescriptions
			SET status = $2::prescription_status_enum, updated_at = NOW()
			WHERE id = $1
		`, prescriptionID, newRxStatus); err != nil {
			return fmt.Errorf("update prescription status: %w", err)
		}

		if s.auditLogger != nil {
			actorID := actor.UserID
			entityID := eventID
			if err := s.auditLogger.AppendTx(ctx, tx, audit.Entry{
				ActorID:    &actorID,
				Action:     "prescription:dispense",
				EntityType: "dispensing_event",
				EntityID:   &entityID,
				Payload: map[string]any{
					"prescription_id":      prescriptionID,
					"prescription_item_id": tok.PrescriptionItemID,
					"quantity_dispensed":   req.Quantity,
					"total_dispensed":      newTotal,
					"prescription_status":  newRxStatus,
				},
			}); err != nil {
				return err
			}
		}

		result = &DispenseResult{
			DispensingEventID:  eventID,
			PrescriptionID:     prescriptionID,
			PrescriptionItemID: tok.PrescriptionItemID,
			QuantityDispensed:  req.Quantity,
			TotalDispensed:     newTotal,
			RemainingQuantity:  qtyPrescribed - newTotal,
			ItemStatus:         newItemStatus,
			PrescriptionStatus: newRxStatus,
			SignatureID:        sigID,
			DispensedAt:        now,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	s.logger.Info("medication_dispensed",
		"event_id", result.DispensingEventID,
		"item_id", result.PrescriptionItemID,
		"qty", result.QuantityDispensed,
		"rx_status", result.PrescriptionStatus,
	)
	return result, nil
}

func nullIfEmpty(s string) *string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return &s
}
