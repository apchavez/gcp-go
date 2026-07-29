package application_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/apchavez/gcp-go/internal/application"
	"github.com/apchavez/gcp-go/internal/domain"
)

// --- hand-written fakes, mirroring the AWS TS sibling's InMemoryStateRepo/CapturingMessageBus/etc. ---

type fakeStateRepo struct {
	items map[string]domain.Appointment
}

func newFakeStateRepo() *fakeStateRepo { return &fakeStateRepo{items: map[string]domain.Appointment{}} }

func (f *fakeStateRepo) Save(_ context.Context, a domain.Appointment) error {
	f.items[a.AppointmentUUID] = a
	return nil
}
func (f *fakeStateRepo) FindByID(_ context.Context, id string) (*domain.Appointment, error) {
	a, ok := f.items[id]
	if !ok {
		return nil, nil
	}
	return &a, nil
}
func (f *fakeStateRepo) markStatus(id, status string) error {
	a := f.items[id]
	a.Status = status
	f.items[id] = a
	return nil
}
func (f *fakeStateRepo) MarkCompleted(_ context.Context, id string) error {
	return f.markStatus(id, domain.StatusCompleted)
}
func (f *fakeStateRepo) ListByInsured(_ context.Context, insuredID string, pageSize int, cursor string) (domain.Page, error) {
	var items []domain.Appointment
	for _, a := range f.items {
		if a.InsuredID == insuredID {
			items = append(items, a)
		}
	}
	return domain.Page{Items: items}, nil
}

type capturingPublisher struct{ published []domain.Appointment }

func (p *capturingPublisher) Publish(_ context.Context, a domain.Appointment) error {
	p.published = append(p.published, a)
	return nil
}

type fakeEventStore struct{ events []domain.AppointmentEvent }

func (s *fakeEventStore) Append(_ context.Context, e domain.AppointmentEvent) error {
	s.events = append(s.events, e)
	return nil
}
func (s *fakeEventStore) FindByAppointmentID(_ context.Context, id string) ([]domain.AppointmentEvent, error) {
	var out []domain.AppointmentEvent
	for _, e := range s.events {
		if e.AppointmentUUID == id {
			out = append(out, e)
		}
	}
	return out, nil
}

type capturingNotifier struct {
	completed []domain.Appointment
}

func (n *capturingNotifier) NotifyCompleted(_ context.Context, a domain.Appointment) error {
	n.completed = append(n.completed, a)
	return nil
}

type fakeRelationalRepo struct{ persisted []domain.Appointment }

func (r *fakeRelationalRepo) Persist(_ context.Context, a domain.Appointment) error {
	r.persisted = append(r.persisted, a)
	return nil
}

type capturingConfirmationPublisher struct{ published []string }

func (p *capturingConfirmationPublisher) PublishConfirmed(_ context.Context, appointmentUUID string) error {
	p.published = append(p.published, appointmentUUID)
	return nil
}

func newService() (*application.AppointmentService, *fakeStateRepo, *capturingPublisher, *fakeEventStore, *capturingNotifier, *fakeRelationalRepo, *capturingConfirmationPublisher) {
	stateRepo := newFakeStateRepo()
	publisher := &capturingPublisher{}
	eventStore := &fakeEventStore{}
	notifier := &capturingNotifier{}
	relationalRepo := &fakeRelationalRepo{}
	confirmationPublisher := &capturingConfirmationPublisher{}
	svc := application.NewAppointmentService(stateRepo, publisher, eventStore, notifier, relationalRepo, confirmationPublisher)
	return svc, stateRepo, publisher, eventStore, notifier, relationalRepo, confirmationPublisher
}

func TestCreate(t *testing.T) {
	svc, stateRepo, publisher, eventStore, _, _, _ := newService()
	ctx := context.Background()

	appointment, err := svc.Create(ctx, application.CreateInput{InsuredID: "00001", ScheduleID: 1})

	require.NoError(t, err)
	assert.Equal(t, domain.StatusPending, appointment.Status)
	assert.NotEmpty(t, appointment.AppointmentUUID)
	assert.Contains(t, stateRepo.items, appointment.AppointmentUUID)
	assert.Len(t, publisher.published, 1)
	require.Len(t, eventStore.events, 1)
	assert.Equal(t, domain.EventAppointmentCreated, eventStore.events[0].EventType)
}

func TestComplete_IsIdempotent(t *testing.T) {
	svc, _, _, eventStore, notifier, _, _ := newService()
	ctx := context.Background()
	appointment, _ := svc.Create(ctx, application.CreateInput{InsuredID: "00001", ScheduleID: 1})

	require.NoError(t, svc.Complete(ctx, appointment.AppointmentUUID))
	eventsAfterFirst := len(eventStore.events)
	require.NoError(t, svc.Complete(ctx, appointment.AppointmentUUID)) // redelivery

	assert.Len(t, notifier.completed, 1)               // only notified once
	assert.Len(t, eventStore.events, eventsAfterFirst) // no duplicate COMPLETED event
}

func TestGetHistory(t *testing.T) {
	svc, _, _, _, _, _, _ := newService()
	ctx := context.Background()
	appointment, _ := svc.Create(ctx, application.CreateInput{InsuredID: "00001", ScheduleID: 1})

	events, err := svc.GetHistory(ctx, appointment.AppointmentUUID)

	require.NoError(t, err)
	require.Len(t, events, 1)
	assert.Equal(t, domain.EventAppointmentCreated, events[0].EventType)
}

func TestListByInsured(t *testing.T) {
	svc, _, _, _, _, _, _ := newService()
	ctx := context.Background()
	_, _ = svc.Create(ctx, application.CreateInput{InsuredID: "00001", ScheduleID: 1})
	_, _ = svc.Create(ctx, application.CreateInput{InsuredID: "00002", ScheduleID: 2})

	page, err := svc.ListByInsured(ctx, "00001", 20, "")

	require.NoError(t, err)
	require.Len(t, page.Items, 1)
	assert.Equal(t, "00001", page.Items[0].InsuredID)
}

