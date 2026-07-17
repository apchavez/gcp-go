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

		if err := svc.Complete(r.Context(), payload.AppointmentUUID); err != nil {
			log.Printf("worker: failed to complete appointment %s: %v", payload.AppointmentUUID, err)
			http.Error(w, "processing failed", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
