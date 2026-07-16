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

// Authenticate extracts and verifies the Bearer token from the Authorization header.
// Returns nil, false if the header is missing/malformed/invalid - callers respond 403.
func Authenticate(r *http.Request) (*Claims, bool) {
	header := r.Header.Get("Authorization")
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
