// Package noop provides no-op implementations of the AppointmentService ports that a
// given Cloud Run service (api/worker/confirm) doesn't exercise - each binary only drives
// a subset of AppointmentService's methods, but the constructor requires all six ports.
package noop

import (
	"context"

	"github.com/apchavez/gcp-go/internal/domain"
)

type Publisher struct{}

func (Publisher) Publish(context.Context, domain.Appointment) error { return nil }

type EventStore struct{}

func (EventStore) Append(context.Context, domain.AppointmentEvent) error { return nil }

func (EventStore) FindByAppointmentID(context.Context, string) ([]domain.AppointmentEvent, error) {
	return nil, nil
}

type RelationalRepository struct{}

func (RelationalRepository) Persist(context.Context, domain.Appointment) error { return nil }

type ConfirmationPublisher struct{}

func (ConfirmationPublisher) PublishConfirmed(context.Context, string) error { return nil }
