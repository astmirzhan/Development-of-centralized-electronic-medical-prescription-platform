package pki

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// NCANodeVerifier validates GOST/RSA CMS signatures via a self-hosted NCANode REST service.
type NCANodeVerifier struct {
	baseURL    string
	httpClient *http.Client
}

// NewNCANodeVerifier creates an NCANode HTTP verifier client.
func NewNCANodeVerifier(baseURL string, httpClient *http.Client) *NCANodeVerifier {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	return &NCANodeVerifier{
		baseURL:    strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		httpClient: httpClient,
	}
}

type ncaVerifyRequest struct {
	RevocationCheck []string `json:"revocationCheck"`
	CMS             string   `json:"cms"`
	Data            string   `json:"data,omitempty"`
}

type ncaVerifyResponse struct {
	Valid   bool        `json:"valid"`
	Data    string      `json:"data"`
	Message string      `json:"message"`
	Status  int         `json:"status"`
	Signers []ncaSigner `json:"signers"`
}

type ncaSigner struct {
	Certificates []ncaCertificate `json:"certificates"`
	// Some NCANode versions expose flat certificate/subject fields on signer.
	Subject      *ncaSubject `json:"subject"`
	SerialNumber string      `json:"serialNumber"`
	OCSPStatus   string      `json:"ocspStatus"`
}

type ncaCertificate struct {
	Valid        bool            `json:"valid"`
	SerialNumber string          `json:"serialNumber"`
	Subject      ncaSubject      `json:"subject"`
	Revocations  []ncaRevocation `json:"revocations"`
}

type ncaSubject struct {
	IIN        string `json:"iin"`
	CommonName string `json:"commonName"`
}

type ncaRevocation struct {
	RevocationReason string `json:"revocationReason"`
	Revoked          bool   `json:"revoked"`
	Status           string `json:"status"`
	By               string `json:"by"`
}

// VerifyCMS posts the CMS signature to NCANode /cms/verify and enforces all 4 security invariants.
func (v *NCANodeVerifier) VerifyCMS(ctx context.Context, cmsBase64 string, expectedCanonical []byte, expectedSignerIIN string) (*VerificationResult, error) {
	if strings.TrimSpace(cmsBase64) == "" {
		return nil, fmt.Errorf("%w: empty CMS payload", ErrInvalidSignature)
	}
	if len(expectedCanonical) == 0 {
		return nil, fmt.Errorf("%w: empty canonical data", ErrPayloadMismatch)
	}

	reqBody := ncaVerifyRequest{
		RevocationCheck: []string{"OCSP"},
		CMS:             strings.TrimSpace(cmsBase64),
		Data:            base64.StdEncoding.EncodeToString(expectedCanonical),
	}
	rawReq, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal ncanode request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, v.baseURL+"/cms/verify", bytes.NewReader(rawReq))
	if err != nil {
		return nil, fmt.Errorf("create ncanode request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := v.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrVerifierUnavailable, err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, fmt.Errorf("%w: read response: %v", ErrVerifierUnavailable, err)
	}

	if resp.StatusCode >= http.StatusInternalServerError {
		return nil, fmt.Errorf("%w: status %d", ErrVerifierUnavailable, resp.StatusCode)
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("%w: status %d: %s", ErrInvalidSignature, resp.StatusCode, strings.TrimSpace(string(respBytes)))
	}

	var parsed ncaVerifyResponse
	if err := json.Unmarshal(respBytes, &parsed); err != nil {
		return nil, fmt.Errorf("decode ncanode response: %w", err)
	}

	// Check (a): Cryptographic validity.
	if !parsed.Valid || len(parsed.Signers) == 0 {
		return nil, fmt.Errorf("%w: %s", ErrInvalidSignature, parsed.Message)
	}

	// Check (d): Signed payload matches canonical record when NCANode returns embedded data.
	if strings.TrimSpace(parsed.Data) != "" {
		decodedData, err := base64.StdEncoding.DecodeString(parsed.Data)
		if err != nil {
			return nil, fmt.Errorf("%w: invalid base64 data from NCANode", ErrPayloadMismatch)
		}
		if subtle.ConstantTimeCompare(decodedData, expectedCanonical) != 1 {
			return nil, ErrPayloadMismatch
		}
	}

	signerIIN, serialNumber, ocspStatus, certValid, revoked := extractSignerDetails(parsed.Signers[0])
	if !certValid {
		return nil, fmt.Errorf("%w: signer certificate invalid", ErrInvalidSignature)
	}
	// Check (b): OCSP status must not be revoked.
	if revoked || strings.EqualFold(ocspStatus, "REVOKED") {
		return nil, fmt.Errorf("%w: ocsp=%s", ErrCertificateRevoked, ocspStatus)
	}
	// Check (c): Signer IIN must match expected actor IIN.
	if strings.TrimSpace(expectedSignerIIN) != "" && signerIIN != strings.TrimSpace(expectedSignerIIN) {
		return nil, fmt.Errorf("%w: got %s, expected %s", ErrSignerIINMismatch, signerIIN, expectedSignerIIN)
	}

	return &VerificationResult{
		Valid:            true,
		SignerIIN:        signerIIN,
		CertSerialNumber: serialNumber,
		OCSPStatus:       ocspStatus,
		SignedDataHash:   SHA256Hex(expectedCanonical),
		RawSignature:     cmsBase64,
	}, nil
}

func extractSignerDetails(s ncaSigner) (iin string, serial string, ocspStatus string, certValid bool, revoked bool) {
	ocspStatus = "GOOD"
	certValid = true

	if len(s.Certificates) > 0 {
		cert := s.Certificates[0]
		iin = strings.TrimSpace(cert.Subject.IIN)
		serial = strings.TrimSpace(cert.SerialNumber)
		certValid = cert.Valid
		for _, rev := range cert.Revocations {
			if rev.Revoked || strings.EqualFold(rev.Status, "REVOKED") {
				revoked = true
				ocspStatus = "REVOKED"
				break
			}
			if rev.Status != "" {
				ocspStatus = strings.ToUpper(rev.Status)
			}
		}
		return iin, serial, ocspStatus, certValid, revoked
	}

	if s.Subject != nil {
		iin = strings.TrimSpace(s.Subject.IIN)
	}
	serial = strings.TrimSpace(s.SerialNumber)
	if s.OCSPStatus != "" {
		ocspStatus = strings.ToUpper(strings.TrimSpace(s.OCSPStatus))
		if ocspStatus == "REVOKED" {
			revoked = true
		}
	}
	return iin, serial, ocspStatus, certValid, revoked
}
