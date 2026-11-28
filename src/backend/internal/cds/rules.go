package cds

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// AgeRule defines a minimum safe age constraint for a medicine class or INN.
type AgeRule struct {
	Code      string
	ATCPrefix string
	INNName   string
	MinAge    int
	Severity  string
	Reason    string
}

var defaultAgeRules = []AgeRule{
	{
		Code:      "CDS_AGE_FLUOROQUINOLONE_PEDIATRIC",
		ATCPrefix: "J01MA",
		INNName:   "ciprofloxacin",
		MinAge:    18,
		Severity:  SeverityCritical,
		Reason:    "Fluoroquinolones are contraindicated in patients under 18 years due to musculoskeletal/cartilage toxicity risk",
	},
	{
		Code:      "CDS_AGE_ASPIRIN_PEDIATRIC",
		ATCPrefix: "B01AC06",
		INNName:   "aspirin",
		MinAge:    16,
		Severity:  SeverityCritical,
		Reason:    "Aspirin is contraindicated under 16 years due to Reye's syndrome risk",
	},
	{
		Code:      "CDS_AGE_TETRACYCLINE_PEDIATRIC",
		ATCPrefix: "J01AA",
		INNName:   "doxycycline",
		MinAge:    8,
		Severity:  SeverityCritical,
		Reason:    "Tetracyclines are contraindicated in children under 8 years due to permanent tooth discoloration",
	},
}

// viralICD10Prefixes lists common outpatient viral/non-bacterial diagnoses where
// antibiotic prescribing requires explicit clinical justification (stewardship).
var viralICD10Codes = map[string]bool{
	"J00":   true,
	"J02.9": true,
	"J06.9": true,
	"J20.9": true,
	"A09":   true,
}

// AgeInYears computes completed calendar years between dob and now in UTC.
func AgeInYears(dob, now time.Time) int {
	d := dob.UTC()
	n := now.UTC()
	years := n.Year() - d.Year()
	if n.Month() < d.Month() || (n.Month() == d.Month() && n.Day() < d.Day()) {
		years--
	}
	if years < 0 {
		return 0
	}
	return years
}

func evaluateAgeRules(ageYears int, medicines []MedicineInfo, rules []AgeRule) []Warning {
	var warnings []Warning
	seen := make(map[string]struct{})

	for _, med := range medicines {
		atc := strings.ToUpper(strings.TrimSpace(med.ATCCode))
		inn := strings.ToLower(strings.TrimSpace(med.INNNameEN))

		for _, rule := range rules {
			if ageYears >= rule.MinAge {
				continue
			}
			matchesATC := rule.ATCPrefix != "" && strings.HasPrefix(atc, rule.ATCPrefix)
			matchesINN := rule.INNName != "" && inn == rule.INNName
			if !matchesATC && !matchesINN {
				continue
			}

			key := rule.Code + ":" + med.MedicineID.String()
			if _, dup := seen[key]; dup {
				continue
			}
			seen[key] = struct{}{}

			warnings = append(warnings, Warning{
				Category:       CategoryAgeContraindication,
				Severity:       rule.Severity,
				Code:           rule.Code,
				Message:        fmt.Sprintf("%s (patient age: %d, minimum age: %d): %s", med.INNNameEN, ageYears, rule.MinAge, rule.Reason),
				MedicineIDs:    []uuid.UUID{med.MedicineID},
				BlocksIssuance: rule.Severity == SeverityCritical,
				RequiresNote:   rule.Severity == SeverityModerate,
			})
		}
	}
	return warnings
}

func evaluateDiagnosisCompatibility(diag *DiagnosisInfo, medicines []MedicineInfo) []Warning {
	if diag == nil {
		return nil
	}
	code := strings.ToUpper(strings.TrimSpace(diag.Code))
	isViral := viralICD10Codes[code] || strings.HasPrefix(code, "J06") || strings.HasPrefix(code, "J00")
	if !isViral {
		return nil
	}

	var antibioticIDs []uuid.UUID
	var antibioticNames []string
	seen := make(map[uuid.UUID]struct{})
	for _, med := range medicines {
		if !med.IsAntibiotic {
			continue
		}
		if _, dup := seen[med.MedicineID]; dup {
			continue
		}
		seen[med.MedicineID] = struct{}{}
		antibioticIDs = append(antibioticIDs, med.MedicineID)
		antibioticNames = append(antibioticNames, med.INNNameEN)
	}
	if len(antibioticIDs) == 0 {
		return nil
	}

	return []Warning{
		{
			Category:       CategoryDiagnosisCompatibility,
			Severity:       SeverityModerate,
			Code:           "CDS_ANTIBIOTIC_STEWARDSHIP",
			Message:        fmt.Sprintf("Antibiotic (%s) prescribed for typically viral diagnosis %s (%s); clinical justification is required", strings.Join(antibioticNames, ", "), diag.Code, diag.NameRU),
			MedicineIDs:    antibioticIDs,
			BlocksIssuance: false,
			RequiresNote:   true,
		},
	}
}
