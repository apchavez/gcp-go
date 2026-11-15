# Single topic + single shared push subscription, feeding the Cloud Run worker that marks
# appointments completed. Mirrors the trimmed-down AWS sibling (one SNS topic -> one SQS
# queue -> one consumer) and Azure sibling (one Service Bus topic -> one subscriber).

resource "google_pubsub_topic" "appointment_created" {
  project = var.gcp_project_id
  name    = "appointment-created"
}

resource "google_pubsub_subscription" "worker" {
  project = var.gcp_project_id
  name    = "appointment-worker"
  topic   = google_pubsub_topic.appointment_created.name

  # Root-cause fix (2026-07-27, found during interview-demo E2E verification): this
  # push_config previously hand-built the URL as "https://<name>-<region>.run.app/",
  # which doesn't match Cloud Run's actual assigned URI (it needs the project-number/hash
  # segment, e.g. "https://clinic-scheduling-worker-dev-4hn6pn4n4q-uc.a.run.app") - every
  # push to the worker 404'd silently (Pub/Sub retried into the DLQ after 5 attempts,
  # worker logs never showed a single incoming request). It was also missing the
  # oidc_token block the comment on worker_pubsub_invoker (cloudrun.tf) already assumed
  # existed - the worker only grants roles/run.invoker to the clinic_scheduling service
  # account, not allUsers, so even a correct URL would still have been rejected with 403.
  push_config {
    push_endpoint = google_cloud_run_v2_service.worker.uri

    oidc_token {
      service_account_email = google_service_account.clinic_scheduling.email
      audience              = google_cloud_run_v2_service.worker.uri
    }
  }

  ack_deadline_seconds       = 30
  message_retention_duration = "604800s" # 7 days, matching the AWS sibling's SQS DLQ retention intent

  retry_policy {
    minimum_backoff = "10s"
    maximum_backoff = "60s"
  }

  dead_letter_policy {
    dead_letter_topic     = google_pubsub_topic.dead_letter.id
    max_delivery_attempts = 5
  }
}

resource "google_pubsub_topic" "dead_letter" {
  project = var.gcp_project_id
  name    = "appointment-created-dlq"
}

# Stage-A -> stage-B confirmation hop, mirroring the AWS sibling's EventBridge bus + rule:
# the worker (stage A) publishes here after persisting to Cloud SQL, and an Eventarc trigger
# (see eventarc.tf) - not a plain push subscription - routes messages on this topic to the
# confirm Cloud Run service (stage B), which marks the aggregate COMPLETED and notifies.
resource "google_pubsub_topic" "appointment_confirmed" {
  project = var.gcp_project_id
  name    = "appointment-confirmed"
}
