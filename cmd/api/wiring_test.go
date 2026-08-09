package main

import (
	"context"
	"testing"

	"github.com/apchavez/gcp-go/internal/infrastructure/notifications"
	"github.com/apchavez/gcp-go/internal/testutil"
)

// TestNewPublisher_ConstructsPubSubPublisher verifies the happy path: pubsub.NewClient
// doesn't validate credentials eagerly (only the first real Publish call would), so this
// exercises the topic-name-default branch without needing a real GCP project.
func TestNewPublisher_ConstructsPubSubPublisher(t *testing.T) {
	testutil.FakeGCPCredentials(t)
	t.Setenv("PUBSUB_CREATED_TOPIC", "")

	publisher := newPublisher(context.Background(), "fake-project")

	if publisher == nil {
		t.Fatal("newPublisher() returned nil")
	}
}

func TestNewPublisher_UsesCustomTopicName(t *testing.T) {
	testutil.FakeGCPCredentials(t)
	t.Setenv("PUBSUB_CREATED_TOPIC", "custom-topic")

	publisher := newPublisher(context.Background(), "fake-project")

	if publisher == nil {
		t.Fatal("newPublisher() returned nil")
	}
}

func TestNewNotifier_ReturnsNoOpWhenSendGridKeyUnset(t *testing.T) {
	t.Setenv("SENDGRID_API_KEY", "")

	notifier := newNotifier()

	if _, ok := notifier.(notifications.NoOpNotifier); !ok {
		t.Fatalf("newNotifier() = %T, want notifications.NoOpNotifier", notifier)
	}
}

func TestNewNotifier_ReturnsSendGridNotifierWhenKeySet(t *testing.T) {
	t.Setenv("SENDGRID_API_KEY", "fake-key")
	t.Setenv("NOTIFIER_FROM_ADDRESS", "custom@example.com")

	notifier := newNotifier()

	if _, ok := notifier.(notifications.NoOpNotifier); ok {
		t.Fatal("newNotifier() returned NoOpNotifier, want a SendGridNotifier")
	}
}
