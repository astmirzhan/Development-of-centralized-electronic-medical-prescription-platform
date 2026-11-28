package prescriptions

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Domain errors for prescription lookup and signing lifecycle.
var (
	ErrPrescriptionNotFound    = errors.New("prescription not found")
	ErrForbiddenActor          = errors.New("actor is not authorized for this prescription")
	ErrInvalidStatusTransition = errors.New("prescription must be in draft status to sign")
	ErrInvalidSigningRequest   = errors.New("cms_signature is required")
)

// PrescriptionDetails carries all immutable fields required for canonicalization and signing.
type PrescriptionDetails struct {
	Input    PrescriptionInput
	DoctorID uuid.UUID
	Status   string
}

// TxRunner runs a callback inside a PostgreSQL transaction.
type TxRunner interface {
	WithTx(ctx context.Context, fn func(pgx.Tx) error) error
}

// Repository loads prescription data and updates signing status.
type Repository interface {
	GetDetails(ctx context.Context, prescriptionID uuid.UUID) (*PrescriptionDetails, error)
	LockDetailsForSignTx(ctx context.Context, tx pgx.Tx, prescriptionID uuid.UUID) (*PrescriptionDetails, error)
	ActivateTx(ctx context.Context, tx pgx.Tx, prescriptionID uuid.UUID) error
}

// PGRepository implements Repository and TxRunner against PostgreSQL 16.
type PGRepository struct {
	pool *pgxpool.Pool
}

// NewPGRepository creates a PostgreSQL prescription repository.
func NewPGRepository(pool *pgxpool.Pool) *PGRepository {
	return &PGRepository{pool: pool}
}

// WithTx executes fn inside a transaction, rolling back on error or panic.
func (r *PGRepository) WithTx(ctx context.Context, fn func(pgx.Tx) error) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if rec := recover(); rec != nil {
			_ = tx.Rollback(context.Background())
			panic(rec)
		}
	}()
	if err := fn(tx); err != nil {
		_ = tx.Rollback(context.Background())
		return err
	}
	return tx.Commit(ctx)
}

type rowQuerier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// GetDetails loads prescription header, actor IIN/BINs, ICD-10 code, and items.
func (r *PGRepository) GetDetails(ctx context.Context, prescriptionID uuid.UUID) (*PrescriptionDetails, error) {
	return loadPrescriptionDetails(ctx, r.pool, prescriptionID, false)
}

// LockDetailsForSignTx locks the prescription row FOR UPDATE inside tx and loads its details.
func (r *PGRepository) LockDetailsForSignTx(ctx context.Context, tx pgx.Tx, prescriptionID uuid.UUID) (*PrescriptionDetails, error) {
	return loadPrescriptionDetails(ctx, tx, prescriptionID, true)
}

// ActivateTx transitions a draft prescription to active status.
func (r *PGRepository) ActivateTx(ctx context.Context, tx pgx.Tx, prescriptionID uuid.UUID) error {
	tag, err := tx.Exec(ctx, `
		UPDATE prescriptions
		SET status = 'active', updated_at = NOW()
		WHERE id = $1 AND status = 'draft'
	`, prescriptionID)
	if err != nil {
		return fmt.Errorf("activate prescription: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return ErrInvalidStatusTransition
	}
	return nil
}

func loadPrescriptionDetails(ctx context.Context, q rowQuerier, prescriptionID uuid.UUID, forUpdate bool) (*PrescriptionDetails, error) {
	lockClause := ""
	if forUpdate {
		lockClause = " FOR UPDATE OF p"
	}

	var details PrescriptionDetails
	err := q.QueryRow(ctx, `
		SELECT
			p.id,
			p.prescription_number,
			p.doctor_id,
			doc.iin,
			org.bin,
			pat.iin,
			icd.code,
			p.status::text,
			p.valid_until,
			p.created_at
		FROM prescriptions p
		JOIN users doc ON doc.id = p.doctor_id
		JOIN organizations org ON org.id = p.clinic_id
		JOIN users pat ON pat.id = p.patient_id
		JOIN icd10_codes icd ON icd.id = p.diagnosis_id
		WHERE p.id = $1`+lockClause, prescriptionID).Scan(
		&details.Input.PrescriptionID,
		&details.Input.PrescriptionNumber,
		&details.DoctorID,
		&details.Input.DoctorIIN,
		&details.Input.ClinicBIN,
		&details.Input.PatientIIN,
		&details.Input.DiagnosisCode,
		&details.Status,
		&details.Input.ValidUntil,
		&details.Input.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrPrescriptionNotFound
		}
		return nil, fmt.Errorf("load prescription header: %w", err)
	}

	rows, err := q.Query(ctx, `
		SELECT
			medicine_form_id,
			dosage_value::float8,
			dosage_unit,
			frequency,
			duration_days,
			quantity_prescribed
		FROM prescription_items
		WHERE prescription_id = $1
		ORDER BY medicine_form_id ASC, id ASC
	`, prescriptionID)
	if err != nil {
		return nil, fmt.Errorf("load prescription items: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var item PrescriptionItemInput
		if err := rows.Scan(
			&item.MedicineFormID,
			&item.DosageValue,
			&item.DosageUnit,
			&item.Frequency,
			&item.DurationDays,
			&item.QuantityPrescribed,
		); err != nil {
			return nil, fmt.Errorf("scan prescription item: %w", err)
		}
		details.Input.Items = append(details.Input.Items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate prescription items: %w", err)
	}
	return &details, nil
}
