package qr

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/xjam-byte/erx-platform/backend/internal/auth"
	"github.com/xjam-byte/erx-platform/backend/internal/httpserver"
)

type stubQRRepo struct {
	eligibility *ItemEligibility
	stored      []*QRToken
}

func (s *stubQRRepo) GetItemEligibility(_ context.Context, id uuid.UUID) (*ItemEligibility, error) {
	if s.eligibility == nil || s.eligibility.PrescriptionItemID != id {
		return nil, ErrPrescriptionItemNotFound
	}
	return s.eligibility, nil
}

func (s *stubQRRepo) InsertToken(_ context.Context, itemID uuid.UUID, tokenHash, nonce string, expiresAt time.Time) (*QRToken, error) {
	tok := &QRToken{
		ID:                 uuid.New(),
		PrescriptionItemID: itemID,
		TokenHash:          tokenHash,
		Nonce:              nonce,
		ExpiresAt:          expiresAt,
		IsConsumed:         false,
		CreatedAt:          time.Now().UTC(),
	}
	s.stored = append(s.stored, tok)
	return tok, nil
}

func (s *stubQRRepo) FindAndLockForRedemption(_ context.Context, _ pgx.Tx, _ string) (*QRToken, error) {
	return nil, ErrTokenNotFound
}

func (s *stubQRRepo) MarkConsumedTx(_ context.Context, _ pgx.Tx, _ uuid.UUID, _ uuid.UUID) error {
	return nil
}

func TestIssueQRTokenSecurityAndEligibility(t *testing.T) {
	patientID := uuid.New()
	itemID := uuid.New()
	repo := &stubQRRepo{
		eligibility: &ItemEligibility{
			PrescriptionItemID: itemID,
			PrescriptionID:     uuid.New(),
			PatientID:          patientID,
			PrescriptionStatus: "active",
			ValidUntil:         time.Now().UTC().Add(24 * time.Hour),
			ItemStatus:         "pending",
			QuantityPrescribed: 20,
			QuantityDispensed:  0,
		},
	}

	svc := NewService(repo, 60*time.Second, 90*time.Second)

	resp, err := svc.IssueToken(context.Background(), itemID, patientID)
	if err != nil {
		t.Fatalf("IssueToken error: %v", err)
	}
	if !strings.HasPrefix(resp.Token, "erx_") || resp.RotateAfterSeconds != 60 || resp.AcceptWindowSeconds != 90 {
		t.Fatalf("unexpected QR response: %#v", resp)
	}
	if len(repo.stored) != 1 {
		t.Fatalf("expected 1 stored token, got %d", len(repo.stored))
	}
	stored := repo.stored[0]
	if stored.TokenHash == resp.Token || stored.TokenHash != HashToken(resp.Token) {
		t.Fatalf("expected DB to store SHA-256 hash, never raw token")
	}

	// Wrong patient must be rejected.
	if _, err := svc.IssueToken(context.Background(), itemID, uuid.New()); !errors.Is(err, ErrItemNotOwnedByPatient) {
		t.Fatalf("expected ErrItemNotOwnedByPatient, got %v", err)
	}

	// Expired prescription must be rejected.
	repo.eligibility.ValidUntil = time.Now().UTC().Add(-1 * time.Minute)
	if _, err := svc.IssueToken(context.Background(), itemID, patientID); !errors.Is(err, ErrItemNotEligibleForQR) {
		t.Fatalf("expected ErrItemNotEligibleForQR for expired prescription, got %v", err)
	}
}

func TestQRHTTPHandler(t *testing.T) {
	patientID := uuid.New()
	itemID := uuid.New()
	repo := &stubQRRepo{
		eligibility: &ItemEligibility{
			PrescriptionItemID: itemID,
			PrescriptionID:     uuid.New(),
			PatientID:          patientID,
			PrescriptionStatus: "partially_dispensed",
			ValidUntil:         time.Now().UTC().Add(24 * time.Hour),
			ItemStatus:         "partially_dispensed",
			QuantityPrescribed: 20,
			QuantityDispensed:  10,
		},
	}

	h := NewHandler(NewService(repo, 60*time.Second, 90*time.Second), nil)
	actor := auth.ActorContext{UserID: patientID, IIN: "981105303419", Role: auth.RolePatient}

	router := httpserver.NewRouter(httpserver.RouterDeps{
		RegisterRoutes: func(r chi.Router) {
			r.Use(func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					next.ServeHTTP(w, r.WithContext(auth.WithActor(r.Context(), actor)))
				})
			})
			h.RegisterRoutes(r)
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/prescription-items/"+itemID.String()+"/qr", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}

	var out IssuedTokenResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if out.Token == "" || out.AcceptWindowSeconds != 90 {
		t.Fatalf("unexpected JSON response: %#v", out)
	}
}
