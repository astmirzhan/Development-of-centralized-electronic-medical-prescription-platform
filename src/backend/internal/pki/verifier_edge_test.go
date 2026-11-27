package pki

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNCANodeVerifierErrorAndFlatSignerCases(t *testing.T) {
	canonical := []byte(`{"prescription_id":"rx-1"}`)

	t.Run("revoked certificate in flat signer format", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			resp := ncaVerifyResponse{
				Valid: true,
				Data:  base64.StdEncoding.EncodeToString(canonical),
				Signers: []ncaSigner{{
					Subject:      &ncaSubject{IIN: "840315301294"},
					SerialNumber: "FLAT-001",
					OCSPStatus:   "REVOKED",
				}},
			}
			_ = json.NewEncoder(w).Encode(resp)
		}))
		defer srv.Close()

		c := NewNCANodeVerifier(srv.URL, srv.Client())
		if _, err := c.VerifyCMS(context.Background(), "ZmFrZQ==", canonical, "840315301294"); !errors.Is(err, ErrCertificateRevoked) {
			t.Fatalf("expected ErrCertificateRevoked, got %v", err)
		}
	})

	t.Run("payload mismatch from NCANode", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			resp := ncaVerifyResponse{
				Valid: true,
				Data:  base64.StdEncoding.EncodeToString([]byte(`{"prescription_id":"tampered"}`)),
				Signers: []ncaSigner{{
					Subject:      &ncaSubject{IIN: "840315301294"},
					SerialNumber: "FLAT-001",
					OCSPStatus:   "GOOD",
				}},
			}
			_ = json.NewEncoder(w).Encode(resp)
		}))
		defer srv.Close()

		c := NewNCANodeVerifier(srv.URL, srv.Client())
		if _, err := c.VerifyCMS(context.Background(), "ZmFrZQ==", canonical, "840315301294"); !errors.Is(err, ErrPayloadMismatch) {
			t.Fatalf("expected ErrPayloadMismatch, got %v", err)
		}
	})

	t.Run("IIN mismatch from NCANode", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			resp := ncaVerifyResponse{
				Valid: true,
				Signers: []ncaSigner{{
					Subject:      &ncaSubject{IIN: "111111111111"},
					SerialNumber: "FLAT-001",
					OCSPStatus:   "GOOD",
				}},
			}
			_ = json.NewEncoder(w).Encode(resp)
		}))
		defer srv.Close()

		c := NewNCANodeVerifier(srv.URL, srv.Client())
		if _, err := c.VerifyCMS(context.Background(), "ZmFrZQ==", canonical, "840315301294"); !errors.Is(err, ErrSignerIINMismatch) {
			t.Fatalf("expected ErrSignerIINMismatch, got %v", err)
		}
	})

	t.Run("HTTP 500 returns ErrVerifierUnavailable and HTTP 400 returns ErrInvalidSignature", func(t *testing.T) {
		status := http.StatusInternalServerError
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(status)
		}))
		defer srv.Close()

		c := NewNCANodeVerifier(srv.URL, srv.Client())
		if _, err := c.VerifyCMS(context.Background(), "ZmFrZQ==", canonical, "840315301294"); !errors.Is(err, ErrVerifierUnavailable) {
			t.Fatalf("expected ErrVerifierUnavailable, got %v", err)
		}

		status = http.StatusBadRequest
		if _, err := c.VerifyCMS(context.Background(), "ZmFrZQ==", canonical, "840315301294"); !errors.Is(err, ErrInvalidSignature) {
			t.Fatalf("expected ErrInvalidSignature, got %v", err)
		}
	})
}

func TestMockVerifierEdgeCases(t *testing.T) {
	v, err := NewMockVerifier("development", true, "")
	if err != nil {
		t.Fatalf("NewMockVerifier error: %v", err)
	}

	if _, err := v.VerifyCMS(context.Background(), "", []byte("x"), "840315301294"); !errors.Is(err, ErrInvalidSignature) {
		t.Fatalf("expected ErrInvalidSignature for empty CMS, got %v", err)
	}
	if _, err := v.VerifyCMS(context.Background(), "ZmFrZQ==", nil, "840315301294"); !errors.Is(err, ErrPayloadMismatch) {
		t.Fatalf("expected ErrPayloadMismatch for empty canonical, got %v", err)
	}
	if _, err := v.VerifyCMS(context.Background(), "!!!not-base64!!!", []byte("x"), "840315301294"); !errors.Is(err, ErrInvalidSignature) {
		t.Fatalf("expected ErrInvalidSignature for bad base64, got %v", err)
	}
	badJSON := base64.StdEncoding.EncodeToString([]byte(`{bad json`))
	if _, err := v.VerifyCMS(context.Background(), badJSON, []byte("x"), "840315301294"); !errors.Is(err, ErrInvalidSignature) {
		t.Fatalf("expected ErrInvalidSignature for bad JSON, got %v", err)
	}
}
