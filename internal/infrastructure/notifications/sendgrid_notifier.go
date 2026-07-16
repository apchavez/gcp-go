package notifications

import (
	"context"
	"fmt"
	"log"

	"github.com/sendgrid/sendgrid-go"
	"github.com/sendgrid/sendgrid-go/helpers/mail"

	"github.com/apchavez/gcp-go/internal/domain"
)

// SendGridNotifier implements domain.AppointmentNotifier via SendGrid - GCP has no
// first-party transactional-email service equivalent to AWS SES / Azure Communication
// Services, so SendGrid is the common real-world choice for GCP-hosted apps. Best-effort:
// errors are logged, never returned, matching the AWS/Azure siblings' notifier contract
// (a notification failure must not fail the appointment lifecycle operation).
type SendGridNotifier struct {
	client *sendgrid.Client
	from   string
}

func NewSendGridNotifier(apiKey, fromAddress string) *SendGridNotifier {
	return &SendGridNotifier{client: sendgrid.NewSendClient(apiKey), from: fromAddress}
}

func (n *SendGridNotifier) send(ctx context.Context, a domain.Appointment, subject, body string) {
	if a.ContactEmail == nil || *a.ContactEmail == "" {
		return
	}
	from := mail.NewEmail("Clinic Scheduling", n.from)
	to := mail.NewEmail("", *a.ContactEmail)
	message := mail.NewSingleEmail(from, subject, to, body, "")
	if _, err := n.client.SendWithContext(ctx, message); err != nil {
		log.Printf("sendgrid notifier: failed to send %q for %s: %v", subject, a.AppointmentUUID, err)
	}
}

func (n *SendGridNotifier) NotifyCompleted(ctx context.Context, a domain.Appointment) error {
	n.send(ctx, a, "Appointment completed",
		fmt.Sprintf("Your appointment %s has been completed.", a.AppointmentUUID))
	return nil
}

func (n *SendGridNotifier) NotifyCancelled(ctx context.Context, a domain.Appointment) error {
	n.send(ctx, a, "Appointment cancelled",
		fmt.Sprintf("Your appointment %s has been cancelled.", a.AppointmentUUID))
	return nil
}

func (n *SendGridNotifier) NotifyRescheduled(ctx context.Context, old, updated domain.Appointment) error {
	n.send(ctx, updated, "Appointment rescheduled",
		fmt.Sprintf("Your appointment %s has been rescheduled to a new appointment (%s).", old.AppointmentUUID, updated.AppointmentUUID))
	return nil
}
