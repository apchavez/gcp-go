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
