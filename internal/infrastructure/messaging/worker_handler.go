package messaging

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"log"
	"net/http"

	"github.com/apchavez/gcp-go/internal/application"
)

// pushEnvelope is the standard Pub/Sub push-subscription HTTP body shape.
type pushEnvelope struct {
	Message struct {
		Data       string            `json:"data"`
		Attributes map[string]string `json:"attributes"`
		MessageID  string            `json:"messageId"`
	} `json:"message"`
	Subscription string `json:"subscription"`
}

type appointmentPayload struct {
	AppointmentUUID string `json:"appointmentUuid"`
}

// NewWorkerHandler returns an http.Handler for the Cloud Run Pub/Sub push subscription
// (single subscription covering both countries - see terraform/pubsub.tf). Mirrors
// AppointmentWorkerBase in the Azure sibling: on failure it
// returns a non-2xx status so Pub/Sub retries/dead-letters the message, matching the
// AWS sibling's SQS-DLQ / Azure sibling's Service Bus FixedDelayRetry semantics.
func NewWorkerHandler(svc *application.AppointmentService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "cannot read body", http.StatusBadRequest)
			return
		}

		var envelope pushEnvelope
		if err := json.Unmarshal(body, &envelope); err != nil {
			log.Printf("worker: malformed push envelope: %v", err)
			http.Error(w, "malformed envelope", http.StatusBadRequest)
			return
		}

		data, err := base64.StdEncoding.DecodeString(envelope.Message.Data)
		if err != nil {
			log.Printf("worker: malformed message data: %v", err)
			http.Error(w, "malformed data", http.StatusBadRequest)
			return
		}

		var payload appointmentPayload
		if err := json.Unmarshal(data, &payload); err != nil || payload.AppointmentUUID == "" {
			log.Printf("worker: message missing appointmentUuid, messageId=%s", envelope.Message.MessageID)
			http.Error(w, "missing appointmentUuid", http.StatusBadRequest)
			return
		}

		// Stage A calls Persist (Cloud SQL write + publish to "appointment-confirmed"),
		// not Complete - Complete is stage B's job (cmd/confirm, Eventarc-triggered off
		// that topic). This handler called svc.Complete for a while by mistake: since
		// cmd/worker wires a real Firestore stateRepo but a noop eventStore/notifier
		// (see cmd/worker/main.go), that bug still flipped the appointment to
		// "completed" in Firestore - which the demo could pass at a glance - while
		// silently never writing to Cloud SQL, never publishing "appointment-confirmed",
		// never invoking the confirm service via Eventarc, and never recording the
		// APPOINTMENT_COMPLETED history event or sending the SendGrid notification.
		// Found 2026-07-27 during interview-demo E2E verification (history stayed stuck
		// on a single APPOINTMENT_CREATED event, and worker/confirm logs never showed
		// any Cloud SQL or Eventarc activity, even though list-by-insured showed
		// "completed").
		if err := svc.Persist(r.Context(), payload.AppointmentUUID); err != nil {
			log.Printf("worker: failed to persist appointment %s: %v", payload.AppointmentUUID, err)
			http.Error(w, "processing failed", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
