// The worker binary is a separate Cloud Run service receiving Pub/Sub push-subscription
// HTTP callbacks for the PE/CL country subscriptions (each filtered by the "country"
// message attribute at the subscription level - see terraform/pubsub.tf). It calls
// AppointmentService.Complete, which is idempotent against at-least-once redelivery.
package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"cloud.google.com/go/firestore"

	"github.com/apchavez/gcp-go/internal/application"
	"github.com/apchavez/gcp-go/internal/domain"
	"github.com/apchavez/gcp-go/internal/infrastructure/messaging"
	"github.com/apchavez/gcp-go/internal/infrastructure/notifications"
	"github.com/apchavez/gcp-go/internal/infrastructure/repos"
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
	eventStore := repos.NewFirestoreEventStore(fsClient)

	notifier := newNotifier()

	// The publisher port is only exercised by CreateAppointment/Reschedule, which the
	// worker binary never calls (it only handles Complete) - a no-op is correct here.
	svc := application.NewAppointmentService(stateRepo, noopPublisher{}, eventStore, notifier)

	mux := http.NewServeMux()
	mux.Handle("/", messaging.NewWorkerHandler(svc))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("worker listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}

type noopPublisher struct{}

func (noopPublisher) Publish(ctx context.Context, a domain.Appointment) error { return nil }

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
