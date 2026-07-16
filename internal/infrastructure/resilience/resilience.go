// Package resilience is a hand-rolled retry + circuit breaker, ported line-for-line from
// the AWS TypeScript sibling's shared/resilience.ts (which itself mirrors the Azure sibling's
// original Resilience4j configuration): Retry - 3 attempts, exponential backoff starting at
// 100ms (100 -> 200 -> 400). CircuitBreaker - count-based window of 10 calls, opens at >=50%
// failures, stays open 30s, allows 3 probe calls in half-open state.
package resilience

import (
	"context"
	"fmt"
	"sync"
	"time"
)

const (
	retryAttempts         = 3
	retryBaseDelay        = 100 * time.Millisecond
	cbWindowSize          = 10
	cbFailureRateThreshold = 0.5
	cbOpenDuration        = 30 * time.Second
	cbHalfOpenProbes      = 3
)

type CircuitOpenError struct{ Name string }

func (e *CircuitOpenError) Error() string {
	return fmt.Sprintf("circuit breaker '%s' is open", e.Name)
}

type circuitState int

const (
	stateClosed circuitState = iota
	stateOpen
	stateHalfOpen
)

type CircuitBreaker struct {
	name  string
	mu    sync.Mutex
	state circuitState
	results []bool
	openedAt time.Time
	halfOpenProbesInFlight int
}

func NewCircuitBreaker(name string) *CircuitBreaker {
	return &CircuitBreaker{name: name, state: stateClosed}
}

func (cb *CircuitBreaker) Execute(fn func() error) error {
	cb.mu.Lock()
	if cb.state == stateOpen {
		if time.Since(cb.openedAt) < cbOpenDuration {
			cb.mu.Unlock()
			return &CircuitOpenError{Name: cb.name}
		}
		cb.state = stateHalfOpen
		cb.halfOpenProbesInFlight = 0
	}
	if cb.state == stateHalfOpen {
		if cb.halfOpenProbesInFlight >= cbHalfOpenProbes {
			cb.mu.Unlock()
			return &CircuitOpenError{Name: cb.name}
		}
		cb.halfOpenProbesInFlight++
	}
	cb.mu.Unlock()

	err := fn()
	cb.record(err == nil)
	return err
}

func (cb *CircuitBreaker) record(success bool) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if cb.state == stateHalfOpen {
		if success {
			cb.state = stateClosed
		} else {
			cb.state = stateOpen
			cb.openedAt = time.Now()
		}
		cb.results = nil
		return
	}

	cb.results = append(cb.results, success)
	if len(cb.results) > cbWindowSize {
		cb.results = cb.results[1:]
	}
	if len(cb.results) == cbWindowSize {
		failures := 0
		for _, r := range cb.results {
			if !r {
				failures++
			}
		}
		failureRate := float64(failures) / float64(len(cb.results))
		if failureRate >= cbFailureRateThreshold {
			cb.state = stateOpen
			cb.openedAt = time.Now()
			cb.results = nil
		}
	}
}

// WithRetry retries fn up to retryAttempts times with exponential backoff.
// A CircuitOpenError is never retried - a call rejected by an open circuit should fail
// fast, not burn through attempts.
func WithRetry(ctx context.Context, fn func() error) error {
	var lastErr error
	for attempt := 1; attempt <= retryAttempts; attempt++ {
		err := fn()
		if err == nil {
			return nil
		}
		lastErr = err
		if _, isOpen := err.(*CircuitOpenError); isOpen {
			return err
		}
		if attempt < retryAttempts {
			delay := retryBaseDelay * time.Duration(1<<(attempt-1))
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
	return lastErr
}

// Resilience wraps a call with a named circuit breaker and the shared retry policy.
type Resilience struct {
	breaker *CircuitBreaker
}

func New(name string) *Resilience {
	return &Resilience{breaker: NewCircuitBreaker(name)}
}

func (r *Resilience) Run(ctx context.Context, fn func() error) error {
	return WithRetry(ctx, func() error {
		return r.breaker.Execute(fn)
	})
}
