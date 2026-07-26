package domain

import "context"

type Page struct {
	Items      []Appointment `json:"items"`
	NextCursor *string       `json:"nextCursor"`
}

// AppointmentStateRepository mirrors IAppointmentStateRepo (AWS sibling) / AppointmentStateRepository (Azure sibling).
type AppointmentStateRepository interface {
	Save(ctx context.Context, a Appointment) error
	FindByID(ctx context.Context, appointmentUUID string) (*Appointment, error)
	MarkCompleted(ctx context.Context, appointmentUUID string) error
	ListByInsured(ctx context.Context, insuredID string, pageSize int, cursor string) (Page, error)
}

// AppointmentEventStore mirrors IAppointmentEventStore - append-only event sourcing log.
type AppointmentEventStore interface {
	Append(ctx context.Context, e AppointmentEvent) error
	FindByAppointmentID(ctx context.Context, appointmentUUID string) ([]AppointmentEvent, error)
}

// AppointmentEventPublisher mirrors IMessageBus - publishes newly created appointments
// to the worker via Pub/Sub (this implementation).
type AppointmentEventPublisher interface {
	Publish(ctx context.Context, a Appointment) error
}

// AppointmentNotifier implementations are best-effort: a notification failure must NOT
// propagate to the caller - the appointment lifecycle takes precedence.
type AppointmentNotifier interface {
	NotifyCompleted(ctx context.Context, a Appointment) error
}

// AppointmentRelationalRepository persists only the final/completed appointment record
// to the relational store (Cloud SQL here), mirroring the AWS/Azure siblings' RDS/Azure SQL side.
type AppointmentRelationalRepository interface {
	Persist(ctx context.Context, a Appointment) error
}
