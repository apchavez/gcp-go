package repos

import (
	"context"
	"testing"
	"time"

	"github.com/apchavez/gcp-go/internal/domain"
	"github.com/apchavez/gcp-go/internal/testutil"
)

func newTestEvent(eventID, appointmentUUID, eventType string) domain.AppointmentEvent {
	return domain.AppointmentEvent{
		EventID:         eventID,
		AppointmentUUID: appointmentUUID,
		EventType:       eventType,
		InsuredID:       "insured-event-store",
		ScheduleID:      42,
		Status:          domain.StatusPending,
		OccurredAt:      time.Now().UTC().Format(time.RFC3339Nano),
	}
}

func TestFirestoreEventStore_AppendAndFindByAppointmentID(t *testing.T) {
	client := testutil.NewFirestoreEmulatorClient(t)
	store := NewFirestoreEventStore(client)
	ctx := context.Background()
	appointmentUUID := "event-store-appt-001"

	created := newTestEvent("event-001", appointmentUUID, domain.EventAppointmentCreated)
	completed := newTestEvent("event-002", appointmentUUID, domain.EventAppointmentCompleted)
	completed.OccurredAt = time.Now().UTC().Add(time.Second).Format(time.RFC3339Nano)

	if err := store.Append(ctx, created); err != nil {
		t.Fatalf("Append() error = %v, want nil", err)
	}
	if err := store.Append(ctx, completed); err != nil {
		t.Fatalf("Append() error = %v, want nil", err)
	}

	events, err := store.FindByAppointmentID(ctx, appointmentUUID)
	if err != nil {
		t.Fatalf("FindByAppointmentID() error = %v, want nil", err)
	}
	if len(events) != 2 {
		t.Fatalf("len(events) = %d, want 2", len(events))
	}
	if events[0].EventType != domain.EventAppointmentCreated || events[1].EventType != domain.EventAppointmentCompleted {
		t.Fatalf("events = %+v, want CREATED before COMPLETED (ordered by occurredAt)", events)
	}
}

func TestFirestoreEventStore_FindByAppointmentID_UnknownAppointmentReturnsEmpty(t *testing.T) {
	client := testutil.NewFirestoreEmulatorClient(t)
	store := NewFirestoreEventStore(client)

	events, err := store.FindByAppointmentID(context.Background(), "does-not-exist")

	if err != nil {
		t.Fatalf("FindByAppointmentID() error = %v, want nil", err)
	}
	if len(events) != 0 {
		t.Fatalf("len(events) = %d, want 0", len(events))
	}
}
