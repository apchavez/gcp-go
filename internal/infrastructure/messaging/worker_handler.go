package messaging

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"log/slog"
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
			slog.Error("worker: malformed push envelope", "error", err)
			http.Error(w, "malformed envelope", http.StatusBadRequest)
			return
		}

		data, err := base64.StdEncoding.DecodeString(envelope.Message.Data)
		if err != nil {
			slog.Error("worker: malformed message data", "error", err)
			http.Error(w, "malformed data", http.StatusBadRequest)
			return
		}

		var payload appointmentPayload
		if err := json.Unmarshal(data, &payload); err != nil || payload.AppointmentUUID == "" {
			slog.Error("worker: message missing appointmentUuid", "messageId", envelope.Message.MessageID)
			http.Error(w, "missing appointmentUuid", http.StatusBadRequest)
			return
		}

		slog.Info("worker: persisting appointment", "appointmentUuid", payload.AppointmentUUID, "messageId", envelope.Message.MessageID)
		if err := svc.Persist(r.Context(), payload.AppointmentUUID); err != nil {
			slog.Error("worker: failed to persist appointment", "appointmentUuid", payload.AppointmentUUID, "error", err)
			http.Error(w, "processing failed", http.StatusInternalServerError)
			return
		}

		slog.Info("worker: appointment persisted", "appointmentUuid", payload.AppointmentUUID)
		w.WriteHeader(http.StatusNoContent)
	}
}
