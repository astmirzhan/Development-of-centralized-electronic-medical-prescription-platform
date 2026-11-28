package pki

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// TestEnvelope is the structured payload accepted by MockVerifier when NCA_TEST_MODE=true.
type TestEnvelope struct {
	SignerIIN  string `json:"signer_iin"`
	CertSerial string `json:"cert_serial"`
	DataHash   string `json:"data_hash"`
	OCSPStatus string `json:"ocsp_status,omitempty"`
	MAC        string `json:"mac"`
}

// MockVerifier provides deterministic, tamper-detecting signature verification for Risk R3
// local development and automated tests when NCA_TEST_MODE=true.
type MockVerifier struct {
	secret []byte
}

// NewMockVerifier constructs a MockVerifier, refusing to initialize outside development/test
// environments or when no explicit secret is supplied.
func NewMockVerifier(appEnv string, testMode bool, secret string) (*MockVerifier, error) {
	normalizedEnv := strings.ToLower(strings.TrimSpace(appEnv))
	if (normalizedEnv != "development" && normalizedEnv != "test") || !testMode {
		return nil, ErrTestModeForbiddenInProd
	}
	if strings.TrimSpace(secret) == "" {
		return nil, errors.New("NCA_MOCK_SECRET is required for mock PKI verifier")
	}
	return &MockVerifier{secret: []byte(secret)}, nil
}

// CreateTestEnvelope generates a signed base64 envelope compatible with MockVerifier.
func CreateTestEnvelope(secret, signerIIN, certSerial string, canonical []byte) (string, error) {
	return CreateTestEnvelopeWithOCSP(secret, signerIIN, certSerial, "GOOD", canonical)
}

// CreateTestEnvelopeWithOCSP generates a signed base64 envelope with an explicit OCSP status.
func CreateTestEnvelopeWithOCSP(secret, signerIIN, certSerial, ocspStatus string, canonical []byte) (string, error) {
	if strings.TrimSpace(secret) == "" {
		return "", errors.New("secret is required to create test envelope")
	}
	if strings.TrimSpace(ocspStatus) == "" {
		ocspStatus = "GOOD"
	}
	dataHash := SHA256Hex(canonical)
	env := TestEnvelope{
		SignerIIN:  strings.TrimSpace(signerIIN),
		CertSerial: strings.TrimSpace(certSerial),
		DataHash:   dataHash,
		OCSPStatus: strings.ToUpper(strings.TrimSpace(ocspStatus)),
	}
	env.MAC = computeEnvelopeMAC([]byte(secret), env.SignerIIN, env.CertSerial, env.DataHash, env.OCSPStatus)
	raw, err := json.Marshal(env)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(raw), nil
}

// VerifyCMS validates the HMAC-signed test envelope, OCSP status, signer IIN, and payload hash.
func (m *MockVerifier) VerifyCMS(_ context.Context, cmsBase64 string, expectedCanonical []byte, expectedSignerIIN string) (*VerificationResult, error) {
	if strings.TrimSpace(cmsBase64) == "" {
		return nil, fmt.Errorf("%w: empty test signature", ErrInvalidSignature)
	}
	if len(expectedCanonical) == 0 {
		return nil, fmt.Errorf("%w: empty canonical data", ErrPayloadMismatch)
	}
	expectedIIN := strings.TrimSpace(expectedSignerIIN)
	if expectedIIN == "" {
		return nil, fmt.Errorf("%w: expected signer IIN is required", ErrSignerIINMismatch)
	}

	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(cmsBase64))
	if err != nil {
		return nil, fmt.Errorf("%w: invalid base64 envelope: %v", ErrInvalidSignature, err)
	}

	var env TestEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, fmt.Errorf("%w: invalid envelope JSON: %v", ErrInvalidSignature, err)
	}

	ocspStatus := strings.ToUpper(strings.TrimSpace(env.OCSPStatus))
	if ocspStatus == "" {
		ocspStatus = "GOOD"
	}

	expectedMAC := computeEnvelopeMAC(m.secret, env.SignerIIN, env.CertSerial, env.DataHash, ocspStatus)
	if !hmac.Equal([]byte(strings.ToLower(strings.TrimSpace(env.MAC))), []byte(expectedMAC)) {
		return nil, fmt.Errorf("%w: MAC mismatch", ErrInvalidSignature)
	}

	if ocspStatus == "REVOKED" {
		return nil, fmt.Errorf("%w: test certificate marked REVOKED", ErrCertificateRevoked)
	}

	expectedHash := SHA256Hex(expectedCanonical)
	if len(env.DataHash) != len(expectedHash) ||
		subtle.ConstantTimeCompare([]byte(strings.ToLower(env.DataHash)), []byte(expectedHash)) != 1 {
		return nil, fmt.Errorf("%w: hash mismatch", ErrPayloadMismatch)
	}

	if env.SignerIIN != expectedIIN {
		return nil, fmt.Errorf("%w: got %s, expected %s", ErrSignerIINMismatch, env.SignerIIN, expectedIIN)
	}

	return &VerificationResult{
		Valid:            true,
		SignerIIN:        env.SignerIIN,
		CertSerialNumber: env.CertSerial,
		OCSPStatus:       ocspStatus,
		SignedDataHash:   expectedHash,
		RawSignature:     cmsBase64,
	}, nil
}

func computeEnvelopeMAC(secret []byte, signerIIN, certSerial, dataHash, ocspStatus string) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(signerIIN))
	mac.Write([]byte("|"))
	mac.Write([]byte(certSerial))
	mac.Write([]byte("|"))
	mac.Write([]byte(dataHash))
	mac.Write([]byte("|"))
	mac.Write([]byte(ocspStatus))
	return hex.EncodeToString(mac.Sum(nil))
}
