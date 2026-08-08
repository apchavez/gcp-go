package messaging_test

import (
	"context"
	"testing"

	"cloud.google.com/go/pubsub"
	"cloud.google.com/go/pubsub/pstest"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/apchavez/gcp-go/internal/domain"
	"github.com/apchavez/gcp-go/internal/infrastructure/messaging"
)

// newFakePubSubTopic spins up an in-memory Pub/Sub emulator (pstest) with a single topic
// and returns a client wired to talk to it - no real GCP project/credentials needed.
func newFakePubSubTopic(t *testing.T, topicID string) *pubsub.Topic {
	t.Helper()
	srv := pstest.NewServer()
	t.Cleanup(func() { _ = srv.Close() })

	conn, err := grpc.NewClient(srv.Addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("grpc.NewClient() error = %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	client, err := pubsub.NewClient(context.Background(), "fake-project", option.WithGRPCConn(conn))
	if err != nil {
		t.Fatalf("pubsub.NewClient() error = %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })

	topic, err := client.CreateTopic(context.Background(), topicID)
	if err != nil {
		t.Fatalf("CreateTopic() error = %v", err)
	}
	return topic
}

func TestPubSubPublisher_Publish_Success(t *testing.T) {
	topic := newFakePubSubTopic(t, "appointment-created")
	publisher := messaging.NewPubSubPublisher(topic)

	err := publisher.Publish(context.Background(), domain.Appointment{AppointmentUUID: "uuid-1"})

	if err != nil {
		t.Fatalf("Publish() error = %v, want nil", err)
	}
}

func TestPubSubPublisher_Publish_ClosedTopicReturnsError(t *testing.T) {
	topic := newFakePubSubTopic(t, "appointment-created")
	topic.Stop()
	publisher := messaging.NewPubSubPublisher(topic)

	err := publisher.Publish(context.Background(), domain.Appointment{AppointmentUUID: "uuid-1"})

	if err == nil {
		t.Fatal("Publish() error = nil, want an error since the topic was stopped")
	}
}
