package auth

import (
	"net/http"
	"os"
	"strings"
	"sync"
)

var (
	secretOnce sync.Once
	secret     string
)

func jwtSecret() string {
	secretOnce.Do(func() {
		secret = os.Getenv("JWT_SECRET")
		if secret == "" {
			panic("JWT_SECRET is not defined")
		}
	})
	return secret
}

// Authenticate extracts and verifies the caller's Bearer token, checking
// X-Forwarded-Authorization before Authorization.
//
// When this app is called through the GCP API Gateway in front of it (terraform/api_gateway.tf),
// ESPv2 always authenticates itself to the Cloud Run backend using its own Google-signed ID
// token in the Authorization header - this happens unconditionally for any Cloud Run/Cloud
// Functions backend, regardless of the backend's IAM invoker policy, and isn't something the
// gateway config can disable. The caller's original bearer token is preserved, but moved to
// X-Forwarded-Authorization instead. Direct calls to this service (tests, Postman, CI) never
// go through the gateway, so they never get an X-Forwarded-Authorization and fall back to
// Authorization as before. Found 2026-07-27 during interview-demo E2E verification: every
// gateway-routed request failed HS256 verification against the injected RS256 Google ID token
// and returned a generic 403 "Access denied" even with a valid, unexpired client token.
func Authenticate(r *http.Request) (*Claims, bool) {
	header := r.Header.Get("X-Forwarded-Authorization")
	if header == "" {
		header = r.Header.Get("Authorization")
	}
	if !strings.HasPrefix(header, "Bearer ") {
		return nil, false
	}
	token := strings.TrimPrefix(header, "Bearer ")
	claims, err := Verify(token, jwtSecret())
	if err != nil {
		return nil, false
	}
	return claims, true
}
