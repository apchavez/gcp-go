resource "google_service_account" "clinic_scheduling" {
  project      = var.gcp_project_id
  account_id   = "clinic-scheduling-${var.environment}"
  display_name = "Clinic Scheduling API/Worker (${var.environment})"
}

resource "google_project_iam_member" "firestore_user" {
  project = var.gcp_project_id
  role    = "roles/datastore.user"
  member  = "serviceAccount:${google_service_account.clinic_scheduling.email}"
}

resource "google_project_iam_member" "pubsub_publisher" {
  project = var.gcp_project_id
  role    = "roles/pubsub.publisher"
  member  = "serviceAccount:${google_service_account.clinic_scheduling.email}"
}

resource "google_project_iam_member" "pubsub_subscriber" {
  project = var.gcp_project_id
  role    = "roles/pubsub.subscriber"
  member  = "serviceAccount:${google_service_account.clinic_scheduling.email}"
}

resource "google_project_iam_member" "cloudsql_client" {
  project = var.gcp_project_id
  role    = "roles/cloudsql.client"
  member  = "serviceAccount:${google_service_account.clinic_scheduling.email}"
}

# Required for the "appointment-confirmed" Eventarc trigger (eventarc.tf) to receive
# events - reuses the same service account as the rest of this project's compute.
resource "google_project_iam_member" "eventarc_event_receiver" {
  project = var.gcp_project_id
  role    = "roles/eventarc.eventReceiver"
  member  = "serviceAccount:${google_service_account.clinic_scheduling.email}"
}

resource "google_secret_manager_secret_iam_member" "jwt_secret_access" {
  secret_id = google_secret_manager_secret.jwt_secret.id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.clinic_scheduling.email}"
}

resource "google_secret_manager_secret_iam_member" "sendgrid_key_access" {
  secret_id = google_secret_manager_secret.sendgrid_api_key.id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.clinic_scheduling.email}"
}

resource "google_secret_manager_secret_iam_member" "cloudsql_password_access" {
  secret_id = google_secret_manager_secret.cloudsql_password.id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.clinic_scheduling.email}"
}

# IAM writes are eventually consistent - the Cloud Run services below read secrets
# referenced by these bindings the moment they're created, and Terraform's dependency
# graph only guarantees the grant *exists*, not that it has propagated everywhere the
# Cloud Run control plane checks. Without this wait, a fast-creating service (e.g. api,
# which has no slow upstream dependency like the Cloud SQL instance the worker service
# waits on) can hit "Permission denied on secret" even though the binding above already
# applied cleanly. See run 30343433812.
resource "time_sleep" "secret_iam_propagation" {
  depends_on = [
    google_secret_manager_secret_iam_member.jwt_secret_access,
    google_secret_manager_secret_iam_member.sendgrid_key_access,
    google_secret_manager_secret_iam_member.cloudsql_password_access,
  ]
  create_duration = "30s"
}
