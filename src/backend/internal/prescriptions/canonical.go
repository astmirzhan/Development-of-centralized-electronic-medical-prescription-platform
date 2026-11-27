package prescriptions

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
)

// CanonicalItem represents an immutable prescription line item for EDS signing.
// Field declaration order defines the canonical JSON key ordering.
type CanonicalItem struct {
	MedicineFormID     string `json:"medicine_form_id"`
	DosageValue        string `json:"dosage_value"`
	DosageUnit         string `json:"dosage_unit"`
	Frequency          string `json:"frequency"`
	DurationDays       int    `json:"duration_days"`
	QuantityPrescribed int    `json:"quantity_prescribed"`
}

// CanonicalPrescription represents the immutable prescription payload signed by the doctor's EDS.
// Field declaration order defines the canonical JSON key ordering.
type CanonicalPrescription struct {
	PrescriptionID     string          `json:"prescription_id"`
	PrescriptionNumber string          `json:"prescription_number"`
	DoctorIIN          string          `json:"doctor_iin"`
	ClinicBIN          string          `json:"clinic_bin"`
	PatientIIN         string          `json:"patient_iin"`
	DiagnosisCode      string          `json:"diagnosis_code"`
	ValidUntil         string          `json:"valid_until"`
	CreatedAt          string          `json:"created_at"`
	Items              []CanonicalItem `json:"items"`
}

// PrescriptionItemInput holds raw item values before canonical normalization.
type PrescriptionItemInput struct {
	MedicineFormID     uuid.UUID
	DosageValue        float64
	DosageUnit         string
	Frequency          string
	DurationDays       int
	QuantityPrescribed int
}

// PrescriptionInput holds raw prescription values before canonical normalization.
type PrescriptionInput struct {
	PrescriptionID     uuid.UUID
	PrescriptionNumber string
	DoctorIIN          string
	ClinicBIN          string
	PatientIIN         string
	DiagnosisCode      string
	ValidUntil         time.Time
	CreatedAt          time.Time
	Items              []PrescriptionItemInput
}

// FormatDosage formats a DECIMAL(10,2) value deterministically to avoid floating-point drift.
func FormatDosage(v float64) string {
	rounded := math.Round(v*100) / 100
	return fmt.Sprintf("%.2f", rounded)
}

// BuildCanonical constructs a normalized, deterministically sorted CanonicalPrescription.
func BuildCanonical(in PrescriptionInput) (CanonicalPrescription, error) {
	if in.PrescriptionID == uuid.Nil {
		return CanonicalPrescription{}, fmt.Errorf("prescription_id is required")
	}
	if strings.TrimSpace(in.PrescriptionNumber) == "" {
		return CanonicalPrescription{}, fmt.Errorf("prescription_number is required")
	}
	if len(in.Items) == 0 {
		return CanonicalPrescription{}, fmt.Errorf("prescription must have at least one item")
	}

	items := make([]CanonicalItem, 0, len(in.Items))
	for _, item := range in.Items {
		items = append(items, CanonicalItem{
			MedicineFormID:     item.MedicineFormID.String(),
			DosageValue:        FormatDosage(item.DosageValue),
			DosageUnit:         strings.TrimSpace(item.DosageUnit),
			Frequency:          strings.TrimSpace(item.Frequency),
			DurationDays:       item.DurationDays,
			QuantityPrescribed: item.QuantityPrescribed,
		})
	}

	slices.SortFunc(items, compareCanonicalItems)

	return CanonicalPrescription{
		PrescriptionID:     in.PrescriptionID.String(),
		PrescriptionNumber: strings.TrimSpace(in.PrescriptionNumber),
		DoctorIIN:          strings.TrimSpace(in.DoctorIIN),
		ClinicBIN:          strings.TrimSpace(in.ClinicBIN),
		PatientIIN:         strings.TrimSpace(in.PatientIIN),
		DiagnosisCode:      strings.TrimSpace(in.DiagnosisCode),
		ValidUntil:         in.ValidUntil.UTC().Format(time.RFC3339),
		CreatedAt:          in.CreatedAt.UTC().Format(time.RFC3339),
		Items:              items,
	}, nil
}

// SerializeCanonical produces compact canonical JSON bytes and its SHA-256 hex digest.
func SerializeCanonical(in PrescriptionInput) ([]byte, string, error) {
	canonical, err := BuildCanonical(in)
	if err != nil {
		return nil, "", err
	}
	raw, err := json.Marshal(canonical)
	if err != nil {
		return nil, "", fmt.Errorf("marshal canonical prescription: %w", err)
	}
	sum := sha256.Sum256(raw)
	return raw, hex.EncodeToString(sum[:]), nil
}

// HashCanonicalBytes returns the lowercase hex SHA-256 digest of canonical JSON bytes.
func HashCanonicalBytes(canonicalJSON []byte) string {
	sum := sha256.Sum256(canonicalJSON)
	return hex.EncodeToString(sum[:])
}

// VerifyCanonicalHash checks whether the input prescription matches expectedSHA256Hex.
func VerifyCanonicalHash(in PrescriptionInput, expectedSHA256Hex string) (bool, error) {
	_, actualHash, err := SerializeCanonical(in)
	if err != nil {
		return false, err
	}
	normalizedExpected := strings.ToLower(strings.TrimSpace(expectedSHA256Hex))
	if len(normalizedExpected) != len(actualHash) {
		return false, nil
	}
	return subtle.ConstantTimeCompare([]byte(actualHash), []byte(normalizedExpected)) == 1, nil
}

func compareCanonicalItems(a, b CanonicalItem) int {
	if c := strings.Compare(a.MedicineFormID, b.MedicineFormID); c != 0 {
		return c
	}
	if c := strings.Compare(a.DosageValue, b.DosageValue); c != 0 {
		return c
	}
	if c := strings.Compare(a.DosageUnit, b.DosageUnit); c != 0 {
		return c
	}
	if c := strings.Compare(a.Frequency, b.Frequency); c != 0 {
		return c
	}
	if a.DurationDays != b.DurationDays {
		if a.DurationDays < b.DurationDays {
			return -1
		}
		return 1
	}
	if a.QuantityPrescribed != b.QuantityPrescribed {
		if a.QuantityPrescribed < b.QuantityPrescribed {
			return -1
		}
		return 1
	}
	return 0
}
