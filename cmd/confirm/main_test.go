package main

import (
	"testing"

	"github.com/apchavez/gcp-go/internal/infrastructure/notifications"
)

func TestNewNotifier_ReturnsNoOpWhenSendGridKeyUnset(t *testing.T) {
	t.Setenv("SENDGRID_API_KEY", "")

	notifier := newNotifier()

	if _, ok := notifier.(notifications.NoOpNotifier); !ok {
		t.Fatalf("newNotifier() = %T, want notifications.NoOpNotifier", notifier)
	}
}

func TestNewNotifier_ReturnsSendGridNotifierWhenKeySet(t *testing.T) {
	t.Setenv("SENDGRID_API_KEY", "fake-key")
	t.Setenv("NOTIFIER_FROM_ADDRESS", "")

	notifier := newNotifier()

	if _, ok := notifier.(notifications.NoOpNotifier); ok {
		t.Fatal("newNotifier() returned NoOpNotifier, want a SendGridNotifier")
	}
}
