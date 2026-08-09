package shared

import (
	"context"
	"log"
	"log/slog"
	"os"

	"cloud.google.com/go/firestore"

	"github.com/apchavez/gcp-go/internal/infrastructure/tracing"
)

// MustEnv returns the value of the named environment variable, or terminates
// the process if it is unset - every binary needs it to start at all, so
// failing fast here beats a nil-pointer panic deeper in the call stack.
func MustEnv(name string) string {
	v := os.Getenv(name)
	if v == "" {
		log.Fatalf("%s is not defined", name)
	}
	return v
}

// Bootstrap wires the OpenTelemetry tracing exporter and a Firestore client
// shared by all three Cloud Run binaries (api/worker/confirm), returning a
// cleanup func that shuts both down. Cleanup failures are logged via slog
// rather than returned, since this always runs during process shutdown.
func Bootstrap(ctx context.Context, gcpProject, serviceName string) (*firestore.Client, func(), error) {
	shutdownTracing, err := tracing.Init(ctx, gcpProject, serviceName)
	if err != nil {
		return nil, nil, err
	}

	fsClient, err := firestore.NewClient(ctx, gcpProject)
	if err != nil {
		if shutErr := shutdownTracing(context.Background()); shutErr != nil {
			slog.Error("failed to shut down tracing", "error", shutErr)
		}
		return nil, nil, err
	}

	cleanup := func() {
		if err := fsClient.Close(); err != nil {
			slog.Error("failed to close Firestore client", "error", err)
		}
		if err := shutdownTracing(context.Background()); err != nil {
			slog.Error("failed to shut down tracing", "error", err)
		}
	}

	return fsClient, cleanup, nil
}
