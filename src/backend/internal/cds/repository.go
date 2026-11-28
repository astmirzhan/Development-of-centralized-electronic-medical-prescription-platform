package cds

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PGRepository implements Repository using batch queries on PostgreSQL 16.
type PGRepository struct {
	pool *pgxpool.Pool
}

// NewPGRepository creates a batch PostgreSQL repository for the CDS engine.
func NewPGRepository(pool *pgxpool.Pool) *PGRepository {
	return &PGRepository{pool: pool}
}

// ResolveMedicineForms resolves a batch of medicine_form_ids to their INN medicine metadata.
func (r *PGRepository) ResolveMedicineForms(ctx context.Context, formIDs []uuid.UUID) ([]MedicineInfo, error) {
	if len(formIDs) == 0 {
		return nil, nil
	}

	rows, err := r.pool.Query(ctx, `
		SELECT
			mf.id,
			m.id,
			m.inn_name_en,
			COALESCE(m.atc_code, ''),
			m.is_antibiotic,
			m.is_narcotic
		FROM medicine_forms mf
		JOIN medicines m ON m.id = mf.medicine_id
		WHERE mf.id = ANY($1)
	`, formIDs)
	if err != nil {
		return nil, fmt.Errorf("resolve medicine forms: %w", err)
	}
	defer rows.Close()

	var result []MedicineInfo
	for rows.Next() {
		var info MedicineInfo
		if err := rows.Scan(
			&info.FormID,
			&info.MedicineID,
			&info.INNNameEN,
			&info.ATCCode,
			&info.IsAntibiotic,
			&info.IsNarcotic,
		); err != nil {
			return nil, fmt.Errorf("scan medicine form: %w", err)
		}
		result = append(result, info)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate medicine forms: %w", err)
	}
	return result, nil
}

// GetActivePatientMedicineIDs returns distinct medicine_ids from the patient's active or
// partially dispensed prescriptions that have not expired.
func (r *PGRepository) GetActivePatientMedicineIDs(ctx context.Context, patientID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT DISTINCT mf.medicine_id
		FROM prescriptions p
		JOIN prescription_items pi ON pi.prescription_id = p.id
		JOIN medicine_forms mf ON mf.id = pi.medicine_form_id
		WHERE p.patient_id = $1
		  AND p.status IN ('active', 'partially_dispensed')
		  AND p.valid_until > NOW()
		  AND pi.status IN ('pending', 'partially_dispensed')
	`, patientID)
	if err != nil {
		return nil, fmt.Errorf("get active patient medicines: %w", err)
	}
	defer rows.Close()

	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan active medicine id: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate active medicine ids: %w", err)
	}
	return ids, nil
}

// FindDrugInteractions queries drug_interactions bidirectionally for any pair within medicineIDs.
func (r *PGRepository) FindDrugInteractions(ctx context.Context, medicineIDs []uuid.UUID) ([]DDIMatch, error) {
	if len(medicineIDs) < 2 {
		return nil, nil
	}

	rows, err := r.pool.Query(ctx, `
		SELECT
			di.medicine_a_id,
			di.medicine_b_id,
			ma.inn_name_en,
			mb.inn_name_en,
			di.severity_level::text,
			COALESCE(di.description, '')
		FROM drug_interactions di
		JOIN medicines ma ON ma.id = di.medicine_a_id
		JOIN medicines mb ON mb.id = di.medicine_b_id
		WHERE di.medicine_a_id = ANY($1)
		  AND di.medicine_b_id = ANY($1)
	`, medicineIDs)
	if err != nil {
		return nil, fmt.Errorf("find drug interactions: %w", err)
	}
	defer rows.Close()

	var matches []DDIMatch
	for rows.Next() {
		var m DDIMatch
		if err := rows.Scan(
			&m.MedicineAID,
			&m.MedicineBID,
			&m.INNNameA,
			&m.INNNameB,
			&m.Severity,
			&m.Description,
		); err != nil {
			return nil, fmt.Errorf("scan drug interaction: %w", err)
		}
		matches = append(matches, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate drug interactions: %w", err)
	}
	return matches, nil
}

// FindPatientAllergies checks patient_allergies against a batch of medicine_ids.
func (r *PGRepository) FindPatientAllergies(ctx context.Context, patientID uuid.UUID, medicineIDs []uuid.UUID) ([]AllergyMatch, error) {
	if len(medicineIDs) == 0 {
		return nil, nil
	}

	rows, err := r.pool.Query(ctx, `
		SELECT
			pa.medicine_id,
			m.inn_name_en,
			COALESCE(pa.reaction_description, '')
		FROM patient_allergies pa
		JOIN medicines m ON m.id = pa.medicine_id
		WHERE pa.patient_id = $1
		  AND pa.medicine_id = ANY($2)
	`, patientID, medicineIDs)
	if err != nil {
		return nil, fmt.Errorf("find patient allergies: %w", err)
	}
	defer rows.Close()

	var matches []AllergyMatch
	for rows.Next() {
		var m AllergyMatch
		if err := rows.Scan(&m.MedicineID, &m.INNNameEN, &m.ReactionDescription); err != nil {
			return nil, fmt.Errorf("scan patient allergy: %w", err)
		}
		matches = append(matches, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate patient allergies: %w", err)
	}
	return matches, nil
}

// GetPatientProfile retrieves date of birth and gender for age contraindication checks.
func (r *PGRepository) GetPatientProfile(ctx context.Context, patientID uuid.UUID) (*PatientClinicalProfile, error) {
	var profile PatientClinicalProfile
	err := r.pool.QueryRow(ctx, `
		SELECT user_id, date_of_birth, gender
		FROM patient_profiles
		WHERE user_id = $1
	`, patientID).Scan(&profile.PatientID, &profile.DateOfBirth, &profile.Gender)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrPatientNotFound
		}
		return nil, fmt.Errorf("get patient profile: %w", err)
	}
	return &profile, nil
}

// GetDiagnosis fetches an ICD-10 record by primary key.
func (r *PGRepository) GetDiagnosis(ctx context.Context, diagnosisID int) (*DiagnosisInfo, error) {
	var diag DiagnosisInfo
	err := r.pool.QueryRow(ctx, `
		SELECT id, code, name_ru, name_kz
		FROM icd10_codes
		WHERE id = $1
	`, diagnosisID).Scan(&diag.ID, &diag.Code, &diag.NameRU, &diag.NameKZ)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrDiagnosisNotFound
		}
		return nil, fmt.Errorf("get diagnosis: %w", err)
	}
	return &diag, nil
}

// IsVerifiedDoctor checks whether doctorID holds a verified doctor profile linked to a verified clinic.
func (r *PGRepository) IsVerifiedDoctor(ctx context.Context, doctorID uuid.UUID) (bool, error) {
	var verified bool
	err := r.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM doctor_profiles dp
			JOIN doctor_clinics dc ON dc.doctor_id = dp.user_id
			JOIN organizations o ON o.id = dc.clinic_id
			WHERE dp.user_id = $1
			  AND dp.is_verified = true
			  AND o.org_type = 'clinic'
			  AND o.is_verified = true
		)
	`, doctorID).Scan(&verified)
	if err != nil {
		return false, fmt.Errorf("verify doctor access: %w", err)
	}
	return verified, nil
}
