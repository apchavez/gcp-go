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

  push_config {
    push_endpoint = "https://${google_cloud_run_v2_service.worker.name}-${var.gcp_region}.run.app/"
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
