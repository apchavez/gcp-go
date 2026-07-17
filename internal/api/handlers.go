// Package api contains the HTTP handler layer - route validation, auth/ownership checks,
// and error-to-status-code mapping, mirroring the AWS TypeScript sibling's
// src/api/lambda/appointment.ts handler-for-handler.
package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"regexp"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/apchavez/gcp-go/internal/application"
	"github.com/apchavez/gcp-go/internal/domain"
	"github.com/apchavez/gcp-go/internal/infrastructure/auth"
	"github.com/apchavez/gcp-go/internal/shared"
)

var insuredIDRegex = regexp.MustCompile(`^\d{5}$`)

const (
	defaultPageSize = 20
	maxPageSize     = 100
)

type Handlers struct {
	svc *application.AppointmentService
}

func NewHandlers(svc *application.AppointmentService) *Handlers {
	return &Handlers{svc: svc}
}

type createRequest struct {
	InsuredID    string  `json:"insuredId"`
	ScheduleID   int     `json:"scheduleId"`
	CountryISO   string  `json:"countryISO"`
	ContactEmail *string `json:"contactEmail"`
}

func (h *Handlers) CreateAppointment(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil || len(body) == 0 {
		shared.Bad(w, "Required body")
		return
	}
	var req createRequest
	if err := json.Unmarshal(body, &req); err != nil {
		shared.Bad(w, "Invalid body (JSON)")
		return
	}

	if req.InsuredID == "" || req.ScheduleID == 0 || req.CountryISO == "" {
		shared.Bad(w, "insuredId, scheduleId and countryISO are required")
		return
	}
	if !insuredIDRegex.MatchString(req.InsuredID) {
		shared.Bad(w, "insuredId must be 5 digits")
		return
	}
	if !domain.IsSupportedCountry(req.CountryISO) {
		shared.Bad(w, "countryISO must be 'PE' or 'CL'")
		return
	}
	if req.ScheduleID < 1 {
		shared.Bad(w, "scheduleId must be a positive integer")
		return
	}

	claims, ok := auth.Authenticate(r)
	if !ok {
		shared.Forbidden(w, "")
		return
	}
	if claims.Role == "insured" && req.InsuredID != claims.Sub {
		shared.Forbidden(w, "insured can only book appointments for themselves")
		return
	}

	appointment, err := h.svc.Create(r.Context(), application.CreateInput{
		InsuredID:    req.InsuredID,
		ScheduleID:   req.ScheduleID,
		CountryISO:   req.CountryISO,
		ContactEmail: req.ContactEmail,
	})
	if err != nil {
		shared.Internal(w, "")
		return
	}
	shared.Created(w, appointment)
}

func (h *Handlers) ListByInsured(w http.ResponseWriter, r *http.Request) {
	insuredID := chi.URLParam(r, "insuredId")
	if !insuredIDRegex.MatchString(insuredID) {
		shared.Bad(w, "insuredId must be 5 digits")
		return
	}

	claims, ok := auth.Authenticate(r)
	if !ok {
		shared.Forbidden(w, "")
		return
	}
	if claims.Role == "insured" && insuredID != claims.Sub {
		shared.Forbidden(w, "insured can only view their own appointments")
		return
	}

	pageSize := parsePageSize(r.URL.Query().Get("pageSize"))
	cursor := r.URL.Query().Get("cursor")

	page, err := h.svc.ListByInsured(r.Context(), insuredID, pageSize, cursor)
	if err != nil {
		shared.Internal(w, "")
		return
	}
	shared.OK(w, page)
}

func (h *Handlers) CancelAppointment(w http.ResponseWriter, r *http.Request) {
	appointmentUUID := chi.URLParam(r, "appointmentUuid")

	claims, ok := auth.Authenticate(r)
	if !ok {
		shared.Forbidden(w, "")
		return
	}

	appointment, err := h.svc.GetByID(r.Context(), appointmentUUID)
	if err != nil {
		shared.Internal(w, "")
		return
	}
	if appointment == nil {
		shared.NotFound(w, "Appointment not found: "+appointmentUUID)
		return
	}
	if claims.Role == "insured" && appointment.InsuredID != claims.Sub {
		shared.Forbidden(w, "insured can only cancel their own appointments")
		return
	}

	if err := h.svc.Cancel(r.Context(), appointmentUUID); err != nil {
		mapDomainError(w, err, "Internal error cancelling appointment")
		return
	}
	shared.OK(w, map[string]string{"message": "Appointment cancelled", "appointmentUuid": appointmentUUID})
}

