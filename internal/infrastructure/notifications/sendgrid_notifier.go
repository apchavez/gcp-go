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
	resp, err := n.client.SendWithContext(ctx, message)
	if err != nil {
		log.Printf("sendgrid notifier: failed to send %q for %s: %v", subject, a.AppointmentUUID, err)
		return
	}
	// The sendgrid-go client only returns a Go error for transport-level failures - an
	// auth/validation rejection (e.g. an invalid or placeholder API key) comes back as a
	// non-2xx HTTP response with err == nil, which this used to silently ignore, making
	// every such failure invisible. Found 2026-07-27 during interview-demo E2E verification
	// (this dev deployment's SENDGRID_API_KEY is a placeholder - see terraform/secrets.tf).
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Printf("sendgrid notifier: %q for %s rejected, status=%d body=%s", subject, a.AppointmentUUID, resp.StatusCode, resp.Body)
	}
}

func (n *SendGridNotifier) NotifyCompleted(ctx context.Context, a domain.Appointment) error {
	n.send(ctx, a, "Appointment completed",
		fmt.Sprintf("Your appointment %s has been completed.", a.AppointmentUUID))
	return nil
}
