package notifications

import (
	"context"

	"github.com/apchavez/gcp-go/internal/domain"
)

// NoOpNotifier is used when no SENDGRID_API_KEY is configured, mirroring the AWS/Azure
// siblings' NoOpAppointmentNotifier fallback.
type NoOpNotifier struct{}

func (NoOpNotifier) NotifyCompleted(ctx context.Context, a domain.Appointment) error   { return nil }
func (NoOpNotifier) NotifyCancelled(ctx context.Context, a domain.Appointment) error   { return nil }
func (NoOpNotifier) NotifyRescheduled(ctx context.Context, old, updated domain.Appointment) error {
	return nil
}
