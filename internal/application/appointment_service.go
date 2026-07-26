package application

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/apchavez/gcp-go/internal/domain"
)

// AppointmentService mirrors the AWS TypeScript sibling's AppointmentService
// (src/app/usecases/appointment.service.ts) method-for-method.
type AppointmentService struct {
	stateRepo domain.AppointmentStateRepository
	publisher domain.AppointmentEventPublisher
	eventStore domain.AppointmentEventStore
	notifier  domain.AppointmentNotifier
}

func NewAppointmentService(
	stateRepo domain.AppointmentStateRepository,
	publisher domain.AppointmentEventPublisher,
	eventStore domain.AppointmentEventStore,
	notifier domain.AppointmentNotifier,
) *AppointmentService {
	return &AppointmentService{stateRepo, publisher, eventStore, notifier}
}

type CreateInput struct {
	InsuredID    string
	ScheduleID   int
	CountryISO   string
	ContactEmail *string
}

func (s *AppointmentService) Create(ctx context.Context, in CreateInput) (domain.Appointment, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	appointment := domain.Appointment{
		AppointmentUUID: uuid.NewString(),
		InsuredID:       in.InsuredID,
		ScheduleID:      in.ScheduleID,
		CountryISO:      in.CountryISO,
		Status:          domain.StatusPending,
		CreatedAt:       now,
		UpdatedAt:       now,
		ContactEmail:    in.ContactEmail,
	}
	if err := s.stateRepo.Save(ctx, appointment); err != nil {
		return domain.Appointment{}, err
	}
	if err := s.publisher.Publish(ctx, appointment); err != nil {
		return domain.Appointment{}, err
	}
	if err := s.eventStore.Append(ctx, domain.NewAppointmentEvent(domain.EventAppointmentCreated, appointment)); err != nil {
		return domain.Appointment{}, err
	}
	return appointment, nil
}

func (s *AppointmentService) ListByInsured(ctx context.Context, insuredID string, pageSize int, cursor string) (domain.Page, error) {
	return s.stateRepo.ListByInsured(ctx, insuredID, pageSize, cursor)
}

func (s *AppointmentService) GetByID(ctx context.Context, appointmentUUID string) (*domain.Appointment, error) {
	return s.stateRepo.FindByID(ctx, appointmentUUID)
}

func (s *AppointmentService) GetHistory(ctx context.Context, appointmentUUID string) ([]domain.AppointmentEvent, error) {
	return s.eventStore.FindByAppointmentID(ctx, appointmentUUID)
}

// Complete is invoked by the Pub/Sub-triggered worker. It is idempotent: if the
// appointment is already COMPLETED (at-least-once redelivery), it silently no-ops.
func (s *AppointmentService) Complete(ctx context.Context, appointmentUUID string) error {
	appointment, err := s.stateRepo.FindByID(ctx, appointmentUUID)
	if err != nil {
		return err
	}
	if appointment == nil {
		return nil
	}
	if appointment.Status == domain.StatusCompleted {
		return nil
	}
	if err := s.stateRepo.MarkCompleted(ctx, appointmentUUID); err != nil {
		return err
	}
	appointment.Status = domain.StatusCompleted
	appointment.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	if err := s.eventStore.Append(ctx, domain.NewAppointmentEvent(domain.EventAppointmentCompleted, *appointment)); err != nil {
		return err
	}
	return s.notifier.NotifyCompleted(ctx, *appointment)
}

