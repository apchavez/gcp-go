# Three topics mirroring the AWS sibling's SNS topics / Azure sibling's Service Bus topics
# (appointment-created / appointment-completed / appointment-cancelled). The Azure Bicep
# originally omitted the cancelled topic despite the code referencing it - all three are
# provisioned here from the start.

resource "google_pubsub_topic" "appointment_created" {
  project = var.gcp_project_id
  name    = "appointment-created"
}

resource "google_pubsub_topic" "appointment_completed" {
  project = var.gcp_project_id
  name    = "appointment-completed"
}

resource "google_pubsub_topic" "appointment_cancelled" {
  project = var.gcp_project_id
  name    = "appointment-cancelled"
}

# Per-country push subscriptions on "appointment-created", filtered by the "country"
# message attribute - the native Pub/Sub equivalent of the Azure sibling's Service Bus
# SQL subscription filters (sys.Subject='PE'/'CL').
resource "google_pubsub_subscription" "worker_pe" {
  project = var.gcp_project_id
  name    = "pe-worker"
  topic   = google_pubsub_topic.appointment_created.name

  filter = "attributes.country = \"PE\""

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

resource "google_pubsub_subscription" "worker_cl" {
  project = var.gcp_project_id
  name    = "cl-worker"
  topic   = google_pubsub_topic.appointment_created.name

  filter = "attributes.country = \"CL\""

  push_config {
    push_endpoint = "https://${google_cloud_run_v2_service.worker.name}-${var.gcp_region}.run.app/"
  }

  ack_deadline_seconds       = 30
  message_retention_duration = "604800s"

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
