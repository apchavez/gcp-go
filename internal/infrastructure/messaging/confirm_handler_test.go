package messaging_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/apchavez/gcp-go/internal/application"
	"github.com/apchavez/gcp-go/internal/domain"
	"github.com/apchavez/gcp-go/internal/infrastructure/messaging"
	"github.com/apchavez/gcp-go/internal/infrastructure/noop"
)

type fakeStateRepo struct {
	items   map[string]domain.Appointment
	findErr error
}

func newFakeStateRepo() *fakeStateRepo { return &fakeStateRepo{items: map[string]domain.Appointment{}} }

func (f *fakeStateRepo) Save(_ context.Context, a domain.Appointment) error {
	f.items[a.AppointmentUUID] = a
	return nil
}
func (f *fakeStateRepo) FindByID(_ context.Context, id string) (*domain.Appointment, error) {
	if f.findErr != nil {
		return nil, f.findErr
	}
	a, ok := f.items[id]
	if !ok {
		return nil, nil
	}
	return &a, nil
}
func (f *fakeStateRepo) MarkCompleted(_ context.Context, id string) error {
	a := f.items[id]
	a.Status = domain.StatusCompleted
	f.items[id] = a
	return nil
}
func (f *fakeStateRepo) ListByInsured(_ context.Context, _ string, _ int, _ string) (domain.Page, error) {
	return domain.Page{}, nil
}

type fakeEventStore struct{ events []domain.AppointmentEvent }

func (s *fakeEventStore) Append(_ context.Context, e domain.AppointmentEvent) error {
	s.events = append(s.events, e)
	return nil
}
func (s *fakeEventStore) FindByAppointmentID(_ context.Context, _ string) ([]domain.AppointmentEvent, error) {
	return nil, nil
}

type capturingNotifier struct{ completed []domain.Appointment }

func (n *capturingNotifier) NotifyCompleted(_ context.Context, a domain.Appointment) error {
	n.completed = append(n.completed, a)
	return nil
}

func pushBody(t *testing.T, appointmentUUID string) string {
	t.Helper()
	payload, err := json.Marshal(map[string]string{"appointmentUuid": appointmentUUID})
	require.NoError(t, err)
	envelope := map[string]any{
		"subscription": "projects/p/subscriptions/appointment-confirmed-trigger",
		"message": map[string]any{
			"data":      base64.StdEncoding.EncodeToString(payload),
			"messageId": "1",
		},
	}
	body, err := json.Marshal(envelope)
	require.NoError(t, err)
	return string(body)
}

func TestConfirmHandler_CompletesAppointment(t *testing.T) {
	stateRepo := newFakeStateRepo()
	eventStore := &fakeEventStore{}
	notifier := &capturingNotifier{}
	stateRepo.items["uuid-1"] = domain.Appointment{AppointmentUUID: "uuid-1", Status: domain.StatusPending}
	svc := application.NewAppointmentService(stateRepo, noop.Publisher{}, eventStore, notifier, noop.RelationalRepository{}, noop.ConfirmationPublisher{})

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(pushBody(t, "uuid-1")))
	req.Header.Set("ce-type", "google.cloud.pubsub.topic.v1.messagePublished")
	rec := httptest.NewRecorder()

	messaging.NewConfirmHandler(svc)(rec, req)

	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Equal(t, domain.StatusCompleted, stateRepo.items["uuid-1"].Status)
	assert.Len(t, notifier.completed, 1)
}

func TestConfirmHandler_MalformedBody(t *testing.T) {
	stateRepo := newFakeStateRepo()
	svc := application.NewAppointmentService(stateRepo, noop.Publisher{}, &fakeEventStore{}, &capturingNotifier{}, noop.RelationalRepository{}, noop.ConfirmationPublisher{})

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("not json"))
	rec := httptest.NewRecorder()

	messaging.NewConfirmHandler(svc)(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestConfirmHandler_UnexpectedCeType(t *testing.T) {
	stateRepo := newFakeStateRepo()
	eventStore := &fakeEventStore{}
	notifier := &capturingNotifier{}
	stateRepo.items["uuid-1"] = domain.Appointment{AppointmentUUID: "uuid-1", Status: domain.StatusPending}
	svc := application.NewAppointmentService(stateRepo, noop.Publisher{}, eventStore, notifier, noop.RelationalRepository{}, noop.ConfirmationPublisher{})

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(pushBody(t, "uuid-1")))
	req.Header.Set("ce-type", "some.other.event.type")
	req.Header.Set("ce-source", "unexpected-source")
	rec := httptest.NewRecorder()

	messaging.NewConfirmHandler(svc)(rec, req)

	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Equal(t, domain.StatusCompleted, stateRepo.items["uuid-1"].Status)
}

func TestConfirmHandler_MalformedMessageData(t *testing.T) {
	stateRepo := newFakeStateRepo()
	svc := application.NewAppointmentService(stateRepo, noop.Publisher{}, &fakeEventStore{}, &capturingNotifier{}, noop.RelationalRepository{}, noop.ConfirmationPublisher{})

	envelope := map[string]any{"message": map[string]any{"data": "not-valid-base64!!", "messageId": "1"}}
	body, err := json.Marshal(envelope)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(string(body)))
	rec := httptest.NewRecorder()

	messaging.NewConfirmHandler(svc)(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestConfirmHandler_CompleteFails(t *testing.T) {
	stateRepo := newFakeStateRepo()
	stateRepo.findErr = assert.AnError
	svc := application.NewAppointmentService(stateRepo, noop.Publisher{}, &fakeEventStore{}, &capturingNotifier{}, noop.RelationalRepository{}, noop.ConfirmationPublisher{})

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(pushBody(t, "uuid-1")))
	rec := httptest.NewRecorder()

	messaging.NewConfirmHandler(svc)(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestConfirmHandler_MissingAppointmentUuid(t *testing.T) {
	stateRepo := newFakeStateRepo()
	svc := application.NewAppointmentService(stateRepo, noop.Publisher{}, &fakeEventStore{}, &capturingNotifier{}, noop.RelationalRepository{}, noop.ConfirmationPublisher{})

	envelope := map[string]any{"message": map[string]any{"data": base64.StdEncoding.EncodeToString([]byte("{}"))}}
	body, err := json.Marshal(envelope)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(string(body)))
	rec := httptest.NewRecorder()

	messaging.NewConfirmHandler(svc)(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}
