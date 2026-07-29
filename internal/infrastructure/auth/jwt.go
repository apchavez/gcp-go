// Package auth implements a hand-rolled HS256 JWT verifier/signer, matching the exact
// same algorithm and claim shape as the AWS TypeScript sibling's infra/jwt.ts and the
// Azure Python sibling's infrastructure/auth/jwt_validator.py - no JWT library, so the
// three clinic-scheduling siblings share one hand-audited implementation of the same logic.
package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

type Claims struct {
	Sub string `json:"sub"`
	Role string `json:"role"`
	Iat int64  `json:"iat"`
	Exp int64  `json:"exp"`
}

var (
	ErrMalformedToken    = errors.New("malformed JWT")
	ErrInvalidSignature  = errors.New("invalid signature")
	ErrTokenExpired      = errors.New("token expired")
)

func base64urlEncode(b []byte) string {
	return base64.RawURLEncoding.EncodeToString(b)
}

func base64urlDecode(s string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(s)
}

// Verify checks signature and exp, then returns the parsed claims.
func Verify(token, secret string) (*Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, ErrMalformedToken
	}
	headerB64, payloadB64, sigB64 := parts[0], parts[1], parts[2]

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(headerB64 + "." + payloadB64))
	expectedSig := base64urlEncode(mac.Sum(nil))

	// Constant-time comparison prevents timing-based signature oracle attacks.
	if subtle.ConstantTimeCompare([]byte(sigB64), []byte(expectedSig)) != 1 {
		return nil, ErrInvalidSignature
	}

	payloadJSON, err := base64urlDecode(payloadB64)
	if err != nil {
		return nil, ErrMalformedToken
	}
	var claims Claims
	if err := json.Unmarshal(payloadJSON, &claims); err != nil {
		return nil, ErrMalformedToken
	}
	if time.Now().Unix() > claims.Exp {
		return nil, ErrTokenExpired
	}
	return &claims, nil
}

// Sign issues a new HS256 token - used only by tests and the local token-generation script,
// mirroring signJwt in the AWS sibling.
func Sign(sub, role, secret string, expiresIn time.Duration) string {
	now := time.Now().Unix()
	header, _ := json.Marshal(map[string]string{"alg": "HS256", "typ": "JWT"})
	payload, _ := json.Marshal(Claims{Sub: sub, Role: role, Iat: now, Exp: now + int64(expiresIn.Seconds())})

	headerB64 := base64urlEncode(header)
	payloadB64 := base64urlEncode(payload)

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(headerB64 + "." + payloadB64))
	sig := base64urlEncode(mac.Sum(nil))

	return headerB64 + "." + payloadB64 + "." + sig
}
