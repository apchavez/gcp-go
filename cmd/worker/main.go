// The worker binary is a separate Cloud Run service receiving Pub/Sub push-subscription
// HTTP callbacks from the single shared "appointment-worker" subscription (see
// terraform/pubsub.tf). It is stage A of the confirmation flow (mirrors the AWS sibling's
// countryWorker λ): it persists the appointment to Cloud SQL and publishes a confirmation
// event that an Eventarc trigger (terraform/eventarc.tf) routes to the confirm service
// (stage B, cmd/confirm), which marks the aggregate COMPLETED and notifies.
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"

	"cloud.google.com/go/firestore"
	"cloud.google.com/go/pubsub"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/apchavez/gcp-go/db/migration"
	"github.com/apchavez/gcp-go/internal/application"
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

	shutdownTracing, err := tracing.Init(ctx, gcpProject, "gcp-go-worker")
	if err != nil {
		log.Fatalf("failed to init tracing: %v", err)
	}
	defer func() {
		if err := shutdownTracing(context.Background()); err != nil {
			log.Printf("failed to shut down tracing: %v", err)
		}
	}()

	fsClient, err := firestore.NewClient(ctx, gcpProject)
	if err != nil {
		log.Fatalf("failed to create Firestore client: %v", err)
	}
	defer func() {
		if err := fsClient.Close(); err != nil {
			log.Printf("failed to close Firestore client: %v", err)
		}
	}()

	stateRepo := repos.NewFirestoreStateRepo(fsClient)

	sqlPool, err := pgxpool.New(ctx, cloudSQLDSN())
	if err != nil {
		log.Fatalf("failed to connect to Cloud SQL: %v", err)
	}
	defer sqlPool.Close()

	// Self-migrating on boot: there's no separate migration-runner step anywhere for this
	// project (see db/migration/migration.go for the full story), so the worker applies its
	// own idempotent DDL every time it starts rather than assuming a table that was never
	// actually created by anything.
	if _, err := sqlPool.Exec(ctx, migration.CreateAppointmentsTable); err != nil {
		log.Fatalf("failed to apply Cloud SQL migration: %v", err)
	}

	relationalRepo := repos.NewCloudSQLRepo(sqlPool)

	psClient, err := pubsub.NewClient(ctx, gcpProject)
	if err != nil {
		log.Fatalf("failed to create Pub/Sub client: %v", err)
	}
	defer func() {
		if err := psClient.Close(); err != nil {
			log.Printf("failed to close Pub/Sub client: %v", err)
		}
	}()
	confirmedTopicName := os.Getenv("PUBSUB_CONFIRMED_TOPIC")
	if confirmedTopicName == "" {
		confirmedTopicName = "appointment-confirmed"
	}
	confirmationPublisher := messaging.NewPubSubConfirmationPublisher(psClient.Topic(confirmedTopicName))

	// Persist (this binary's only call) never touches the publisher/eventStore/notifier
	// ports - those are exercised by cmd/api's Create and cmd/confirm's Complete respectively.
	svc := application.NewAppointmentService(stateRepo, noop.Publisher{}, noop.EventStore{}, notifications.NoOpNotifier{}, relationalRepo, confirmationPublisher)

	mux := http.NewServeMux()
	mux.Handle("/", otelhttp.NewHandler(messaging.NewWorkerHandler(svc), "gcp-go-worker"))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("worker listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}

// cloudSQLDSN builds a pgx DSN connecting via the Cloud SQL Auth unix socket mounted at
// /cloudsql/<connection_name> (see the volume/volume_mount on the worker Cloud Run service
// in terraform/cloudrun.tf) - CLOUDSQL_PASSWORD arrives via a Secret Manager-backed env var,
// so it can't be baked into a single Terraform-composed DSN string without exposing it in
// the Terraform plan/state as part of a larger interpolated value.
func cloudSQLDSN() string {
	host := mustEnv("CLOUDSQL_SOCKET_DIR")
	user := mustEnv("CLOUDSQL_USER")
	db := mustEnv("CLOUDSQL_DB")
	password := mustEnv("CLOUDSQL_PASSWORD")
	return fmt.Sprintf("postgres://%s:%s@/%s?host=%s", url.QueryEscape(user), url.QueryEscape(password), url.QueryEscape(db), url.QueryEscape(host))
}
