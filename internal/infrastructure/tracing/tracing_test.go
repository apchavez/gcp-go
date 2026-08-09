package tracing

import (
	"context"
	"testing"

	"github.com/apchavez/gcp-go/internal/testutil"
)

// TestInit_ConfiguresGlobalTracerProvider verifies the happy path: the Cloud Trace
// exporter and OTel resource construct successfully (neither validates GCP credentials
// eagerly - only the first real span export would), and the returned shutdown func runs
// without panicking.
func TestInit_ConfiguresGlobalTracerProvider(t *testing.T) {
	testutil.FakeGCPCredentials(t)
	ctx := context.Background()

	shutdown, err := Init(ctx, "fake-project-id", "test-service")

	if err != nil {
		t.Fatalf("Init() error = %v, want nil", err)
	}
	if shutdown == nil {
		t.Fatal("Init() returned nil shutdown func")
	}

	if err := shutdown(ctx); err != nil {
		t.Fatalf("shutdown() error = %v, want nil", err)
	}
}

func TestStartSpan_EndsSpanWithoutError(t *testing.T) {
	ctx := context.Background()

	spanCtx, end := StartSpan(ctx, "test-span")
	end(nil)

	if spanCtx == nil {
		t.Fatal("StartSpan() returned nil context")
	}
}

func TestStartSpan_RecordsError(t *testing.T) {
	ctx := context.Background()

	_, end := StartSpan(ctx, "test-span-with-error")
	end(context.DeadlineExceeded)
}
