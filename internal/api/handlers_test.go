package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/apchavez/gcp-go/internal/api"
	"github.com/apchavez/gcp-go/internal/application"
	"github.com/apchavez/gcp-go/internal/domain"
	"github.com/apchavez/gcp-go/internal/infrastructure/auth"
	"github.com/apchavez/gcp-go/internal/infrastructure/noop"
	"github.com/apchavez/gcp-go/internal/infrastructure/notifications"
	"github.com/apchavez/gcp-go/internal/shared"
)

const testSecret = "test-only-secret-do-not-use-in-production"

type fakeStateRepo struct {
	items        map[string]domain.Appointment
	saveErr      error
	findErr      error
	listErr      error
	lastPageSize int
	lastCursor   string
}

func newFakeStateRepo() *fakeStateRepo {
	return &fakeStateRepo{items: map[string]domain.Appointment{}}
}
func (f *fakeStateRepo) Save(_ context.Context, a domain.Appointment) error {
	if f.saveErr != nil {
		return f.saveErr
	}
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
func (f *fakeStateRepo) MarkCompleted(_ context.Context, id string) error { return nil }
func (f *fakeStateRepo) ListByInsured(_ context.Context, insuredID string, pageSize int, cursor string) (domain.Page, error) {
	f.lastPageSize = pageSize
	f.lastCursor = cursor
	if f.listErr != nil {
		return domain.Page{}, f.listErr
	}
	var items []domain.Appointment
	for _, a := range f.items {
		if a.InsuredID == insuredID {
			items = append(items, a)
		}
	}
	return domain.Page{Items: items}, nil
}

func newTestHandlers(t *testing.T, stateRepo *fakeStateRepo) *api.Handlers {
	t.Helper()
	svc := application.NewAppointmentService(stateRepo, noop.Publisher{}, noop.EventStore{}, notifications.NoOpNotifier{}, noop.RelationalRepository{}, noop.ConfirmationPublisher{})
	return api.NewHandlers(svc)
}

func authedRequest(t *testing.T, method, target, body, sub, role string) *http.Request {
	t.Helper()
	t.Setenv("JWT_SECRET", testSecret)
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	if sub != "" {
		token := auth.Sign(sub, role, testSecret, time.Hour)
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return req
}

func withURLParam(req *http.Request, key, value string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, value)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

func TestCreateAppointment_EmptyBody(t *testing.T) {
	h := newTestHandlers(t, newFakeStateRepo())
	req := authedRequest(t, http.MethodPost, "/appointments", "", "00001", "insured")
	rec := httptest.NewRecorder()

	h.CreateAppointment(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestCreateAppointment_InvalidJSON(t *testing.T) {
	h := newTestHandlers(t, newFakeStateRepo())
	req := authedRequest(t, http.MethodPost, "/appointments", "not json", "00001", "insured")
	rec := httptest.NewRecorder()

	h.CreateAppointment(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestCreateAppointment_InvalidInsuredIDFormat(t *testing.T) {
	h := newTestHandlers(t, newFakeStateRepo())
	req := authedRequest(t, http.MethodPost, "/appointments", `{"insuredId":"abc","scheduleId":1}`, "00001", "insured")
	rec := httptest.NewRecorder()

	h.CreateAppointment(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestCreateAppointment_Unauthenticated(t *testing.T) {
	h := newTestHandlers(t, newFakeStateRepo())
	req := authedRequest(t, http.MethodPost, "/appointments", `{"insuredId":"00001","scheduleId":1}`, "", "")
	rec := httptest.NewRecorder()

	h.CreateAppointment(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestCreateAppointment_InsuredCannotBookForSomeoneElse(t *testing.T) {
	h := newTestHandlers(t, newFakeStateRepo())
	req := authedRequest(t, http.MethodPost, "/appointments", `{"insuredId":"00002","scheduleId":1}`, "00001", "insured")
	rec := httptest.NewRecorder()

	h.CreateAppointment(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestCreateAppointment_Success(t *testing.T) {
	h := newTestHandlers(t, newFakeStateRepo())
	req := authedRequest(t, http.MethodPost, "/appointments", `{"insuredId":"00001","scheduleId":1}`, "00001", "insured")
	rec := httptest.NewRecorder()

	h.CreateAppointment(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}
}

func TestCreateAppointment_ServiceErrorReturns500(t *testing.T) {
	stateRepo := newFakeStateRepo()
	stateRepo.saveErr = context.DeadlineExceeded
	h := newTestHandlers(t, stateRepo)
	req := authedRequest(t, http.MethodPost, "/appointments", `{"insuredId":"00001","scheduleId":1}`, "00001", "insured")
	rec := httptest.NewRecorder()

	h.CreateAppointment(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestListByInsured_ServiceErrorReturns500(t *testing.T) {
	stateRepo := newFakeStateRepo()
	stateRepo.listErr = context.DeadlineExceeded
	h := newTestHandlers(t, stateRepo)
	req := authedRequest(t, http.MethodGet, "/appointments/00001", "", "00001", "insured")
	req = withURLParam(req, "insuredId", "00001")
	rec := httptest.NewRecorder()

	h.ListByInsured(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestListByInsured_PageSizeQueryParam(t *testing.T) {
	tests := []struct {
		name  string
		query string
		want  int
	}{
		{"empty defaults to 20", "", 20},
		{"non-numeric defaults to 20", "?pageSize=abc", 20},
		{"zero defaults to 20", "?pageSize=0", 20},
		{"above max defaults to 20", "?pageSize=1000", 20},
		{"valid custom value is honored", "?pageSize=5", 5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stateRepo := newFakeStateRepo()
			h := newTestHandlers(t, stateRepo)
			req := authedRequest(t, http.MethodGet, "/appointments/00001"+tt.query, "", "00001", "insured")
			req = withURLParam(req, "insuredId", "00001")
			rec := httptest.NewRecorder()

			h.ListByInsured(rec, req)

			if stateRepo.lastPageSize != tt.want {
				t.Fatalf("pageSize = %d, want %d", stateRepo.lastPageSize, tt.want)
			}
		})
	}
}

func TestGetAppointmentHistory_ServiceErrorReturns500(t *testing.T) {
	stateRepo := newFakeStateRepo()
	stateRepo.findErr = context.DeadlineExceeded
	h := newTestHandlers(t, stateRepo)
	req := authedRequest(t, http.MethodGet, "/appointments/uuid-1/history", "", "00001", "insured")
	req = withURLParam(req, "appointmentUuid", "uuid-1")
	rec := httptest.NewRecorder()

	h.GetAppointmentHistory(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestListByInsured_InvalidInsuredIDFormat(t *testing.T) {
	h := newTestHandlers(t, newFakeStateRepo())
	req := authedRequest(t, http.MethodGet, "/appointments/abc", "", "00001", "insured")
	req = withURLParam(req, "insuredId", "abc")
	rec := httptest.NewRecorder()

	h.ListByInsured(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestListByInsured_Forbidden(t *testing.T) {
	h := newTestHandlers(t, newFakeStateRepo())
	req := authedRequest(t, http.MethodGet, "/appointments/00002", "", "00001", "insured")
	req = withURLParam(req, "insuredId", "00002")
	rec := httptest.NewRecorder()

	h.ListByInsured(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestListByInsured_Success(t *testing.T) {
	h := newTestHandlers(t, newFakeStateRepo())
	req := authedRequest(t, http.MethodGet, "/appointments/00001", "", "00001", "insured")
	req = withURLParam(req, "insuredId", "00001")
	rec := httptest.NewRecorder()

	h.ListByInsured(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestGetAppointmentHistory_Unauthenticated(t *testing.T) {
	h := newTestHandlers(t, newFakeStateRepo())
	req := authedRequest(t, http.MethodGet, "/appointments/uuid-1/history", "", "", "")
	req = withURLParam(req, "appointmentUuid", "uuid-1")
	rec := httptest.NewRecorder()

	h.GetAppointmentHistory(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestGetAppointmentHistory_NotFound(t *testing.T) {
	h := newTestHandlers(t, newFakeStateRepo())
	req := authedRequest(t, http.MethodGet, "/appointments/uuid-missing/history", "", "00001", "insured")
	req = withURLParam(req, "appointmentUuid", "uuid-missing")
	rec := httptest.NewRecorder()

	h.GetAppointmentHistory(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestGetAppointmentHistory_ForbiddenForOtherInsured(t *testing.T) {
	stateRepo := newFakeStateRepo()
	stateRepo.items["uuid-1"] = domain.Appointment{AppointmentUUID: "uuid-1", InsuredID: "00002"}
	h := newTestHandlers(t, stateRepo)
	req := authedRequest(t, http.MethodGet, "/appointments/uuid-1/history", "", "00001", "insured")
	req = withURLParam(req, "appointmentUuid", "uuid-1")
	rec := httptest.NewRecorder()

	h.GetAppointmentHistory(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestGetAppointmentHistory_Success(t *testing.T) {
	stateRepo := newFakeStateRepo()
	stateRepo.items["uuid-1"] = domain.Appointment{AppointmentUUID: "uuid-1", InsuredID: "00001"}
	h := newTestHandlers(t, stateRepo)
	req := authedRequest(t, http.MethodGet, "/appointments/uuid-1/history", "", "00001", "insured")
	req = withURLParam(req, "appointmentUuid", "uuid-1")
	rec := httptest.NewRecorder()

	h.GetAppointmentHistory(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestHealth_Up(t *testing.T) {
	h := newTestHandlers(t, newFakeStateRepo())
	handler := h.Health(func(r *http.Request) shared.HealthStatus {
		return shared.NewHealthStatus(map[string]string{"firestore": shared.HealthUp})
	})
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var body shared.HealthStatus
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Status != shared.HealthUp {
		t.Fatalf("status field = %q, want %q", body.Status, shared.HealthUp)
	}
}

func TestHealth_Down(t *testing.T) {
	h := newTestHandlers(t, newFakeStateRepo())
	handler := h.Health(func(r *http.Request) shared.HealthStatus {
		return shared.NewHealthStatus(map[string]string{"firestore": shared.HealthDown})
	})
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
}
