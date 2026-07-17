package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/apchavez/gcp-go/internal/domain"
)

func TestIsSupportedCountry(t *testing.T) {
	cases := []struct {
		country string
		want    bool
	}{
		{"PE", true},
		{"CL", true},
		{"US", false},
		{"", false},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.want, domain.IsSupportedCountry(tc.country), tc.country)
	}
}

func TestNewAppointmentEvent(t *testing.T) {
	appointment := domain.Appointment{
		AppointmentUUID: "abc-123",
		InsuredID:       "00001",
		ScheduleID:      42,
		CountryISO:      domain.CountryPE,
		Status:          domain.StatusPending,
	}
	event := domain.NewAppointmentEvent(domain.EventAppointmentCreated, appointment)

	assert.NotEmpty(t, event.EventID)
	assert.Equal(t, appointment.AppointmentUUID, event.AppointmentUUID)
	assert.Equal(t, domain.EventAppointmentCreated, event.EventType)
	assert.Equal(t, appointment.InsuredID, event.InsuredID)
	assert.Equal(t, appointment.ScheduleID, event.ScheduleID)
	assert.Equal(t, appointment.CountryISO, event.CountryISO)
	assert.Equal(t, appointment.Status, event.Status)
	assert.NotEmpty(t, event.OccurredAt)
}

func TestErrors(t *testing.T) {
	nf := &domain.NotFoundError{Message: "not found"}
	assert.Equal(t, "not found", nf.Error())

	c := &domain.ConflictError{Message: "conflict"}
	assert.Equal(t, "conflict", c.Error())
}
