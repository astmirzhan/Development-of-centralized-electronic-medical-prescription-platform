package cds

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

// Warning categories returned by the CDS engine.
const (
	CategoryAllergy                = "ALLERGY"
	CategoryDrugInteraction        = "DRUG_INTERACTION"
	CategoryAgeContraindication    = "AGE_CONTRAINDICATION"
	CategoryDiagnosisCompatibility = "DIAGNOSIS_COMPATIBILITY"
)

// Severity levels matching PostgreSQL ddi_severity_enum.
const (
	SeverityCritical = "critical"
	SeverityModerate = "moderate"
	SeverityMinor    = "minor"
)

// Domain errors returned by CDS evaluation and data lookup.
var (
	ErrInvalidRequest       = errors.New("invalid CDS check request")
	ErrPatientNotFound      = errors.New("patient profile not found")
	ErrDiagnosisNotFound    = errors.New("ICD-10 diagnosis code not found")
	ErrMedicineFormNotFound = errors.New("one or more medicine forms not found")
)

// CheckRequest represents input parameters for clinical safety validation.
type CheckRequest struct {
	PatientID       uuid.UUID   `json:"patient_id"`
	DiagnosisID     int         `json:"diagnosis_id"`
	MedicineFormIDs []uuid.UUID `json:"medicine_form_ids"`
	Justification   string      `json:"justification"`
}

// Warning describes a single clinical safety finding.
type Warning struct {
	Category       string      `json:"category"`
	Severity       string      `json:"severity"`
	Code           string      `json:"code"`
	Message        string      `json:"message"`
	MedicineIDs    []uuid.UUID `json:"medicine_ids"`
	BlocksIssuance bool        `json:"blocks_issuance"`
	RequiresNote   bool        `json:"requires_note"`
}

// CheckResult is the aggregated decision returned by the CDS engine.
type CheckResult struct {
	Allowed               bool      `json:"allowed"`
	RequiresJustification bool      `json:"requires_justification"`
	Warnings              []Warning `json:"warnings"`
	CheckedAt             time.Time `json:"checked_at"`
	DurationMs            int64     `json:"duration_ms"`
}

// Engine / Checker evaluates clinical safety rules prior to prescription issuance.
type Engine interface {
	Evaluate(ctx context.Context, req CheckRequest) (*CheckResult, error)
}

// Checker is an alias for Engine for callers preferring Checker naming.
type Checker = Engine

// MedicineInfo holds resolved INN metadata for a requested medicine form.
type MedicineInfo struct {
	FormID       uuid.UUID
	MedicineID   uuid.UUID
	INNNameEN    string
	ATCCode      string
	IsAntibiotic bool
	IsNarcotic   bool
}

// DDIMatch represents a bidirectional drug-drug interaction record.
type DDIMatch struct {
	MedicineAID uuid.UUID
	MedicineBID uuid.UUID
	INNNameA    string
	INNNameB    string
	Severity    string
	Description string
}

// AllergyMatch represents a patient allergy matching a prescribed medicine.
type AllergyMatch struct {
	MedicineID          uuid.UUID
	INNNameEN           string
	ReactionDescription string
}

// PatientClinicalProfile holds demographic context required by age rules.
type PatientClinicalProfile struct {
	PatientID   uuid.UUID
	DateOfBirth time.Time
	Gender      string
}

// DiagnosisInfo holds ICD-10 code metadata for diagnosis compatibility rules.
type DiagnosisInfo struct {
	ID     int
	Code   string
	NameRU string
	NameKZ string
}

// Repository abstracts batch clinical data queries for the CDS engine.
type Repository interface {
	ResolveMedicineForms(ctx context.Context, formIDs []uuid.UUID) ([]MedicineInfo, error)
	GetActivePatientMedicineIDs(ctx context.Context, patientID uuid.UUID) ([]uuid.UUID, error)
	FindDrugInteractions(ctx context.Context, medicineIDs []uuid.UUID) ([]DDIMatch, error)
	FindPatientAllergies(ctx context.Context, patientID uuid.UUID, medicineIDs []uuid.UUID) ([]AllergyMatch, error)
	GetPatientProfile(ctx context.Context, patientID uuid.UUID) (*PatientClinicalProfile, error)
	GetDiagnosis(ctx context.Context, diagnosisID int) (*DiagnosisInfo, error)
}
