package cds

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/xjam-byte/erx-platform/backend/internal/httpserver"
)

type stubRepo struct {
	forms      map[uuid.UUID]MedicineInfo
	activeMeds []uuid.UUID
	ddis       []DDIMatch
	allergies  []AllergyMatch
	profile    *PatientClinicalProfile
	diagnosis  *DiagnosisInfo
}

func (s *stubRepo) ResolveMedicineForms(_ context.Context, formIDs []uuid.UUID) ([]MedicineInfo, error) {
	out := make([]MedicineInfo, 0, len(formIDs))
	for _, id := range formIDs {
		if info, ok := s.forms[id]; ok {
			out = append(out, info)
		}
	}
	return out, nil
}

func (s *stubRepo) GetActivePatientMedicineIDs(_ context.Context, _ uuid.UUID) ([]uuid.UUID, error) {
	return s.activeMeds, nil
}

func (s *stubRepo) FindDrugInteractions(_ context.Context, medicineIDs []uuid.UUID) ([]DDIMatch, error) {
	set := make(map[uuid.UUID]struct{}, len(medicineIDs))
	for _, id := range medicineIDs {
		set[id] = struct{}{}
	}
	var out []DDIMatch
	for _, d := range s.ddis {
		_, okA := set[d.MedicineAID]
		_, okB := set[d.MedicineBID]
		if okA && okB {
			out = append(out, d)
		}
	}
	return out, nil
}

func (s *stubRepo) FindPatientAllergies(_ context.Context, _ uuid.UUID, medicineIDs []uuid.UUID) ([]AllergyMatch, error) {
	set := make(map[uuid.UUID]struct{}, len(medicineIDs))
	for _, id := range medicineIDs {
		set[id] = struct{}{}
	}
	var out []AllergyMatch
	for _, a := range s.allergies {
		if _, ok := set[a.MedicineID]; ok {
			out = append(out, a)
		}
	}
	return out, nil
}

func (s *stubRepo) GetPatientProfile(_ context.Context, _ uuid.UUID) (*PatientClinicalProfile, error) {
	if s.profile == nil {
		return nil, ErrPatientNotFound
	}
	return s.profile, nil
}

func (s *stubRepo) GetDiagnosis(_ context.Context, _ int) (*DiagnosisInfo, error) {
	if s.diagnosis == nil {
		return nil, ErrDiagnosisNotFound
	}
	return s.diagnosis, nil
}

func newFixtureRepo() (*stubRepo, map[string]uuid.UUID, map[string]uuid.UUID) {
	medIDs := map[string]uuid.UUID{
		"warfarin":      uuid.New(),
		"aspirin":       uuid.New(),
		"ciprofloxacin": uuid.New(),
		"ibuprofen":     uuid.New(),
		"amoxicillin":   uuid.New(),
		"paracetamol":   uuid.New(),
	}
	formIDs := map[string]uuid.UUID{
		"warfarin":      uuid.New(),
		"aspirin":       uuid.New(),
		"ciprofloxacin": uuid.New(),
		"ibuprofen":     uuid.New(),
		"amoxicillin":   uuid.New(),
		"paracetamol":   uuid.New(),
	}

	repo := &stubRepo{
		forms: map[uuid.UUID]MedicineInfo{
			formIDs["warfarin"]:      {FormID: formIDs["warfarin"], MedicineID: medIDs["warfarin"], INNNameEN: "Warfarin", ATCCode: "B01AA03"},
			formIDs["aspirin"]:       {FormID: formIDs["aspirin"], MedicineID: medIDs["aspirin"], INNNameEN: "Aspirin", ATCCode: "B01AC06"},
			formIDs["ciprofloxacin"]: {FormID: formIDs["ciprofloxacin"], MedicineID: medIDs["ciprofloxacin"], INNNameEN: "Ciprofloxacin", ATCCode: "J01MA02", IsAntibiotic: true},
			formIDs["ibuprofen"]:     {FormID: formIDs["ibuprofen"], MedicineID: medIDs["ibuprofen"], INNNameEN: "Ibuprofen", ATCCode: "M01AE01"},
			formIDs["amoxicillin"]:   {FormID: formIDs["amoxicillin"], MedicineID: medIDs["amoxicillin"], INNNameEN: "Amoxicillin", ATCCode: "J01CA04", IsAntibiotic: true},
			formIDs["paracetamol"]:   {FormID: formIDs["paracetamol"], MedicineID: medIDs["paracetamol"], INNNameEN: "Paracetamol", ATCCode: "N02BE01"},
		},
		ddis: []DDIMatch{
			{
				MedicineAID: medIDs["warfarin"],
				MedicineBID: medIDs["aspirin"],
				INNNameA:    "Warfarin",
				INNNameB:    "Aspirin",
				Severity:    SeverityCritical,
				Description: "Severe bleeding risk",
			},
			{
				MedicineAID: medIDs["ciprofloxacin"],
				MedicineBID: medIDs["ibuprofen"],
				INNNameA:    "Ciprofloxacin",
				INNNameB:    "Ibuprofen",
				Severity:    SeverityModerate,
				Description: "CNS stimulation risk",
			},
			{
				MedicineAID: medIDs["amoxicillin"],
				MedicineBID: medIDs["ibuprofen"],
				INNNameA:    "Amoxicillin",
				INNNameB:    "Ibuprofen",
				Severity:    SeverityMinor,
				Description: "Mild GI irritation",
			},
		},
		profile: &PatientClinicalProfile{
			PatientID:   uuid.New(),
			DateOfBirth: time.Date(1995, 5, 10, 0, 0, 0, 0, time.UTC),
			Gender:      "male",
		},
		diagnosis: &DiagnosisInfo{
			ID:     1,
			Code:   "J18.9",
			NameRU: "Пневмония неуточненная",
		},
	}
	return repo, medIDs, formIDs
}

