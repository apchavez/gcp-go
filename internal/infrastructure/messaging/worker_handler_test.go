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
	"github.com/apchavez/gcp-go/internal/infrastructure/notifications"
)

type capturingRelationalRepo struct{ persisted []domain.Appointment }

func (r *capturingRelationalRepo) Persist(_ context.Context, a domain.Appointment) error {
	r.persisted = append(r.persisted, a)
	return nil
}

type capturingConfirmationPublisher struct{ published []string }

func (p *capturingConfirmationPublisher) PublishConfirmed(_ context.Context, appointmentUUID string) error {
	p.published = append(p.published, appointmentUUID)
	return nil
}

func workerPushBody(t *testing.T, appointmentUUID string) string {
	t.Helper()
	payload, err := json.Marshal(map[string]string{"appointmentUuid": appointmentUUID})
	require.NoError(t, err)
	envelope := map[string]any{
		"subscription": "projects/p/subscriptions/appointment-worker",
		"message": map[string]any{
			"data":      base64.StdEncoding.EncodeToString(payload),
			"messageId": "1",
		},
	}
	body, err := json.Marshal(envelope)
	require.NoError(t, err)
	return string(body)
}

// TestWorkerHandler_PersistsToRelationalStoreAndPublishesConfirmation guards against the
// regression found 2026-07-27 during interview-demo E2E verification: the handler used to
// call svc.Complete (stage B's method) instead of svc.Persist (stage A's), which - because
// cmd/worker wires a real Firestore stateRepo but a noop eventStore/notifier - still flipped
// the appointment to "completed" in Firestore, masking the fact that Cloud SQL was never
// written to and "appointment-confirmed" was never published, so the confirm service and
// SendGrid notification were never exercised at all.
func TestWorkerHandler_PersistsToRelationalStoreAndPublishesConfirmation(t *testing.T) {
	stateRepo := newFakeStateRepo()
	relationalRepo := &capturingRelationalRepo{}
	confirmationPublisher := &capturingConfirmationPublisher{}
	stateRepo.items["uuid-1"] = domain.Appointment{AppointmentUUID: "uuid-1", Status: domain.StatusPending}
	svc := application.NewAppointmentService(stateRepo, noop.Publisher{}, noop.EventStore{}, notifications.NoOpNotifier{}, relationalRepo, confirmationPublisher)

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(workerPushBody(t, "uuid-1")))
	rec := httptest.NewRecorder()

	messaging.NewWorkerHandler(svc)(rec, req)

	assert.Equal(t, http.StatusNoContent, rec.Code)
	require.Len(t, relationalRepo.persisted, 1)
	assert.Equal(t, "uuid-1", relationalRepo.persisted[0].AppointmentUUID)
	require.Len(t, confirmationPublisher.published, 1)
	assert.Equal(t, "uuid-1", confirmationPublisher.published[0])
	assert.Equal(t, domain.StatusPending, stateRepo.items["uuid-1"].Status)
}

func TestWorkerHandler_MalformedBody(t *testing.T) {
	stateRepo := newFakeStateRepo()
	svc := application.NewAppointmentService(stateRepo, noop.Publisher{}, noop.EventStore{}, notifications.NoOpNotifier{}, &capturingRelationalRepo{}, &capturingConfirmationPublisher{})

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("not json"))
	rec := httptest.NewRecorder()

	messaging.NewWorkerHandler(svc)(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestWorkerHandler_MissingAppointmentUuid(t *testing.T) {
	stateRepo := newFakeStateRepo()
	svc := application.NewAppointmentService(stateRepo, noop.Publisher{}, noop.EventStore{}, notifications.NoOpNotifier{}, &capturingRelationalRepo{}, &capturingConfirmationPublisher{})

	envelope := map[string]any{"message": map[string]any{"data": base64.StdEncoding.EncodeToString([]byte("{}"))}}
	body, err := json.Marshal(envelope)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(string(body)))
	rec := httptest.NewRecorder()

	messaging.NewWorkerHandler(svc)(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}
