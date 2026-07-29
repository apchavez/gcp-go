package messaging

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"log"
	"net/http"

	"github.com/apchavez/gcp-go/internal/application"
)

// eventarcPubSubEventType is the CloudEvents "ce-type" header Eventarc sets for a
// Pub/Sub-transport trigger (see terraform/eventarc.tf's matching_criteria). Logged for
// observability only - the handler doesn't reject on a mismatch, since Eventarc's binary
// content mode delivery is otherwise structurally identical to a raw Pub/Sub push.
const eventarcPubSubEventType = "google.cloud.pubsub.topic.v1.messagePublished"

// NewConfirmHandler returns an http.Handler for the confirm Cloud Run service - the
// Eventarc-triggered stage B mirroring the AWS sibling's confirmAppointment λ (consuming
// the SQS queue an EventBridge rule routes "AppointmentConfirmed" into). Eventarc delivers
// Pub/Sub-transport events to Cloud Run in CloudEvents binary content mode: the CloudEvents
// attributes travel as ce-* HTTP headers, and the body is the same MessagePublishedData
// JSON shape as a raw Pub/Sub push subscription (see pushEnvelope in worker_handler.go),
// so the parsing logic here is deliberately identical to NewWorkerHandler's.
func NewConfirmHandler(svc *application.AppointmentService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if ceType := r.Header.Get("ce-type"); ceType != "" && ceType != eventarcPubSubEventType {
			log.Printf("confirm: unexpected ce-type header %q (source=%q)", ceType, r.Header.Get("ce-source"))
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "cannot read body", http.StatusBadRequest)
			return
		}

		var envelope pushEnvelope
		if err := json.Unmarshal(body, &envelope); err != nil {
			log.Printf("confirm: malformed push envelope: %v", err)
			http.Error(w, "malformed envelope", http.StatusBadRequest)
			return
		}

		data, err := base64.StdEncoding.DecodeString(envelope.Message.Data)
		if err != nil {
			log.Printf("confirm: malformed message data: %v", err)
			http.Error(w, "malformed data", http.StatusBadRequest)
			return
		}

		var payload appointmentPayload
		if err := json.Unmarshal(data, &payload); err != nil || payload.AppointmentUUID == "" {
			log.Printf("confirm: message missing appointmentUuid, messageId=%s", envelope.Message.MessageID)
			http.Error(w, "missing appointmentUuid", http.StatusBadRequest)
			return
		}

		if err := svc.Complete(r.Context(), payload.AppointmentUUID); err != nil {
			log.Printf("confirm: failed to complete appointment %s: %v", payload.AppointmentUUID, err)
			http.Error(w, "processing failed", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
