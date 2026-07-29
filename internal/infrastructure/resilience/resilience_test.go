package resilience_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/apchavez/gcp-go/internal/infrastructure/resilience"
)

func TestWithRetry_SucceedsWithoutRetryingOnFirstSuccess(t *testing.T) {
	calls := 0
	err := resilience.WithRetry(context.Background(), func() error {
		calls++
		return nil
	})

	require.NoError(t, err)
	assert.Equal(t, 1, calls)
}

func TestWithRetry_RetriesUpToThreeAttempts(t *testing.T) {
	calls := 0
	failErr := errors.New("boom")
	err := resilience.WithRetry(context.Background(), func() error {
		calls++
		return failErr
	})

	assert.ErrorIs(t, err, failErr)
	assert.Equal(t, 3, calls)
}

func TestWithRetry_SucceedsOnSecondAttempt(t *testing.T) {
	calls := 0
	err := resilience.WithRetry(context.Background(), func() error {
		calls++
		if calls < 2 {
			return errors.New("transient")
		}
		return nil
	})

	require.NoError(t, err)
	assert.Equal(t, 2, calls)
}

func TestWithRetry_NeverRetriesCircuitOpenError(t *testing.T) {
	calls := 0
	err := resilience.WithRetry(context.Background(), func() error {
		calls++
		return &resilience.CircuitOpenError{Name: "test"}
	})

	var openErr *resilience.CircuitOpenError
	require.ErrorAs(t, err, &openErr)
	assert.Equal(t, 1, calls)
}

func TestCircuitBreaker_OpensAfterFailureThreshold(t *testing.T) {
	cb := resilience.NewCircuitBreaker("test")
	failErr := errors.New("boom")

	// 10-call window, 50% failure threshold: 5 failures out of 10 calls opens it.
	for i := 0; i < 5; i++ {
		_ = cb.Execute(func() error { return failErr })
	}
	for i := 0; i < 5; i++ {
		_ = cb.Execute(func() error { return nil })
	}

	err := cb.Execute(func() error { return nil })

	var openErr *resilience.CircuitOpenError
	assert.ErrorAs(t, err, &openErr)
}

func TestCircuitBreaker_StaysClosedBelowThreshold(t *testing.T) {
	cb := resilience.NewCircuitBreaker("test")
	failErr := errors.New("boom")

	for i := 0; i < 4; i++ {
		_ = cb.Execute(func() error { return failErr })
	}
	for i := 0; i < 6; i++ {
		_ = cb.Execute(func() error { return nil })
	}

	err := cb.Execute(func() error { return nil })

	assert.NoError(t, err)
}

func TestResilience_Run(t *testing.T) {
	r := resilience.New("test-run")
	calls := 0

	err := r.Run(context.Background(), func() error {
		calls++
		return nil
	})

	require.NoError(t, err)
	assert.Equal(t, 1, calls)
}
