package cds

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/xjam-byte/erx-platform/backend/internal/auth"
	"github.com/xjam-byte/erx-platform/backend/internal/httpserver"
)

type failingRepo struct {
	stubRepo
	resolveErr error
	activeErr  error
	ddiErr     error
	allergyErr error
}

func (f *failingRepo) ResolveMedicineForms(ctx context.Context, ids []uuid.UUID) ([]MedicineInfo, error) {
	if f.resolveErr != nil {
		return nil, f.resolveErr
	}
	return f.stubRepo.ResolveMedicineForms(ctx, ids)
}

func (f *failingRepo) GetActivePatientMedicineIDs(ctx context.Context, pid uuid.UUID) ([]uuid.UUID, error) {
	if f.activeErr != nil {
		return nil, f.activeErr
	}
	return f.stubRepo.GetActivePatientMedicineIDs(ctx, pid)
}

func (f *failingRepo) FindDrugInteractions(ctx context.Context, ids []uuid.UUID) ([]DDIMatch, error) {
	if f.ddiErr != nil {
		return nil, f.ddiErr
	}
	return f.stubRepo.FindDrugInteractions(ctx, ids)
}

func (f *failingRepo) FindPatientAllergies(ctx context.Context, pid uuid.UUID, ids []uuid.UUID) ([]AllergyMatch, error) {
	if f.allergyErr != nil {
		return nil, f.allergyErr
	}
	return f.stubRepo.FindPatientAllergies(ctx, pid, ids)
}

func TestCDSBidirectionalDDIAndEdgeCases(t *testing.T) {
	t.Run("bidirectional DDI detects (A,B) when queried as (B,A) or (A,B)", func(t *testing.T) {
		repo, medIDs, formIDs := newFixtureRepo()
		// Add an interaction stored strictly as (ibuprofen, ciprofloxacin) and test both orders.
		repo.ddis = []DDIMatch{{
			MedicineAID: medIDs["ibuprofen"],
			MedicineBID: medIDs["ciprofloxacin"],
			INNNameA:    "Ibuprofen",
			INNNameB:    "Ciprofloxacin",
			Severity:    SeverityModerate,
			Description: "Bidirectional moderate DDI",
		}}
		svc := NewService(repo, nil)

		res1, err := svc.Evaluate(context.Background(), CheckRequest{
			PatientID:       repo.profile.PatientID,
			DiagnosisID:     1,
			MedicineFormIDs: []uuid.UUID{formIDs["ciprofloxacin"], formIDs["ibuprofen"]},
		})
		if err != nil {
			t.Fatalf("Evaluate(A,B) error: %v", err)
		}
		res2, err := svc.Evaluate(context.Background(), CheckRequest{
			PatientID:       repo.profile.PatientID,
			DiagnosisID:     1,
			MedicineFormIDs: []uuid.UUID{formIDs["ibuprofen"], formIDs["ciprofloxacin"]},
		})
		if err != nil {
			t.Fatalf("Evaluate(B,A) error: %v", err)
		}
		if len(res1.Warnings) != 1 || len(res2.Warnings) != 1 || res1.Warnings[0].Code != res2.Warnings[0].Code {
			t.Fatalf("expected identical bidirectional warning, got %#v vs %#v", res1.Warnings, res2.Warnings)
		}
	})

	t.Run("pediatric aspirin and doxycycline rules and non-antibiotic viral diagnosis", func(t *testing.T) {
		repo, _, formIDs := newFixtureRepo()
		doxyMedID := uuid.New()
		doxyFormID := uuid.New()
		repo.forms[doxyFormID] = MedicineInfo{
			FormID:       doxyFormID,
			MedicineID:   doxyMedID,
			INNNameEN:    "Doxycycline",
			ATCCode:      "J01AA02",
			IsAntibiotic: true,
		}
		repo.profile.DateOfBirth = time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
		repo.diagnosis = &DiagnosisInfo{ID: 3, Code: "J00", NameRU: "Ринит"}
		svc := NewService(repo, nil)
		svc.nowFn = func() time.Time { return time.Date(2026, 11, 27, 0, 0, 0, 0, time.UTC) }

		// Child (6 years) with Aspirin + Doxycycline triggers both pediatric critical blocks + antibiotic stewardship.
		res, err := svc.Evaluate(context.Background(), CheckRequest{
			PatientID:       repo.profile.PatientID,
			DiagnosisID:     3,
			MedicineFormIDs: []uuid.UUID{formIDs["aspirin"], doxyFormID},
		})
		if err != nil {
			t.Fatalf("Evaluate error: %v", err)
		}
		if res.Allowed || len(res.Warnings) < 3 {
			t.Fatalf("expected critical blocks + stewardship warning, got %#v", res)
		}

		// Non-antibiotic (Paracetamol) on viral J00 should be allowed with zero warnings.
		safeRes, err := svc.Evaluate(context.Background(), CheckRequest{
			PatientID:       repo.profile.PatientID,
			DiagnosisID:     3,
			MedicineFormIDs: []uuid.UUID{formIDs["paracetamol"]},
		})
		if err != nil {
			t.Fatalf("Evaluate paracetamol error: %v", err)
		}
		if !safeRes.Allowed || len(safeRes.Warnings) != 0 {
			t.Fatalf("expected Allowed=true and 0 warnings for paracetamol on J00, got %#v", safeRes)
		}
	})

	t.Run("validation and lookup error cases", func(t *testing.T) {
		repo, _, formIDs := newFixtureRepo()
		svc := NewService(repo, nil)

		invalidCases := []CheckRequest{
			{PatientID: uuid.Nil, DiagnosisID: 1, MedicineFormIDs: []uuid.UUID{formIDs["paracetamol"]}},
			{PatientID: uuid.New(), DiagnosisID: 0, MedicineFormIDs: []uuid.UUID{formIDs["paracetamol"]}},
			{PatientID: uuid.New(), DiagnosisID: 1, MedicineFormIDs: nil},
			{PatientID: uuid.New(), DiagnosisID: 1, MedicineFormIDs: []uuid.UUID{uuid.Nil}},
		}
		for i, tc := range invalidCases {
			if _, err := svc.Evaluate(context.Background(), tc); !errors.Is(err, ErrInvalidRequest) {
				t.Fatalf("case %d: expected ErrInvalidRequest, got %v", i, err)
			}
		}

		if _, err := svc.Evaluate(context.Background(), CheckRequest{
			PatientID:       uuid.New(),
			DiagnosisID:     1,
			MedicineFormIDs: []uuid.UUID{uuid.New()},
		}); !errors.Is(err, ErrMedicineFormNotFound) {
			t.Fatalf("expected ErrMedicineFormNotFound, got %v", err)
		}
	})
}

