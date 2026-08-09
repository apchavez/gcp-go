package testutil

import (
	"context"
	"os"
	"testing"

	"cloud.google.com/go/firestore"
)

// NewFirestoreEmulatorClient returns a *firestore.Client pointed at the Firestore
// emulator (FIRESTORE_EMULATOR_HOST). It skips the test if the emulator isn't reachable,
// so the suite still runs (minus these cases) on a machine/CI job without the emulator
// wired up, instead of failing outright.
func NewFirestoreEmulatorClient(t *testing.T) *firestore.Client {
	t.Helper()

	host := os.Getenv("FIRESTORE_EMULATOR_HOST")
	if host == "" {
		host = "localhost:8080"
		t.Setenv("FIRESTORE_EMULATOR_HOST", host)
	}

	ctx := context.Background()
	client, err := firestore.NewClient(ctx, "fake-project-id")
	if err != nil {
		t.Skipf("firestore emulator not reachable at %s: %v", host, err)
	}

	// Fail fast (and skip, not fatal) if the emulator isn't actually listening -
	// NewClient above succeeds even with nothing running, since it doesn't dial until
	// the first real call.
	it := client.Collections(ctx)
	if _, err := it.Next(); err != nil && err.Error() != "no more items in iterator" {
		_ = client.Close()
		t.Skipf("firestore emulator not reachable at %s: %v", host, err)
	}

	t.Cleanup(func() { _ = client.Close() })
	return client
}
