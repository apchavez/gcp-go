package messaging_test

import (
	"context"
	"testing"

	"github.com/apchavez/gcp-go/internal/infrastructure/messaging"
)

func TestPubSubConfirmationPublisher_PublishConfirmed_Success(t *testing.T) {
	topic := newFakePubSubTopic(t, "appointment-confirmed")
	publisher := messaging.NewPubSubConfirmationPublisher(topic)

	err := publisher.PublishConfirmed(context.Background(), "uuid-1")

	if err != nil {
		t.Fatalf("PublishConfirmed() error = %v, want nil", err)
	}
}

func TestPubSubConfirmationPublisher_PublishConfirmed_ClosedTopicReturnsError(t *testing.T) {
	topic := newFakePubSubTopic(t, "appointment-confirmed")
	topic.Stop()
	publisher := messaging.NewPubSubConfirmationPublisher(topic)

	err := publisher.PublishConfirmed(context.Background(), "uuid-1")

	if err == nil {
		t.Fatal("PublishConfirmed() error = nil, want an error since the topic was stopped")
	}
}
