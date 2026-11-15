# Eventarc is GCP's equivalent of AWS EventBridge / Azure Event Grid - a managed event
# bus/router, as opposed to a plain Pub/Sub push subscription (used for the
# appointment-created -> worker hop in pubsub.tf). Routes "appointment-confirmed" messages
# to the confirm Cloud Run service, mirroring the AWS sibling's EventBridge rule -> SQS ->
# confirmAppointment λ hop.

resource "google_project_service" "eventarc" {
  project            = var.gcp_project_id
  service            = "eventarc.googleapis.com"
  disable_on_destroy = false # other resources may depend on the API staying enabled; avoid an accidental disable on a routine destroy
}

resource "google_eventarc_trigger" "appointment_confirmed" {
  project  = var.gcp_project_id
  name     = "appointment-confirmed-trigger"
  location = var.gcp_region

  matching_criteria {
    attribute = "type"
    value     = "google.cloud.pubsub.topic.v1.messagePublished"
  }

  transport {
    pubsub {
      topic = google_pubsub_topic.appointment_confirmed.id
    }
  }

  destination {
    cloud_run_service {
      service = google_cloud_run_v2_service.confirm.name
      region  = var.gcp_region
    }
  }

  service_account = google_service_account.clinic_scheduling.email

  depends_on = [
    google_project_service.eventarc,
    google_project_iam_member.eventarc_event_receiver,
    google_cloud_run_v2_service_iam_member.confirm_eventarc_invoker,
  ]
}
