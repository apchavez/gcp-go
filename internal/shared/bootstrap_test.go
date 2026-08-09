package shared

import (
	"context"
	"testing"

	"github.com/apchavez/gcp-go/internal/testutil"
)

func TestMustEnv_ReturnsValueWhenSet(t *testing.T) {
	t.Setenv("GCP_GO_TEST_VAR", "some-value")

	got := MustEnv("GCP_GO_TEST_VAR")

	if got != "some-value" {
		t.Fatalf("MustEnv() = %q, want %q", got, "some-value")
	}
}

// TestBootstrap_WiresTracingAndFirestore verifies the happy path: both the OpenTelemetry
// exporter and the Firestore client construct successfully (neither validates credentials
// eagerly - that only happens on the first real network call), and the returned cleanup
// func runs without panicking. The tracing/Firestore error branches aren't exercised here:
// forcing texporter.New or firestore.NewClient to fail would require injecting a broken
// gRPC transport, which isn't worth the complexity for bootstrap wiring code.
func TestBootstrap_WiresTracingAndFirestore(t *testing.T) {
	testutil.FakeGCPCredentials(t)
	ctx := context.Background()

	client, cleanup, err := Bootstrap(ctx, "fake-project-id", "test-service")

	if err != nil {
		t.Fatalf("Bootstrap() error = %v, want nil", err)
	}
	if client == nil {
		t.Fatal("Bootstrap() returned nil Firestore client")
	}
	if cleanup == nil {
		t.Fatal("Bootstrap() returned nil cleanup func")
	}

	cleanup()
}
