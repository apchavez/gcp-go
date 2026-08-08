package main

import (
	"context"
	"log"
	"os"

	"cloud.google.com/go/pubsub"

	"github.com/apchavez/gcp-go/internal/domain"
	"github.com/apchavez/gcp-go/internal/infrastructure/messaging"
	"github.com/apchavez/gcp-go/internal/infrastructure/notifications"
)

func newPublisher(ctx context.Context, gcpProject string) domain.AppointmentEventPublisher {
	psClient, err := pubsub.NewClient(ctx, gcpProject)
	if err != nil {
		log.Fatalf("failed to create Pub/Sub client: %v", err)
	}
	topicName := os.Getenv("PUBSUB_CREATED_TOPIC")
	if topicName == "" {
		topicName = "appointment-created"
	}
	return messaging.NewPubSubPublisher(psClient.Topic(topicName))
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
