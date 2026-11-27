package cds

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"
)

// Service implements the CDS Engine / Checker interface.
type Service struct {
	repo     Repository
	logger   *slog.Logger
	ageRules []AgeRule
	nowFn    func() time.Time
}

// NewService constructs a CDS evaluation service.
func NewService(repo Repository, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{
		repo:     repo,
		logger:   logger,
		ageRules: defaultAgeRules,
		nowFn:    func() time.Time { return time.Now().UTC() },
	}
}

// Evaluate executes all four clinical safety checks and applies the severity decision policy.
func (s *Service) Evaluate(ctx context.Context, req CheckRequest) (*CheckResult, error) {
	start := s.nowFn()
	if err := validateRequest(req); err != nil {
		return nil, err
	}

	var (
		resolvedForms     []MedicineInfo
		activeMedicineIDs []uuid.UUID
		profile           *PatientClinicalProfile
		diagnosis         *DiagnosisInfo
	)

	g1, g1Ctx := errgroup.WithContext(ctx)
	g1.Go(func() error {
		var err error
		resolvedForms, err = s.repo.ResolveMedicineForms(g1Ctx, req.MedicineFormIDs)
		return err
	})
	g1.Go(func() error {
		var err error
		activeMedicineIDs, err = s.repo.GetActivePatientMedicineIDs(g1Ctx, req.PatientID)
		return err
	})
	g1.Go(func() error {
		var err error
		profile, err = s.repo.GetPatientProfile(g1Ctx, req.PatientID)
		return err
	})
	g1.Go(func() error {
		var err error
		diagnosis, err = s.repo.GetDiagnosis(g1Ctx, req.DiagnosisID)
		return err
	})
	if err := g1.Wait(); err != nil {
		return nil, err
	}

	if err := ensureAllFormsResolved(req.MedicineFormIDs, resolvedForms); err != nil {
		return nil, err
	}

	newMedIDs := uniqueNewMedicineIDs(resolvedForms)
	newMedSet := make(map[uuid.UUID]struct{}, len(newMedIDs))
	for _, id := range newMedIDs {
		newMedSet[id] = struct{}{}
	}
	combinedMedIDs := mergeUniqueIDs(newMedIDs, activeMedicineIDs)

	var (
		ddiMatches     []DDIMatch
		allergyMatches []AllergyMatch
	)

	g2, g2Ctx := errgroup.WithContext(ctx)
	g2.Go(func() error {
		var err error
		ddiMatches, err = s.repo.FindDrugInteractions(g2Ctx, combinedMedIDs)
		return err
	})
	g2.Go(func() error {
		var err error
		allergyMatches, err = s.repo.FindPatientAllergies(g2Ctx, req.PatientID, newMedIDs)
		return err
	})
	if err := g2.Wait(); err != nil {
		return nil, err
	}

	warnings := make([]Warning, 0, len(allergyMatches)+len(ddiMatches)+2)

	// Check B: Patient allergies (critical hard block).
	for _, a := range allergyMatches {
		msg := fmt.Sprintf("Patient has a documented allergy to %s", a.INNNameEN)
		if a.ReactionDescription != "" {
			msg = fmt.Sprintf("%s (%s)", msg, a.ReactionDescription)
		}
		warnings = append(warnings, Warning{
			Category:       CategoryAllergy,
			Severity:       SeverityCritical,
			Code:           "CDS_ALLERGY_MATCH",
			Message:        msg,
			MedicineIDs:    []uuid.UUID{a.MedicineID},
			BlocksIssuance: true,
			RequiresNote:   false,
		})
	}

	// Check A: Bidirectional Drug-Drug Interactions across new + active prescriptions.
	for _, d := range ddiMatches {
		_, aIsNew := newMedSet[d.MedicineAID]
		_, bIsNew := newMedSet[d.MedicineBID]
		if !aIsNew && !bIsNew {
			continue
		}
		warnings = append(warnings, Warning{
			Category:       CategoryDrugInteraction,
			Severity:       d.Severity,
			Code:           ddiCodeForSeverity(d.Severity),
			Message:        fmt.Sprintf("Interaction between %s and %s: %s", d.INNNameA, d.INNNameB, d.Description),
			MedicineIDs:    []uuid.UUID{d.MedicineAID, d.MedicineBID},
			BlocksIssuance: d.Severity == SeverityCritical,
			RequiresNote:   d.Severity == SeverityModerate,
		})
	}

	// Check C: Patient age contraindications.
	ageYears := AgeInYears(profile.DateOfBirth, s.nowFn())
	warnings = append(warnings, evaluateAgeRules(ageYears, resolvedForms, s.ageRules)...)

	// Check D: ICD-10 diagnosis compatibility / antibiotic stewardship.
	warnings = append(warnings, evaluateDiagnosisCompatibility(diagnosis, resolvedForms)...)

	result := applyDecisionPolicy(warnings, req.Justification, s.nowFn(), time.Since(start).Milliseconds())

	s.logger.Info("cds_evaluation",
		"patient_id", req.PatientID,
		"diagnosis_id", req.DiagnosisID,
		"medicine_count", len(newMedIDs),
		"warning_count", len(result.Warnings),
		"allowed", result.Allowed,
		"requires_justification", result.RequiresJustification,
		"duration_ms", result.DurationMs,
	)

	return result, nil
}

