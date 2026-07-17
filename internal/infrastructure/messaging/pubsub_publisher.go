package messaging

import (
	"context"
	"encoding/json"

	"cloud.google.com/go/pubsub"

	"github.com/apchavez/gcp-go/internal/domain"
	"github.com/apchavez/gcp-go/internal/infrastructure/resilience"
)

// PubSubPublisher implements domain.AppointmentEventPublisher, publishing newly created
// appointments to the "appointment-created" topic with a "country" message attribute -
// the PE/CL push subscriptions filter on that attribute, replacing the AWS sibling's
// SNS-topic-per-message-attribute fan-out / Azure sibling's Service Bus SQL subscription
// filters (`sys.Subject='PE'`) with Pub/Sub's own native attribute-filter mechanism.
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

// CompletedCancelledPublisher publishes lifecycle events used by downstream consumers
// (e.g. analytics/reporting) - mirrors the "appointment-completed"/"appointment-cancelled"
// topics that exist in the Azure sibling's messaging layer, provisioned here in Terraform
// from the start (the Azure Bicep originally omitted the cancelled topic - fixed there,
// not repeated here).
type CompletedCancelledPublisher struct {
	completedTopic *pubsub.Topic
	cancelledTopic *pubsub.Topic
	res            *resilience.Resilience
}

func NewCompletedCancelledPublisher(completedTopic, cancelledTopic *pubsub.Topic) *CompletedCancelledPublisher {
	return &CompletedCancelledPublisher{completedTopic, cancelledTopic, resilience.New("pubsub-lifecycle-publisher")}
}

func (p *CompletedCancelledPublisher) PublishCompleted(ctx context.Context, a domain.Appointment) error {
	return p.publish(ctx, p.completedTopic, a)
}

func (p *CompletedCancelledPublisher) PublishCancelled(ctx context.Context, a domain.Appointment) error {
	return p.publish(ctx, p.cancelledTopic, a)
}

func (p *CompletedCancelledPublisher) publish(ctx context.Context, topic *pubsub.Topic, a domain.Appointment) error {
	return p.res.Run(ctx, func() error {
		payload, err := json.Marshal(a)
		if err != nil {
			return err
		}
		result := topic.Publish(ctx, &pubsub.Message{Data: payload})
		_, err = result.Get(ctx)
		return err
	})
}
