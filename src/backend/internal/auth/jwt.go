package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
)

// TokenClaims represents verified HS256 JWT claims for an authenticated actor.
type TokenClaims struct {
	Sub  string `json:"sub"`
	IIN  string `json:"iin"`
	Role string `json:"role"`
	Iat  int64  `json:"iat"`
	Nbf  int64  `json:"nbf,omitempty"`
	Exp  int64  `json:"exp"`
}

// ErrInvalidToken indicates a malformed, expired, or unverified JWT access token.
var ErrInvalidToken = errors.New("invalid or expired access token")

type jwtHeader struct {
	Alg string `json:"alg"`
	Typ string `json:"typ"`
}

// SignAccessToken creates an HS256 JWT encoding actor UserID, IIN, and Role.
func SignAccessToken(signingKey string, actor ActorContext, ttl time.Duration) (string, error) {
	if strings.TrimSpace(signingKey) == "" {
		return "", errors.New("jwt signing key is required")
	}
	if actor.UserID == uuid.Nil || strings.TrimSpace(actor.IIN) == "" || strings.TrimSpace(actor.Role) == "" {
		return "", errors.New("actor user_id, iin, and role are required")
	}
	if ttl <= 0 {
		return "", errors.New("token ttl must be positive")
	}

	now := time.Now().UTC()
	hdrJSON, err := json.Marshal(jwtHeader{Alg: "HS256", Typ: "JWT"})
	if err != nil {
		return "", err
	}
	claimsJSON, err := json.Marshal(TokenClaims{
		Sub:  actor.UserID.String(),
		IIN:  strings.TrimSpace(actor.IIN),
		Role: strings.TrimSpace(actor.Role),
		Iat:  now.Unix(),
		Nbf:  now.Unix(),
		Exp:  now.Add(ttl).Unix(),
	})
	if err != nil {
		return "", err
	}

	signingInput := base64.RawURLEncoding.EncodeToString(hdrJSON) + "." + base64.RawURLEncoding.EncodeToString(claimsJSON)
	mac := hmac.New(sha256.New, []byte(signingKey))
	mac.Write([]byte(signingInput))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return signingInput + "." + sig, nil
}

// ParseAccessToken verifies an HS256 JWT and returns the authenticated ActorContext.
func ParseAccessToken(signingKey, rawToken string, now time.Time) (ActorContext, error) {
	if strings.TrimSpace(signingKey) == "" {
		return ActorContext{}, ErrInvalidToken
	}

	parts := strings.Split(strings.TrimSpace(rawToken), ".")
	if len(parts) != 3 {
		return ActorContext{}, ErrInvalidToken
	}

	hdrBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return ActorContext{}, ErrInvalidToken
	}
	var hdr jwtHeader
	if err := json.Unmarshal(hdrBytes, &hdr); err != nil || hdr.Alg != "HS256" || (hdr.Typ != "" && !strings.EqualFold(hdr.Typ, "JWT")) {
		return ActorContext{}, ErrInvalidToken
	}

	signingInput := parts[0] + "." + parts[1]
	sigBytes, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return ActorContext{}, ErrInvalidToken
	}
	mac := hmac.New(sha256.New, []byte(signingKey))
	mac.Write([]byte(signingInput))
	if !hmac.Equal(sigBytes, mac.Sum(nil)) {
		return ActorContext{}, ErrInvalidToken
	}

	claimsBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return ActorContext{}, ErrInvalidToken
	}
	var claims TokenClaims
	if err := json.Unmarshal(claimsBytes, &claims); err != nil {
		return ActorContext{}, ErrInvalidToken
	}

	nowUnix := now.UTC().Unix()
	if claims.Exp <= nowUnix {
		return ActorContext{}, ErrInvalidToken
	}
	if claims.Nbf != 0 && claims.Nbf > nowUnix {
		return ActorContext{}, ErrInvalidToken
	}

	userID, err := uuid.Parse(strings.TrimSpace(claims.Sub))
	if err != nil || userID == uuid.Nil {
		return ActorContext{}, ErrInvalidToken
	}
	iin := strings.TrimSpace(claims.IIN)
	role := strings.TrimSpace(claims.Role)
	if iin == "" || role == "" {
		return ActorContext{}, ErrInvalidToken
	}

	return ActorContext{
		UserID: userID,
		IIN:    iin,
		Role:   role,
	}, nil
}

// JWTMiddleware authenticates requests bearing a valid Authorization: Bearer <jwt> header
// and attaches ActorContext to the request context.
func JWTMiddleware(signingKey string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
			const prefix = "Bearer "
			if len(authHeader) <= len(prefix) || !strings.EqualFold(authHeader[:len(prefix)], prefix) {
				writeAuthError(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing or malformed bearer token")
				return
			}

			rawToken := strings.TrimSpace(authHeader[len(prefix):])
			actor, err := ParseAccessToken(signingKey, rawToken, time.Now().UTC())
			if err != nil {
				writeAuthError(w, http.StatusUnauthorized, "UNAUTHORIZED", "invalid or expired access token")
				return
			}

			next.ServeHTTP(w, r.WithContext(WithActor(r.Context(), actor)))
		})
	}
}

// RequireRole enforces that the authenticated actor holds one of the allowed roles.
func RequireRole(roles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			actor, ok := ActorFromContext(r.Context())
			if !ok {
				writeAuthError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authenticated actor context is required")
				return
			}
			if !slices.Contains(roles, actor.Role) {
				writeAuthError(w, http.StatusForbidden, "FORBIDDEN", fmt.Sprintf("role %q is not permitted for this endpoint", actor.Role))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func writeAuthError(w http.ResponseWriter, status int, code, detail string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status": status,
		"code":   code,
		"title":  http.StatusText(status),
		"detail": detail,
	})
}
