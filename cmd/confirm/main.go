// The confirm binary is a separate Cloud Run service invoked by an Eventarc trigger
// (terraform/eventarc.tf) bound to the "appointment-confirmed" Pub/Sub topic. It is stage B
// of the confirmation flow (mirrors the AWS sibling's confirmAppointment λ, consuming the
// SQS queue an EventBridge rule routes into): it marks the appointment COMPLETED in
// Firestore and sends the completion notification, decoupled from cmd/worker's relational
// persist (stage A).
package main

import (
	"context"
	"log"
	"log/slog"
	"net/http"
	"os"

	"cloud.google.com/go/firestore"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/apchavez/gcp-go/internal/application"
	"github.com/apchavez/gcp-go/internal/domain"
	"github.com/apchavez/gcp-go/internal/infrastructure/messaging"
	"github.com/apchavez/gcp-go/internal/infrastructure/noop"
	"github.com/apchavez/gcp-go/internal/infrastructure/notifications"
	"github.com/apchavez/gcp-go/internal/infrastructure/repos"
	"github.com/apchavez/gcp-go/internal/infrastructure/tracing"
)

func mustEnv(name string) string {
	v := os.Getenv(name)
	if v == "" {
		log.Fatalf("%s is not defined", name)
	}
	return v
}

func main() {
	ctx := context.Background()
	gcpProject := mustEnv("GCP_PROJECT_ID")

	shutdownTracing, err := tracing.Init(ctx, gcpProject, "gcp-go-confirm")
	if err != nil {
		log.Fatalf("failed to init tracing: %v", err)
	}
	defer func() {
		if err := shutdownTracing(context.Background()); err != nil {
			slog.Error("failed to shut down tracing", "error", err)
		}
	}()

	fsClient, err := firestore.NewClient(ctx, gcpProject)
	if err != nil {
		log.Fatalf("failed to create Firestore client: %v", err)
	}
	defer func() {
		if err := fsClient.Close(); err != nil {
			slog.Error("failed to close Firestore client", "error", err)
		}
	}()

	stateRepo := repos.NewFirestoreStateRepo(fsClient)
	eventStore := repos.NewFirestoreEventStore(fsClient)
	notifier := newNotifier()

	svc := application.NewAppointmentService(stateRepo, noop.Publisher{}, eventStore, notifier, noop.RelationalRepository{}, noop.ConfirmationPublisher{})

	mux := http.NewServeMux()
	mux.Handle("/", otelhttp.NewHandler(messaging.NewConfirmHandler(svc), "gcp-go-confirm"))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	slog.Info("confirm listening", "port", port)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}

func newNotifier() domain.AppointmentNotifier {
	apiKey := os.Getenv("SENDGRID_API_KEY")
	if apiKey == "" {
		return notifications.NoOpNotifier{}
	}
	from := os.Getenv("NOTIFIER_FROM_ADDRESS")
	if from == "" {
		from = "no-reply@clinic-scheduling.example.com"
	}
	return notifications.NewSendGridNotifier(apiKey, from)
}
