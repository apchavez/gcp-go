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
	stateRepo             domain.AppointmentStateRepository
	publisher             domain.AppointmentEventPublisher
	eventStore            domain.AppointmentEventStore
	notifier              domain.AppointmentNotifier
	relationalRepo        domain.AppointmentRelationalRepository
	confirmationPublisher domain.AppointmentConfirmationPublisher
}

func NewAppointmentService(
	stateRepo domain.AppointmentStateRepository,
	publisher domain.AppointmentEventPublisher,
	eventStore domain.AppointmentEventStore,
	notifier domain.AppointmentNotifier,
	relationalRepo domain.AppointmentRelationalRepository,
	confirmationPublisher domain.AppointmentConfirmationPublisher,
) *AppointmentService {
	return &AppointmentService{stateRepo, publisher, eventStore, notifier, relationalRepo, confirmationPublisher}
}

type CreateInput struct {
	InsuredID    string
	ScheduleID   int
	ContactEmail *string
}

func (s *AppointmentService) Create(ctx context.Context, in CreateInput) (domain.Appointment, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	appointment := domain.Appointment{
		AppointmentUUID: uuid.NewString(),
		InsuredID:       in.InsuredID,
		ScheduleID:      in.ScheduleID,
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

// Persist is invoked by the Pub/Sub-triggered worker (stage A, mirrors the AWS sibling's
// countryWorker λ). It writes the relational side (Cloud SQL here) and publishes a
// confirmation event that Eventarc routes to Complete (stage B) - it does NOT mark the
// aggregate COMPLETED or notify itself, matching the AWS EventBridge hop.
func (s *AppointmentService) Persist(ctx context.Context, appointmentUUID string) error {
	appointment, err := s.stateRepo.FindByID(ctx, appointmentUUID)
	if err != nil {
		return err
	}
	if appointment == nil {
		return nil
	}
	if err := s.relationalRepo.Persist(ctx, *appointment); err != nil {
		return err
	}
	return s.confirmationPublisher.PublishConfirmed(ctx, appointmentUUID)
}

// Complete is invoked by the confirm service (stage B, Eventarc-triggered off the
// "appointment-confirmed" topic that Persist publishes to). It is idempotent: if the
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
