package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"cloud.google.com/go/firestore"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	apihandlers "github.com/apchavez/gcp-go/internal/api"
	"github.com/apchavez/gcp-go/internal/application"
	"github.com/apchavez/gcp-go/internal/infrastructure/repos"
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

	svc := application.NewAppointmentService(stateRepo, publisher, eventStore, notifier)
	handlers := apihandlers.NewHandlers(svc)

	var sqlPool *pgxpool.Pool
	if dsn := os.Getenv("CLOUDSQL_DSN"); dsn != "" {
		sqlPool, err = pgxpool.New(ctx, dsn)
		if err != nil {
			log.Printf("warning: failed to connect to Cloud SQL: %v", err)
		}
	}
	_ = sqlPool // wired into ProcessAppointment via the worker binary, not the API service

	r := chi.NewRouter()
	r.Post("/appointments", handlers.CreateAppointment)
	r.Get("/appointments/{insuredId}", handlers.ListByInsured)
	r.Delete("/appointments/{appointmentUuid}", handlers.CancelAppointment)
	r.Patch("/appointments/{appointmentUuid}/reschedule", handlers.RescheduleAppointment)
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
