package pki

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
)

// Typed PKI verification errors.
var (
	ErrInvalidSignature        = errors.New("EDS cryptographic signature is invalid")
	ErrCertificateRevoked      = errors.New("EDS certificate is revoked or expired")
	ErrSignerIINMismatch       = errors.New("EDS signer IIN does not match expected actor IIN")
	ErrPayloadMismatch         = errors.New("EDS signed payload does not match canonical record")
	ErrVerifierUnavailable     = errors.New("NCANode verification service unavailable")
	ErrTestModeForbiddenInProd = errors.New("mock PKI verifier is forbidden when APP_ENV=production")
)

// VerificationResult summarizes cryptographic, OCSP, and identity checks of a CMS signature.
type VerificationResult struct {
	Valid            bool   `json:"valid"`
	SignerIIN        string `json:"signer_iin"`
	CertSerialNumber string `json:"cert_serial_number"`
	OCSPStatus       string `json:"ocsp_status"`
	SignedDataHash   string `json:"signed_data_hash"`
	RawSignature     string `json:"raw_signature"`
}

// Verifier validates NCA RK CMS signatures against canonical payload bytes and actor IIN.
type Verifier interface {
	VerifyCMS(ctx context.Context, cmsBase64 string, expectedCanonical []byte, expectedSignerIIN string) (*VerificationResult, error)
}

// SHA256Hex computes the lowercase hex SHA-256 digest of data.
func SHA256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