type rescheduleRequest struct {
	NewScheduleID int `json:"newScheduleId"`
}

func (h *Handlers) RescheduleAppointment(w http.ResponseWriter, r *http.Request) {
	appointmentUUID := chi.URLParam(r, "appointmentUuid")

	body, err := io.ReadAll(r.Body)
	var req rescheduleRequest
	if err != nil || len(body) == 0 || json.Unmarshal(body, &req) != nil {
		shared.Bad(w, "Request body is required")
		return
	}
	if req.NewScheduleID < 1 {
		shared.Bad(w, "newScheduleId (integer >= 1) is required")
		return
	}

	claims, ok := auth.Authenticate(r)
	if !ok {
		shared.Forbidden(w, "")
		return
	}

	appointment, err := h.svc.GetByID(r.Context(), appointmentUUID)
	if err != nil {
		shared.Internal(w, "")
		return
	}
	if appointment == nil {
		shared.NotFound(w, "Appointment not found: "+appointmentUUID)
		return
	}
	if claims.Role == "insured" && appointment.InsuredID != claims.Sub {
		shared.Forbidden(w, "insured can only reschedule their own appointments")
		return
	}

	newAppointment, err := h.svc.Reschedule(r.Context(), appointmentUUID, req.NewScheduleID)
	if err != nil {
		mapDomainError(w, err, "Internal error rescheduling appointment")
		return
	}
	shared.Accepted(w, map[string]any{
		"message":            "Appointment rescheduled",
		"newAppointmentUuid": newAppointment.AppointmentUUID,
		"newScheduleId":      req.NewScheduleID,
	})
}

// GetAppointmentHistory looks up the appointment first and checks ownership against its
// own insuredId - unlike the pre-fix Java/Python history handlers (which derived the owner
// from events[0].insuredId and let a zero-event appointment bypass the 403 entirely), this
// matches the corrected pattern the Azure Python port's GetAppointmentHistoryUseCase now uses.
func (h *Handlers) GetAppointmentHistory(w http.ResponseWriter, r *http.Request) {
	appointmentUUID := chi.URLParam(r, "appointmentUuid")

	claims, ok := auth.Authenticate(r)
	if !ok {
		shared.Forbidden(w, "")
		return
	}

	appointment, err := h.svc.GetByID(r.Context(), appointmentUUID)
	if err != nil {
		shared.Internal(w, "")
		return
	}
	if appointment == nil {
		shared.NotFound(w, "Appointment not found: "+appointmentUUID)
		return
	}
	if claims.Role == "insured" && appointment.InsuredID != claims.Sub {
		shared.Forbidden(w, "insured can only view their own appointment history")
		return
	}

	events, err := h.svc.GetHistory(r.Context(), appointmentUUID)
	if err != nil {
		shared.Internal(w, "")
		return
	}
	shared.OK(w, events)
}

func (h *Handlers) Health(healthCheck func(r *http.Request) shared.HealthStatus) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		status := healthCheck(r)
		code := http.StatusOK
		if status.Status != shared.HealthUp {
			code = http.StatusServiceUnavailable
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		_ = json.NewEncoder(w).Encode(status)
	}
}

func mapDomainError(w http.ResponseWriter, err error, fallback string) {
	var notFound *domain.NotFoundError
	var conflict *domain.ConflictError
	switch {
	case errors.As(err, &notFound):
		shared.NotFound(w, notFound.Message)
	case errors.As(err, &conflict):
		shared.Conflict(w, conflict.Message)
	default:
		shared.Internal(w, fallback)
	}
}

func parsePageSize(raw string) int {
	if raw == "" {
		return defaultPageSize
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 || n > maxPageSize {
		return defaultPageSize
	}
	return n
}
