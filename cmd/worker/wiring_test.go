package main

import (
	"context"
	"testing"

	"cloud.google.com/go/pubsub"

	"github.com/apchavez/gcp-go/internal/testutil"
)

func TestNewConfirmationPublisher_UsesDefaultTopicName(t *testing.T) {
	testutil.FakeGCPCredentials(t)
	t.Setenv("PUBSUB_CONFIRMED_TOPIC", "")
	psClient, err := pubsub.NewClient(context.Background(), "fake-project")
	if err != nil {
		t.Fatalf("pubsub.NewClient() error = %v", err)
	}
	defer func() { _ = psClient.Close() }()

	publisher := newConfirmationPublisher(psClient)

	if publisher == nil {
		t.Fatal("newConfirmationPublisher() returned nil")
	}
}

func TestNewConfirmationPublisher_UsesCustomTopicName(t *testing.T) {
	testutil.FakeGCPCredentials(t)
	t.Setenv("PUBSUB_CONFIRMED_TOPIC", "custom-confirmed-topic")
	psClient, err := pubsub.NewClient(context.Background(), "fake-project")
	if err != nil {
		t.Fatalf("pubsub.NewClient() error = %v", err)
	}
	defer func() { _ = psClient.Close() }()

	publisher := newConfirmationPublisher(psClient)

	if publisher == nil {
		t.Fatal("newConfirmationPublisher() returned nil")
	}
}