func TestCDSEngineDecisions(t *testing.T) {
	t.Run("critical DDI blocks issuance even with justification", func(t *testing.T) {
		repo, _, formIDs := newFixtureRepo()
		svc := NewService(repo, nil)

		res, err := svc.Evaluate(context.Background(), CheckRequest{
			PatientID:       repo.profile.PatientID,
			DiagnosisID:     1,
			MedicineFormIDs: []uuid.UUID{formIDs["aspirin"], formIDs["warfarin"]},
			Justification:   "Doctor override attempt",
		})
		if err != nil {
			t.Fatalf("Evaluate() error: %v", err)
		}
		if res.Allowed {
			t.Fatal("expected Allowed=false for critical DDI")
		}
		if len(res.Warnings) != 1 || res.Warnings[0].Code != "CDS_DDI_CRITICAL" || !res.Warnings[0].BlocksIssuance {
			t.Fatalf("unexpected warnings: %#v", res.Warnings)
		}
	})

	t.Run("cross-prescription DDI detects active medications", func(t *testing.T) {
		repo, medIDs, formIDs := newFixtureRepo()
		repo.activeMeds = []uuid.UUID{medIDs["warfarin"]}
		svc := NewService(repo, nil)

		res, err := svc.Evaluate(context.Background(), CheckRequest{
			PatientID:       repo.profile.PatientID,
			DiagnosisID:     1,
			MedicineFormIDs: []uuid.UUID{formIDs["aspirin"]},
		})
		if err != nil {
			t.Fatalf("Evaluate() error: %v", err)
		}
		if res.Allowed {
			t.Fatal("expected Allowed=false due to active Warfarin + new Aspirin")
		}
	})

	t.Run("patient allergy blocks issuance", func(t *testing.T) {
		repo, medIDs, formIDs := newFixtureRepo()
		repo.allergies = []AllergyMatch{{
			MedicineID:          medIDs["amoxicillin"],
			INNNameEN:           "Amoxicillin",
			ReactionDescription: "Anaphylaxis",
		}}
		svc := NewService(repo, nil)

		res, err := svc.Evaluate(context.Background(), CheckRequest{
			PatientID:       repo.profile.PatientID,
			DiagnosisID:     1,
			MedicineFormIDs: []uuid.UUID{formIDs["amoxicillin"]},
			Justification:   "Urgent",
		})
		if err != nil {
			t.Fatalf("Evaluate() error: %v", err)
		}
		if res.Allowed {
			t.Fatal("expected Allowed=false for allergy match")
		}
		if len(res.Warnings) == 0 || res.Warnings[0].Category != CategoryAllergy {
			t.Fatalf("expected allergy warning, got %#v", res.Warnings)
		}
	})

	t.Run("moderate antibiotic stewardship blocks without justification and allows with justification", func(t *testing.T) {
		repo, _, formIDs := newFixtureRepo()
		repo.diagnosis = &DiagnosisInfo{ID: 2, Code: "J06.9", NameRU: "ОРВИ"}
		svc := NewService(repo, nil)

		resNoNote, err := svc.Evaluate(context.Background(), CheckRequest{
			PatientID:       repo.profile.PatientID,
			DiagnosisID:     2,
			MedicineFormIDs: []uuid.UUID{formIDs["amoxicillin"]},
		})
		if err != nil {
			t.Fatalf("Evaluate() error: %v", err)
		}
		if resNoNote.Allowed || !resNoNote.RequiresJustification {
			t.Fatalf("expected Allowed=false, RequiresJustification=true; got %#v", resNoNote)
		}

		resWithNote, err := svc.Evaluate(context.Background(), CheckRequest{
			PatientID:       repo.profile.PatientID,
			DiagnosisID:     2,
			MedicineFormIDs: []uuid.UUID{formIDs["amoxicillin"]},
			Justification:   "Confirmed secondary bacterial sinusitis by CRP/CBC",
		})
		if err != nil {
			t.Fatalf("Evaluate() error: %v", err)
		}
		if !resWithNote.Allowed || resWithNote.RequiresJustification {
			t.Fatalf("expected Allowed=true with justification; got %#v", resWithNote)
		}
		if len(resWithNote.Warnings) != 1 || resWithNote.Warnings[0].Code != "CDS_ANTIBIOTIC_STEWARDSHIP" {
			t.Fatalf("expected stewardship warning retained, got %#v", resWithNote.Warnings)
		}
	})

	t.Run("minor DDI allows issuance with informational warning", func(t *testing.T) {
		repo, _, formIDs := newFixtureRepo()
		svc := NewService(repo, nil)

		res, err := svc.Evaluate(context.Background(), CheckRequest{
			PatientID:       repo.profile.PatientID,
			DiagnosisID:     1,
			MedicineFormIDs: []uuid.UUID{formIDs["amoxicillin"], formIDs["ibuprofen"]},
		})
		if err != nil {
			t.Fatalf("Evaluate() error: %v", err)
		}
		if !res.Allowed || res.RequiresJustification {
			t.Fatalf("expected Allowed=true for minor DDI; got %#v", res)
		}
		if len(res.Warnings) != 1 || res.Warnings[0].Severity != SeverityMinor {
			t.Fatalf("expected 1 minor warning, got %#v", res.Warnings)
		}
	})

	t.Run("pediatric fluoroquinolone contraindication blocks under 18", func(t *testing.T) {
		repo, _, formIDs := newFixtureRepo()
		repo.profile.DateOfBirth = time.Date(2015, 5, 10, 0, 0, 0, 0, time.UTC)
		svc := NewService(repo, nil)
		svc.nowFn = func() time.Time { return time.Date(2026, 11, 27, 0, 0, 0, 0, time.UTC) }

		res, err := svc.Evaluate(context.Background(), CheckRequest{
			PatientID:       repo.profile.PatientID,
			DiagnosisID:     1,
			MedicineFormIDs: []uuid.UUID{formIDs["ciprofloxacin"]},
		})
		if err != nil {
			t.Fatalf("Evaluate() error: %v", err)
		}
		if res.Allowed {
			t.Fatal("expected Allowed=false for pediatric Ciprofloxacin")
		}
		if len(res.Warnings) == 0 || res.Warnings[0].Code != "CDS_AGE_FLUOROQUINOLONE_PEDIATRIC" {
			t.Fatalf("expected pediatric warning, got %#v", res.Warnings)
		}
	})
}

