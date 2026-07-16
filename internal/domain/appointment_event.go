package domain

import (
	"time"

	"github.com/google/uuid"
)

const (
	EventAppointmentCreated     = "APPOINTMENT_CREATED"
	EventAppointmentCompleted   = "APPOINTMENT_COMPLETED"
	EventAppointmentCancelled   = "APPOINTMENT_CANCELLED"
	EventAppointmentRescheduled = "APPOINTMENT_RESCHEDULED"
)

type AppointmentEvent struct {
	EventID         string `json:"eventId" firestore:"eventId"`
	AppointmentUUID string `json:"appointmentUuid" firestore:"appointmentUuid"`
	EventType       string `json:"eventType" firestore:"eventType"`
	InsuredID       string `json:"insuredId" firestore:"insuredId"`
	ScheduleID      int    `json:"scheduleId" firestore:"scheduleId"`
	CountryISO      string `json:"countryISO" firestore:"countryISO"`
	Status          string `json:"status" firestore:"status"`
	OccurredAt      string `json:"occurredAt" firestore:"occurredAt"`
}

// NewAppointmentEvent mirrors makeAppointmentEvent from the AWS TypeScript sibling.
func NewAppointmentEvent(eventType string, a Appointment) AppointmentEvent {
	return AppointmentEvent{
		EventID:         uuid.NewString(),
		AppointmentUUID: a.AppointmentUUID,
		EventType:       eventType,
		InsuredID:       a.InsuredID,
		ScheduleID:      a.ScheduleID,
		CountryISO:      a.CountryISO,
		Status:          a.Status,
		OccurredAt:      time.Now().UTC().Format(time.RFC3339),
	}
}