func validateRequest(req CheckRequest) error {
	if req.PatientID == uuid.Nil {
		return fmt.Errorf("%w: patient_id is required", ErrInvalidRequest)
	}
	if req.DiagnosisID <= 0 {
		return fmt.Errorf("%w: diagnosis_id must be positive", ErrInvalidRequest)
	}
	if len(req.MedicineFormIDs) == 0 {
		return fmt.Errorf("%w: at least one medicine_form_id is required", ErrInvalidRequest)
	}
	for _, id := range req.MedicineFormIDs {
		if id == uuid.Nil {
			return fmt.Errorf("%w: medicine_form_ids must not contain nil UUID", ErrInvalidRequest)
		}
	}
	return nil
}

func ensureAllFormsResolved(requested []uuid.UUID, resolved []MedicineInfo) error {
	found := make(map[uuid.UUID]struct{}, len(resolved))
	for _, info := range resolved {
		found[info.FormID] = struct{}{}
	}
	for _, reqID := range requested {
		if _, ok := found[reqID]; !ok {
			return fmt.Errorf("%w: %s", ErrMedicineFormNotFound, reqID)
		}
	}
	return nil
}

func uniqueNewMedicineIDs(forms []MedicineInfo) []uuid.UUID {
	seen := make(map[uuid.UUID]struct{}, len(forms))
	out := make([]uuid.UUID, 0, len(forms))
	for _, f := range forms {
		if _, ok := seen[f.MedicineID]; ok {
			continue
		}
		seen[f.MedicineID] = struct{}{}
		out = append(out, f.MedicineID)
	}
	return out
}

func mergeUniqueIDs(a, b []uuid.UUID) []uuid.UUID {
	out := make([]uuid.UUID, 0, len(a)+len(b))
	seen := make(map[uuid.UUID]struct{}, len(a)+len(b))
	for _, id := range a {
		if _, ok := seen[id]; !ok {
			seen[id] = struct{}{}
			out = append(out, id)
		}
	}
	for _, id := range b {
		if _, ok := seen[id]; !ok {
			seen[id] = struct{}{}
			out = append(out, id)
		}
	}
	return out
}

func ddiCodeForSeverity(severity string) string {
	switch severity {
	case SeverityCritical:
		return "CDS_DDI_CRITICAL"
	case SeverityModerate:
		return "CDS_DDI_MODERATE"
	default:
		return "CDS_DDI_MINOR"
	}
}

func applyDecisionPolicy(warnings []Warning, justification string, checkedAt time.Time, durationMs int64) *CheckResult {
	hasCritical := slices.ContainsFunc(warnings, func(w Warning) bool {
		return w.Severity == SeverityCritical
	})
	hasModerate := slices.ContainsFunc(warnings, func(w Warning) bool {
		return w.Severity == SeverityModerate
	})

	allowed := true
	requiresJustification := false

	if hasCritical {
		allowed = false
	} else if hasModerate && strings.TrimSpace(justification) == "" {
		allowed = false
		requiresJustification = true
	}

	return &CheckResult{
		Allowed:               allowed,
		RequiresJustification: requiresJustification,
		Warnings:              warnings,
		CheckedAt:             checkedAt,
		DurationMs:            durationMs,
	}
}
