package noop_test

import (
	"context"
	"testing"

	"github.com/apchavez/gcp-go/internal/domain"
	"github.com/apchavez/gcp-go/internal/infrastructure/noop"
)

// TestNoopImplementations_AreSafeZeroOps guards the contract every no-op adapter must
// hold: each method is callable on a zero-value receiver and never errors, since the
// three Cloud Run binaries (api/worker/confirm) each wire a different subset of these
// into AppointmentService depending on which ports they don't exercise.
func TestNoopImplementations_AreSafeZeroOps(t *testing.T) {
	ctx := context.Background()

	if err := (noop.Publisher{}).Publish(ctx, domain.Appointment{}); err != nil {
		t.Fatalf("Publisher.Publish() error = %v, want nil", err)
	}

	if err := (noop.EventStore{}).Append(ctx, domain.AppointmentEvent{}); err != nil {
		t.Fatalf("EventStore.Append() error = %v, want nil", err)
	}
	events, err := (noop.EventStore{}).FindByAppointmentID(ctx, "uuid-1")
	if err != nil {
		t.Fatalf("EventStore.FindByAppointmentID() error = %v, want nil", err)
	}
	if events != nil {
		t.Fatalf("EventStore.FindByAppointmentID() = %v, want nil", events)
	}

	if err := (noop.RelationalRepository{}).Persist(ctx, domain.Appointment{}); err != nil {
		t.Fatalf("RelationalRepository.Persist() error = %v, want nil", err)
	}

	if err := (noop.ConfirmationPublisher{}).PublishConfirmed(ctx, "uuid-1"); err != nil {
		t.Fatalf("ConfirmationPublisher.PublishConfirmed() error = %v, want nil", err)
	}
}