func TestCDSHandlerErrorMappings(t *testing.T) {
	baseRepo, _, formIDs := newFixtureRepo()
	repo := &failingRepo{stubRepo: *baseRepo}
	svc := NewService(repo, nil)
	h := NewHandler(svc, nil)

	router := httpserver.NewRouter(httpserver.RouterDeps{
		RegisterRoutes: func(r chi.Router) {
			r.Use(func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					actor := auth.ActorContext{UserID: uuid.New(), IIN: "840315301294", Role: auth.RoleDoctor}
					next.ServeHTTP(w, r.WithContext(auth.WithActor(r.Context(), actor)))
				})
			})
			h.RegisterRoutes(r)
		},
	})

	send := func(req CheckRequest) int {
		b, _ := json.Marshal(req)
		httpReq := httptest.NewRequest(http.MethodPost, "/api/v1/cds/check", bytes.NewReader(b))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httpReq)
		return rec.Code
	}

	// Missing medicine form -> 404
	if code := send(CheckRequest{PatientID: baseRepo.profile.PatientID, DiagnosisID: 1, MedicineFormIDs: []uuid.UUID{uuid.New()}}); code != http.StatusNotFound {
		t.Fatalf("expected 404 for missing form, got %d", code)
	}

	// Missing diagnosis -> 404
	repo.diagnosis = nil
	if code := send(CheckRequest{PatientID: baseRepo.profile.PatientID, DiagnosisID: 999, MedicineFormIDs: []uuid.UUID{formIDs["paracetamol"]}}); code != http.StatusNotFound {
		t.Fatalf("expected 404 for missing diagnosis, got %d", code)
	}

	// Missing patient -> 404
	repo.profile = nil
	if code := send(CheckRequest{PatientID: uuid.New(), DiagnosisID: 1, MedicineFormIDs: []uuid.UUID{formIDs["paracetamol"]}}); code != http.StatusNotFound {
		t.Fatalf("expected 404 for missing patient, got %d", code)
	}

	// Unexpected DB failure -> 500
	repo.profile = &PatientClinicalProfile{PatientID: uuid.New(), DateOfBirth: time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC)}
	repo.diagnosis = &DiagnosisInfo{ID: 1, Code: "J18.9", NameRU: "Пневмония"}
	repo.resolveErr = errors.New("db connection dropped")
	if code := send(CheckRequest{PatientID: uuid.New(), DiagnosisID: 1, MedicineFormIDs: []uuid.UUID{formIDs["paracetamol"]}}); code != http.StatusInternalServerError {
		t.Fatalf("expected 500 for unexpected repo error, got %d", code)
	}
}
