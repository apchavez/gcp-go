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
func (f *fakeStateRepo) MarkCompleted(_ context.Context, id string) error   { return f.markStatus(id, domain.StatusCompleted) }
func (f *fakeStateRepo) MarkCancelled(_ context.Context, id string) error   { return f.markStatus(id, domain.StatusCancelled) }
func (f *fakeStateRepo) MarkRescheduled(_ context.Context, id string) error { return f.markStatus(id, domain.StatusRescheduled) }
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
	completed, cancelled []domain.Appointment
	rescheduledOld, rescheduledNew *domain.Appointment
}

func (n *capturingNotifier) NotifyCompleted(_ context.Context, a domain.Appointment) error {
	n.completed = append(n.completed, a)
	return nil
}
func (n *capturingNotifier) NotifyCancelled(_ context.Context, a domain.Appointment) error {
	n.cancelled = append(n.cancelled, a)
	return nil
}
func (n *capturingNotifier) NotifyRescheduled(_ context.Context, old, updated domain.Appointment) error {
	n.rescheduledOld = &old
	n.rescheduledNew = &updated
	return nil
}

func newService() (*application.AppointmentService, *fakeStateRepo, *capturingPublisher, *fakeEventStore, *capturingNotifier) {
	stateRepo := newFakeStateRepo()
	publisher := &capturingPublisher{}
	eventStore := &fakeEventStore{}
	notifier := &capturingNotifier{}
	svc := application.NewAppointmentService(stateRepo, publisher, eventStore, notifier)
	return svc, stateRepo, publisher, eventStore, notifier
}

func TestCreate(t *testing.T) {
	svc, stateRepo, publisher, eventStore, _ := newService()
	ctx := context.Background()

	appointment, err := svc.Create(ctx, application.CreateInput{InsuredID: "00001", ScheduleID: 1, CountryISO: domain.CountryPE})

	require.NoError(t, err)
	assert.Equal(t, domain.StatusPending, appointment.Status)
	assert.NotEmpty(t, appointment.AppointmentUUID)
	assert.Contains(t, stateRepo.items, appointment.AppointmentUUID)
	assert.Len(t, publisher.published, 1)
	require.Len(t, eventStore.events, 1)
	assert.Equal(t, domain.EventAppointmentCreated, eventStore.events[0].EventType)
}

func TestCancel_HappyPath(t *testing.T) {
	svc, stateRepo, _, eventStore, notifier := newService()
	ctx := context.Background()
	appointment, _ := svc.Create(ctx, application.CreateInput{InsuredID: "00001", ScheduleID: 1, CountryISO: domain.CountryPE})

	err := svc.Cancel(ctx, appointment.AppointmentUUID)

	require.NoError(t, err)
	assert.Equal(t, domain.StatusCancelled, stateRepo.items[appointment.AppointmentUUID].Status)
	assert.Len(t, notifier.cancelled, 1)
	assert.Equal(t, domain.EventAppointmentCancelled, eventStore.events[len(eventStore.events)-1].EventType)
}

func TestCancel_NotFound(t *testing.T) {
	svc, _, _, _, _ := newService()
	err := svc.Cancel(context.Background(), "missing")

	var notFound *domain.NotFoundError
	require.ErrorAs(t, err, &notFound)
}

func TestCancel_AlreadyCompleted_ReturnsConflict(t *testing.T) {
	svc, stateRepo, _, _, _ := newService()
	ctx := context.Background()
	appointment, _ := svc.Create(ctx, application.CreateInput{InsuredID: "00001", ScheduleID: 1, CountryISO: domain.CountryPE})
	require.NoError(t, svc.Complete(ctx, appointment.AppointmentUUID))
	_ = stateRepo

	err := svc.Cancel(ctx, appointment.AppointmentUUID)

	var conflict *domain.ConflictError
	require.ErrorAs(t, err, &conflict)
}

func TestReschedule_CreatesNewPendingAppointment(t *testing.T) {
	svc, stateRepo, publisher, eventStore, notifier := newService()
	ctx := context.Background()
	old, _ := svc.Create(ctx, application.CreateInput{InsuredID: "00001", ScheduleID: 1, CountryISO: domain.CountryCL})

	newAppointment, err := svc.Reschedule(ctx, old.AppointmentUUID, 99)

	require.NoError(t, err)
	assert.NotEqual(t, old.AppointmentUUID, newAppointment.AppointmentUUID)
	assert.Equal(t, domain.StatusPending, newAppointment.Status)
	assert.Equal(t, 99, newAppointment.ScheduleID)
	assert.Equal(t, domain.StatusRescheduled, stateRepo.items[old.AppointmentUUID].Status)
	assert.Len(t, publisher.published, 2) // original create + reschedule's new-appointment publish
	assert.NotNil(t, notifier.rescheduledOld)
	assert.NotNil(t, notifier.rescheduledNew)

	var createdCount, rescheduledCount int
	for _, e := range eventStore.events {
		switch e.EventType {
		case domain.EventAppointmentCreated:
			createdCount++
		case domain.EventAppointmentRescheduled:
			rescheduledCount++
		}
	}
	assert.Equal(t, 2, createdCount) // original + rescheduled-new
	assert.Equal(t, 1, rescheduledCount)
}

