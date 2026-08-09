package auth_test

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/apchavez/gcp-go/internal/infrastructure/auth"
)

func TestAuthenticate_UsesAuthorizationHeader(t *testing.T) {
	t.Setenv("JWT_SECRET", testSecret)
	token := auth.Sign("00001", "insured", testSecret, time.Hour)

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	claims, ok := auth.Authenticate(req)

	require.True(t, ok)
	assert.Equal(t, "00001", claims.Sub)
}

func TestAuthenticate_PrefersXForwardedAuthorizationOverAuthorization(t *testing.T) {
	t.Setenv("JWT_SECRET", testSecret)
	forwardedToken := auth.Sign("00002", "insured", testSecret, time.Hour)
	// A different, unrelated Authorization header (simulating ESPv2's own Google-signed ID
	// token injected in front of Cloud Run) must be ignored in favor of X-Forwarded-Authorization.
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer not-a-real-token")
	req.Header.Set("X-Forwarded-Authorization", "Bearer "+forwardedToken)

	claims, ok := auth.Authenticate(req)

	require.True(t, ok)
	assert.Equal(t, "00002", claims.Sub)
}

func TestAuthenticate_MissingHeader(t *testing.T) {
	t.Setenv("JWT_SECRET", testSecret)
	req := httptest.NewRequest("GET", "/", nil)

	_, ok := auth.Authenticate(req)

	assert.False(t, ok)
}

func TestAuthenticate_MalformedHeaderMissingBearerPrefix(t *testing.T) {
	t.Setenv("JWT_SECRET", testSecret)
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "not-bearer-scheme")

	_, ok := auth.Authenticate(req)

	assert.False(t, ok)
}

func TestAuthenticate_InvalidToken(t *testing.T) {
	t.Setenv("JWT_SECRET", testSecret)
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer not-a-jwt")

	_, ok := auth.Authenticate(req)

	assert.False(t, ok)
}
