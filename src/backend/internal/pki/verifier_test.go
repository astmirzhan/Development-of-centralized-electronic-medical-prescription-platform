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

func TestMockVerifierSecurityInvariants(t *testing.T) {
	if _, err := NewMockVerifier("production", true, "secret"); !errors.Is(err, ErrTestModeForbiddenInProd) {
		t.Fatalf("expected ErrTestModeForbiddenInProd in production, got %v", err)
	}

	v, err := NewMockVerifier("development", true, "secret-key")
	if err != nil {
		t.Fatalf("NewMockVerifier error: %v", err)
	}

	canonical := []byte(`{"prescription_id":"rx-1","dosage_value":"500.00"}`)
	env, err := CreateTestEnvelope("secret-key", "840315301294", "SERIAL-001", canonical)
	if err != nil {
		t.Fatalf("CreateTestEnvelope error: %v", err)
	}

	res, err := v.VerifyCMS(context.Background(), env, canonical, "840315301294")
	if err != nil {
		t.Fatalf("VerifyCMS expected success, got %v", err)
	}
	if !res.Valid || res.SignerIIN != "840315301294" || res.OCSPStatus != "GOOD" {
		t.Fatalf("unexpected verification result: %#v", res)
	}

	// Tampered canonical bytes must fail.
	tampered := []byte(`{"prescription_id":"rx-1","dosage_value":"500.01"}`)
	if _, err := v.VerifyCMS(context.Background(), env, tampered, "840315301294"); !errors.Is(err, ErrPayloadMismatch) {
		t.Fatalf("expected ErrPayloadMismatch, got %v", err)
	}

	// Signer IIN mismatch must fail.
	if _, err := v.VerifyCMS(context.Background(), env, canonical, "999999999999"); !errors.Is(err, ErrSignerIINMismatch) {
		t.Fatalf("expected ErrSignerIINMismatch, got %v", err)
	}

	// Revoked certificate must fail.
	revokedEnv, _ := CreateTestEnvelopeWithOCSP("secret-key", "840315301294", "SERIAL-001", "REVOKED", canonical)
	if _, err := v.VerifyCMS(context.Background(), revokedEnv, canonical, "840315301294"); !errors.Is(err, ErrCertificateRevoked) {
		t.Fatalf("expected ErrCertificateRevoked, got %v", err)
	}
}

func TestNCANodeVerifierHTTP(t *testing.T) {
	canonical := []byte(`{"prescription_id":"rx-1"}`)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/cms/verify" || r.Method != http.MethodPost {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		resp := ncaVerifyResponse{
			Valid: true,
			Data:  base64.StdEncoding.EncodeToString(canonical),
			Signers: []ncaSigner{{
				Certificates: []ncaCertificate{{
					Valid:        true,
					SerialNumber: "CERT-ABC",
					Subject:      ncaSubject{IIN: "840315301294", CommonName: "KASSYMOV AIBEK"},
					Revocations:  []ncaRevocation{{Revoked: false, Status: "GOOD"}},
				}},
			}},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	client := NewNCANodeVerifier(srv.URL, srv.Client())
	res, err := client.VerifyCMS(context.Background(), "ZmFrZUNNUw==", canonical, "840315301294")
	if err != nil {
		t.Fatalf("VerifyCMS error: %v", err)
	}
	if res.CertSerialNumber != "CERT-ABC" || res.OCSPStatus != "GOOD" {
		t.Fatalf("unexpected NCANode result: %#v", res)
	}
}
