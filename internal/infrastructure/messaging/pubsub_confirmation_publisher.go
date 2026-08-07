package messaging

import (
	"context"
	"encoding/json"
	"log/slog"

	"cloud.google.com/go/pubsub"

	"github.com/apchavez/gcp-go/internal/infrastructure/resilience"
)

// PubSubConfirmationPublisher implements domain.AppointmentConfirmationPublisher,
// publishing to the "appointment-confirmed" topic. An Eventarc trigger (see
// terraform/eventarc.tf) routes messages on this topic to the confirm Cloud Run service -
// mirroring the AWS sibling's EventBridgeConfirmationBus -> EventBridge -> SQS hop.
type PubSubConfirmationPublisher struct {
	confirmedTopic *pubsub.Topic
	res            *resilience.Resilience
}

func NewPubSubConfirmationPublisher(confirmedTopic *pubsub.Topic) *PubSubConfirmationPublisher {
	return &PubSubConfirmationPublisher{confirmedTopic: confirmedTopic, res: resilience.New("pubsub-confirmation-publisher")}
}

func (p *PubSubConfirmationPublisher) PublishConfirmed(ctx context.Context, appointmentUUID string) error {
	err := p.res.Run(ctx, func() error {
		payload, err := json.Marshal(appointmentPayload{AppointmentUUID: appointmentUUID})
		if err != nil {
			return err
		}
		result := p.confirmedTopic.Publish(ctx, &pubsub.Message{Data: payload})
		_, err = result.Get(ctx)
		return err
	})
	if err != nil {
		slog.Error("failed to publish appointment-confirmed event", "appointmentUuid", appointmentUUID, "error", err)
		return err
	}
	slog.Info("published appointment-confirmed event", "appointmentUuid", appointmentUUID)
	return nil
}
