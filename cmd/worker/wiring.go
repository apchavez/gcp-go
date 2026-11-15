package main

import (
	"os"

	"cloud.google.com/go/pubsub/v2"

	"github.com/apchavez/gcp-go/internal/domain"
	"github.com/apchavez/gcp-go/internal/infrastructure/messaging"
)

func newConfirmationPublisher(psClient *pubsub.Client) domain.AppointmentConfirmationPublisher {
	confirmedTopicName := os.Getenv("PUBSUB_CONFIRMED_TOPIC")
	if confirmedTopicName == "" {
		confirmedTopicName = "appointment-confirmed"
	}
	return messaging.NewPubSubConfirmationPublisher(psClient.Publisher(confirmedTopicName))
}