func TestAgeInYears(t *testing.T) {
	dob := time.Date(2008, 11, 28, 0, 0, 0, 0, time.UTC)
	dayBefore := time.Date(2026, 11, 27, 12, 0, 0, 0, time.UTC)
	onBirthday := time.Date(2026, 11, 28, 0, 0, 0, 0, time.UTC)

	if got := AgeInYears(dob, dayBefore); got != 17 {
		t.Fatalf("expected 17 day before birthday, got %d", got)
	}
	if got := AgeInYears(dob, onBirthday); got != 18 {
		t.Fatalf("expected 18 on birthday, got %d", got)
	}
}

func TestCDSHTTPHandler(t *testing.T) {
	repo, _, formIDs := newFixtureRepo()
	svc := NewService(repo, nil)
	h := NewHandler(svc, nil)
	router := httpserver.NewRouter(httpserver.RouterDeps{
		RegisterRoutes: func(r chi.Router) {
			h.RegisterRoutes(r)
		},
	})

	payload, _ := json.Marshal(CheckRequest{
		PatientID:       repo.profile.PatientID,
		DiagnosisID:     1,
		MedicineFormIDs: []uuid.UUID{formIDs["paracetamol"]},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/cds/check", bytes.NewReader(payload))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}

	var res CheckResult
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if !res.Allowed {
		t.Fatalf("expected Allowed=true, got %#v", res)
	}

	badReq := httptest.NewRequest(http.MethodPost, "/api/v1/cds/check", bytes.NewBufferString(`{}`))
	badRec := httptest.NewRecorder()
	router.ServeHTTP(badRec, badReq)
	if badRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request, got %d", badRec.Code)
	}
}

func BenchmarkEngineEvaluate(b *testing.B) {
	repo, _, formIDs := newFixtureRepo()
	svc := NewService(repo, nil)
	req := CheckRequest{
		PatientID:       repo.profile.PatientID,
		DiagnosisID:     1,
		MedicineFormIDs: []uuid.UUID{formIDs["ciprofloxacin"], formIDs["ibuprofen"]},
		Justification:   "Clinical necessity",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		res, err := svc.Evaluate(context.Background(), req)
		if err != nil {
			b.Fatalf("Evaluate() error: %v", err)
		}
		if res.DurationMs >= 150 {
			b.Fatalf("expected DurationMs < 150ms, got %d", res.DurationMs)
		}
	}
}