func TestGetByID_NotFound(t *testing.T) {
	svc, _, _, _, _, _, _ := newService()

	appointment, err := svc.GetByID(context.Background(), "missing")

	require.NoError(t, err)
	assert.Nil(t, appointment)
}

func TestGetByID_Found(t *testing.T) {
	svc, _, _, _, _, _, _ := newService()
	ctx := context.Background()
	created, _ := svc.Create(ctx, application.CreateInput{InsuredID: "00001", ScheduleID: 1})

	found, err := svc.GetByID(ctx, created.AppointmentUUID)

	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, created.AppointmentUUID, found.AppointmentUUID)
}

func TestComplete_AppointmentNotFound_NoOps(t *testing.T) {
	svc, _, _, eventStore, notifier, _, _ := newService()

	err := svc.Complete(context.Background(), "missing")

	require.NoError(t, err)
	assert.Empty(t, eventStore.events)
	assert.Empty(t, notifier.completed)
}

func TestPersist_PersistsRelationallyAndPublishesConfirmation(t *testing.T) {
	svc, _, _, _, _, relationalRepo, confirmationPublisher := newService()
	ctx := context.Background()
	appointment, _ := svc.Create(ctx, application.CreateInput{InsuredID: "00001", ScheduleID: 1})

	err := svc.Persist(ctx, appointment.AppointmentUUID)

	require.NoError(t, err)
	require.Len(t, relationalRepo.persisted, 1)
	assert.Equal(t, appointment.AppointmentUUID, relationalRepo.persisted[0].AppointmentUUID)
	assert.Equal(t, []string{appointment.AppointmentUUID}, confirmationPublisher.published)
}

func TestPersist_AppointmentNotFound_NoOps(t *testing.T) {
	svc, _, _, _, _, relationalRepo, confirmationPublisher := newService()

	err := svc.Persist(context.Background(), "missing")

	require.NoError(t, err)
	assert.Empty(t, relationalRepo.persisted)
	assert.Empty(t, confirmationPublisher.published)
}

type erroringRelationalRepo struct{ persistErr error }

func (r *erroringRelationalRepo) Persist(_ context.Context, _ domain.Appointment) error {
	return r.persistErr
}

func TestPersist_PropagatesRelationalError(t *testing.T) {
	stateRepo := newFakeStateRepo()
	confirmationPublisher := &capturingConfirmationPublisher{}
	svc := application.NewAppointmentService(stateRepo, &capturingPublisher{}, &fakeEventStore{}, &capturingNotifier{}, &erroringRelationalRepo{persistErr: assert.AnError}, confirmationPublisher)
	ctx := context.Background()
	appointment, _ := svc.Create(ctx, application.CreateInput{InsuredID: "00001", ScheduleID: 1})

	err := svc.Persist(ctx, appointment.AppointmentUUID)

	assert.ErrorIs(t, err, assert.AnError)
	assert.Empty(t, confirmationPublisher.published)
}

type erroringConfirmationPublisher struct{ publishErr error }

func (p *erroringConfirmationPublisher) PublishConfirmed(_ context.Context, _ string) error {
	return p.publishErr
}

func TestPersist_PropagatesConfirmationPublishError(t *testing.T) {
	stateRepo := newFakeStateRepo()
	relationalRepo := &fakeRelationalRepo{}
	svc := application.NewAppointmentService(stateRepo, &capturingPublisher{}, &fakeEventStore{}, &capturingNotifier{}, relationalRepo, &erroringConfirmationPublisher{publishErr: assert.AnError})
	ctx := context.Background()
	appointment, _ := svc.Create(ctx, application.CreateInput{InsuredID: "00001", ScheduleID: 1})

	err := svc.Persist(ctx, appointment.AppointmentUUID)

	assert.ErrorIs(t, err, assert.AnError)
	assert.Len(t, relationalRepo.persisted, 1)
}

// --- error-path coverage using fakes that return an injected error ---

type erroringStateRepo struct {
	*fakeStateRepo
	saveErr error
}

func (r *erroringStateRepo) Save(ctx context.Context, a domain.Appointment) error {
	if r.saveErr != nil {
		return r.saveErr
	}
	return r.fakeStateRepo.Save(ctx, a)
}

type erroringPublisher struct{ publishErr error }

func (p *erroringPublisher) Publish(_ context.Context, _ domain.Appointment) error {
	return p.publishErr
}

func TestCreate_PropagatesSaveError(t *testing.T) {
	stateRepo := &erroringStateRepo{fakeStateRepo: newFakeStateRepo(), saveErr: assert.AnError}
	svc := application.NewAppointmentService(stateRepo, &capturingPublisher{}, &fakeEventStore{}, &capturingNotifier{}, &fakeRelationalRepo{}, &capturingConfirmationPublisher{})

	_, err := svc.Create(context.Background(), application.CreateInput{InsuredID: "00001", ScheduleID: 1})

	assert.ErrorIs(t, err, assert.AnError)
}

func TestCreate_PropagatesPublishError(t *testing.T) {
	stateRepo := newFakeStateRepo()
	publisher := &erroringPublisher{publishErr: assert.AnError}
	svc := application.NewAppointmentService(stateRepo, publisher, &fakeEventStore{}, &capturingNotifier{}, &fakeRelationalRepo{}, &capturingConfirmationPublisher{})

	_, err := svc.Create(context.Background(), application.CreateInput{InsuredID: "00001", ScheduleID: 1})

	assert.ErrorIs(t, err, assert.AnError)
}
