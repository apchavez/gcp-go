package auth_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/apchavez/gcp-go/internal/infrastructure/auth"
)

const testSecret = "test-only-secret-do-not-use-in-production"

func TestSignAndVerify_RoundTrip(t *testing.T) {
	token := auth.Sign("00001", "insured", testSecret, time.Hour)

	claims, err := auth.Verify(token, testSecret)

	require.NoError(t, err)
	assert.Equal(t, "00001", claims.Sub)
	assert.Equal(t, "insured", claims.Role)
}

func TestVerify_TamperedSignature(t *testing.T) {
	token := auth.Sign("00001", "insured", testSecret, time.Hour)
	tampered := token[:len(token)-4] + "abcd"

	_, err := auth.Verify(tampered, testSecret)

	assert.ErrorIs(t, err, auth.ErrInvalidSignature)
}

func TestVerify_WrongSecret(t *testing.T) {
	token := auth.Sign("00001", "insured", testSecret, time.Hour)

	_, err := auth.Verify(token, "wrong-secret")

	assert.ErrorIs(t, err, auth.ErrInvalidSignature)
}

func TestVerify_Expired(t *testing.T) {
	token := auth.Sign("00001", "insured", testSecret, -time.Hour)

	_, err := auth.Verify(token, testSecret)

	assert.ErrorIs(t, err, auth.ErrTokenExpired)
}

func TestVerify_MalformedToken(t *testing.T) {
	_, err := auth.Verify("not-a-jwt", testSecret)

	assert.ErrorIs(t, err, auth.ErrMalformedToken)
}

func TestVerify_MalformedBase64(t *testing.T) {
	_, err := auth.Verify("!!!.!!!.!!!", testSecret)

	assert.Error(t, err)
}
