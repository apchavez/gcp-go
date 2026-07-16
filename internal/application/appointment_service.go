package application

import (
	"context"
	"fmt"
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

// Complete is invoked by the Pub/Sub-triggered country worker. It is idempotent: if the
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

func (s *AppointmentService) Cancel(ctx context.Context, appointmentUUID string) error {
	appointment, err := s.requirePending(ctx, appointmentUUID, "cancelled")
	if err != nil {
		return err
	}
	if err := s.stateRepo.MarkCancelled(ctx, appointmentUUID); err != nil {
		return err
	}
	updated := *appointment
	updated.Status = domain.StatusCancelled
	updated.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	if err := s.eventStore.Append(ctx, domain.NewAppointmentEvent(domain.EventAppointmentCancelled, updated)); err != nil {
		return err
	}
	return s.notifier.NotifyCancelled(ctx, updated)
}

func (s *AppointmentService) Reschedule(ctx context.Context, appointmentUUID string, newScheduleID int) (domain.Appointment, error) {
	old, err := s.requirePending(ctx, appointmentUUID, "rescheduled")
	if err != nil {
		return domain.Appointment{}, err
	}
	if err := s.stateRepo.MarkRescheduled(ctx, appointmentUUID); err != nil {
		return domain.Appointment{}, err
	}
	rescheduledOld := *old
	rescheduledOld.Status = domain.StatusRescheduled
	rescheduledOld.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	now := time.Now().UTC().Format(time.RFC3339)
	newAppointment := domain.Appointment{
		AppointmentUUID: uuid.NewString(),
		InsuredID:       old.InsuredID,
		ScheduleID:      newScheduleID,
		CountryISO:      old.CountryISO,
		Status:          domain.StatusPending,
		CreatedAt:       now,
		UpdatedAt:       now,
		ContactEmail:    old.ContactEmail,
	}
	if err := s.stateRepo.Save(ctx, newAppointment); err != nil {
		return domain.Appointment{}, err
	}
	if err := s.publisher.Publish(ctx, newAppointment); err != nil {
		return domain.Appointment{}, err
	}
	if err := s.eventStore.Append(ctx, domain.NewAppointmentEvent(domain.EventAppointmentRescheduled, rescheduledOld)); err != nil {
		return domain.Appointment{}, err
	}
	if err := s.eventStore.Append(ctx, domain.NewAppointmentEvent(domain.EventAppointmentCreated, newAppointment)); err != nil {
		return domain.Appointment{}, err
	}
	if err := s.notifier.NotifyRescheduled(ctx, rescheduledOld, newAppointment); err != nil {
		return domain.Appointment{}, err
	}
	return newAppointment, nil
}

func (s *AppointmentService) requirePending(ctx context.Context, appointmentUUID, action string) (*domain.Appointment, error) {
	appointment, err := s.stateRepo.FindByID(ctx, appointmentUUID)
	if err != nil {
		return nil, err
	}
	if appointment == nil {
		return nil, &domain.NotFoundError{Message: fmt.Sprintf("Appointment not found: %s", appointmentUUID)}
	}
	if appointment.Status != domain.StatusPending {
		return nil, &domain.ConflictError{Message: fmt.Sprintf("Only a PENDING appointment can be %s", action)}
	}
	return appointment, nil
}