func TestComplete_IsIdempotent(t *testing.T) {
	svc, _, _, eventStore, notifier := newService()
	ctx := context.Background()
	appointment, _ := svc.Create(ctx, application.CreateInput{InsuredID: "00001", ScheduleID: 1, CountryISO: domain.CountryPE})

	require.NoError(t, svc.Complete(ctx, appointment.AppointmentUUID))
	eventsAfterFirst := len(eventStore.events)
	require.NoError(t, svc.Complete(ctx, appointment.AppointmentUUID)) // redelivery

	assert.Len(t, notifier.completed, 1) // only notified once
	assert.Len(t, eventStore.events, eventsAfterFirst) // no duplicate COMPLETED event
}

func TestGetHistory(t *testing.T) {
	svc, _, _, _, _ := newService()
	ctx := context.Background()
	appointment, _ := svc.Create(ctx, application.CreateInput{InsuredID: "00001", ScheduleID: 1, CountryISO: domain.CountryPE})

	events, err := svc.GetHistory(ctx, appointment.AppointmentUUID)

	require.NoError(t, err)
	require.Len(t, events, 1)
	assert.Equal(t, domain.EventAppointmentCreated, events[0].EventType)
}

func TestListByInsured(t *testing.T) {
	svc, _, _, _, _ := newService()
	ctx := context.Background()
	_, _ = svc.Create(ctx, application.CreateInput{InsuredID: "00001", ScheduleID: 1, CountryISO: domain.CountryPE})
	_, _ = svc.Create(ctx, application.CreateInput{InsuredID: "00002", ScheduleID: 2, CountryISO: domain.CountryCL})

	page, err := svc.ListByInsured(ctx, "00001", 20, "")

	require.NoError(t, err)
	require.Len(t, page.Items, 1)
	assert.Equal(t, "00001", page.Items[0].InsuredID)
}

func TestGetByID_NotFound(t *testing.T) {
	svc, _, _, _, _ := newService()

	appointment, err := svc.GetByID(context.Background(), "missing")

	require.NoError(t, err)
	assert.Nil(t, appointment)
}

func TestGetByID_Found(t *testing.T) {
	svc, _, _, _, _ := newService()
	ctx := context.Background()
	created, _ := svc.Create(ctx, application.CreateInput{InsuredID: "00001", ScheduleID: 1, CountryISO: domain.CountryPE})

	found, err := svc.GetByID(ctx, created.AppointmentUUID)

	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, created.AppointmentUUID, found.AppointmentUUID)
}

func TestReschedule_NotFound(t *testing.T) {
	svc, _, _, _, _ := newService()

	_, err := svc.Reschedule(context.Background(), "missing", 5)

	var notFound *domain.NotFoundError
	require.ErrorAs(t, err, &notFound)
}

func TestReschedule_AlreadyCompleted_ReturnsConflict(t *testing.T) {
	svc, _, _, _, _ := newService()
	ctx := context.Background()
	appointment, _ := svc.Create(ctx, application.CreateInput{InsuredID: "00001", ScheduleID: 1, CountryISO: domain.CountryPE})
	require.NoError(t, svc.Complete(ctx, appointment.AppointmentUUID))

	_, err := svc.Reschedule(ctx, appointment.AppointmentUUID, 5)

	var conflict *domain.ConflictError
	require.ErrorAs(t, err, &conflict)
}

func TestComplete_AppointmentNotFound_NoOps(t *testing.T) {
	svc, _, _, eventStore, notifier := newService()

	err := svc.Complete(context.Background(), "missing")

	require.NoError(t, err)
	assert.Empty(t, eventStore.events)
	assert.Empty(t, notifier.completed)
}

// --- error-path coverage using fakes that return an injected error ---

type erroringStateRepo struct{ *fakeStateRepo; saveErr error }

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
	svc := application.NewAppointmentService(stateRepo, &capturingPublisher{}, &fakeEventStore{}, &capturingNotifier{})

	_, err := svc.Create(context.Background(), application.CreateInput{InsuredID: "00001", ScheduleID: 1, CountryISO: domain.CountryPE})

	assert.ErrorIs(t, err, assert.AnError)
}

func TestCreate_PropagatesPublishError(t *testing.T) {
	stateRepo := newFakeStateRepo()
	publisher := &erroringPublisher{publishErr: assert.AnError}
	svc := application.NewAppointmentService(stateRepo, publisher, &fakeEventStore{}, &capturingNotifier{})

	_, err := svc.Create(context.Background(), application.CreateInput{InsuredID: "00001", ScheduleID: 1, CountryISO: domain.CountryPE})

	assert.ErrorIs(t, err, assert.AnError)
}
