package main

import (
	"context"
	"log"
	"log/slog"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	apihandlers "github.com/apchavez/gcp-go/internal/api"
	"github.com/apchavez/gcp-go/internal/application"
	"github.com/apchavez/gcp-go/internal/infrastructure/noop"
	"github.com/apchavez/gcp-go/internal/infrastructure/repos"
	"github.com/apchavez/gcp-go/internal/shared"
)

func main() {
	ctx := context.Background()

	gcpProject := shared.MustEnv("GCP_PROJECT_ID")
	_ = shared.MustEnv("JWT_SECRET")

	fsClient, cleanup, err := shared.Bootstrap(ctx, gcpProject, "gcp-go-api")
	if err != nil {
		log.Fatalf("failed to bootstrap: %v", err)
	}
	defer cleanup()

	stateRepo := repos.NewFirestoreStateRepo(fsClient)
	eventStore := repos.NewFirestoreEventStore(fsClient)

	publisher := newPublisher(ctx, gcpProject)
	notifier := newNotifier()

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
	slog.Info("api listening", "port", port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}
