package domain

// Status values mirror the AWS TypeScript sibling's lowercase string literals exactly,
// since the JSON wire format is meant to be interoperable across all clinic-scheduling siblings.
const (
	StatusPending   = "pending"
	StatusCompleted = "completed"
)

type Appointment struct {
	AppointmentUUID string  `json:"appointmentUuid" firestore:"appointmentUuid"`
	InsuredID       string  `json:"insuredId" firestore:"insuredId"`
	ScheduleID      int     `json:"scheduleId" firestore:"scheduleId"`
	Status          string  `json:"status" firestore:"status"`
	CreatedAt       string  `json:"createdAt" firestore:"createdAt"`
	UpdatedAt       string  `json:"updatedAt" firestore:"updatedAt"`
	ContactEmail    *string `json:"contactEmail,omitempty" firestore:"contactEmail,omitempty"`
}
