package messaging

import (
	"context"
	"encoding/json"

	"cloud.google.com/go/pubsub"

	"github.com/apchavez/gcp-go/internal/domain"
	"github.com/apchavez/gcp-go/internal/infrastructure/resilience"
)

// PubSubPublisher implements domain.AppointmentEventPublisher, publishing newly created
// appointments to the single "appointment-created" topic, consumed by one shared worker
// push subscription (see terraform/pubsub.tf) - no per-country fan-out.
type PubSubPublisher struct {
	createdTopic *pubsub.Topic
	res          *resilience.Resilience
}

func NewPubSubPublisher(createdTopic *pubsub.Topic) *PubSubPublisher {
	return &PubSubPublisher{createdTopic: createdTopic, res: resilience.New("pubsub-publisher")}
}

func (p *PubSubPublisher) Publish(ctx context.Context, a domain.Appointment) error {
	return p.res.Run(ctx, func() error {
		payload, err := json.Marshal(a)
		if err != nil {
			return err
		}
		result := p.createdTopic.Publish(ctx, &pubsub.Message{
			Data: payload,
			Attributes: map[string]string{
				"country": a.CountryISO,
			},
		})
		_, err = result.Get(ctx)
		return err
	})
}
