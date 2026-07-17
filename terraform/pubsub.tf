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

# Single push subscription for both countries. The "country" message attribute doesn't
# route to a different downstream resource here - PE and CL share the same Cloud SQL
# instance (see cloudsql_repo.go) and the worker handler doesn't even read the attribute -
# unlike the AWS sibling's per-country RDS instances, there's no reason to split into
# per-country subscriptions pushing to the same Cloud Run endpoint.
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
