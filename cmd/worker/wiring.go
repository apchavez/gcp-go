package main

import (
	"os"

	"cloud.google.com/go/pubsub"

	"github.com/apchavez/gcp-go/internal/domain"
	"github.com/apchavez/gcp-go/internal/infrastructure/messaging"
)

func newConfirmationPublisher(psClient *pubsub.Client) domain.AppointmentConfirmationPublisher {
	confirmedTopicName := os.Getenv("PUBSUB_CONFIRMED_TOPIC")
	if confirmedTopicName == "" {
		confirmedTopicName = "appointment-confirmed"
	}
	return messaging.NewPubSubConfirmationPublisher(psClient.Topic(confirmedTopicName))
}
