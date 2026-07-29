package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"cloud.google.com/go/firestore"
	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	apihandlers "github.com/apchavez/gcp-go/internal/api"
	"github.com/apchavez/gcp-go/internal/application"
	"github.com/apchavez/gcp-go/internal/infrastructure/noop"
	"github.com/apchavez/gcp-go/internal/infrastructure/repos"
	"github.com/apchavez/gcp-go/internal/infrastructure/tracing"
	"github.com/apchavez/gcp-go/internal/shared"
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
	_ = mustEnv("JWT_SECRET") // read once at startup so a misconfigured deploy fails fast

	shutdownTracing, err := tracing.Init(ctx, gcpProject, "gcp-go-api")
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
	eventStore := repos.NewFirestoreEventStore(fsClient)

	publisher := newPublisher(ctx, gcpProject)
	notifier := newNotifier()

	// The api service never touches the relational store or the confirmation topic - those
	// are wired for real in cmd/worker (Persist, stage A) and cmd/confirm (Complete, stage B).
	svc := application.NewAppointmentService(stateRepo, publisher, eventStore, notifier, noop.RelationalRepository{}, noop.ConfirmationPublisher{})
	handlers := apihandlers.NewHandlers(svc)

	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return otelhttp.NewHandler(next, "gcp-go-api")
	})
	r.Post("/appointments", handlers.CreateAppointment)
	r.Get("/appointments/{insuredId}", handlers.ListByInsured)
	r.Get("/appointments/{appointmentUuid}/history", handlers.GetAppointmentHistory)
	r.Get("/health", handlers.Health(func(req *http.Request) shared.HealthStatus {
		checks := map[string]string{"firestore": shared.HealthUp}
		if _, err := fsClient.Collection("appointments").Limit(1).Documents(req.Context()).Next(); err != nil && err.Error() != "no more items in iterator" {
			checks["firestore"] = shared.HealthDown
		}
		return shared.NewHealthStatus(checks)
	}))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}
