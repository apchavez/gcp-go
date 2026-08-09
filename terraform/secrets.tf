resource "google_secret_manager_secret" "jwt_secret" {
  project   = var.gcp_project_id
  secret_id = "clinic-scheduling-jwt-secret-${var.environment}"
  replication {
    auto {}
  }
}

resource "google_secret_manager_secret_version" "jwt_secret" {
  secret      = google_secret_manager_secret.jwt_secret.id
  secret_data = var.jwt_secret
}

resource "google_secret_manager_secret" "sendgrid_api_key" {
  project   = var.gcp_project_id
  secret_id = "clinic-scheduling-sendgrid-key-${var.environment}"
  replication {
    auto {}
  }
}

resource "google_secret_manager_secret_version" "sendgrid_api_key" {
  secret = google_secret_manager_secret.sendgrid_api_key.id
  # Secret Manager rejects an empty payload outright; fall back to a placeholder when
  # SendGrid isn't configured. The notifier adapter treats any non-2xx SendGrid response
  # as a best-effort failure (logged, never fatal), so an invalid placeholder key is safe.
  secret_data = var.sendgrid_api_key != "" ? var.sendgrid_api_key : "not-configured"
}

resource "google_secret_manager_secret" "cloudsql_password" {
  project   = var.gcp_project_id
  secret_id = "clinic-scheduling-cloudsql-password-${var.environment}"
  replication {
    auto {}
  }
}

resource "google_secret_manager_secret_version" "cloudsql_password" {
  secret      = google_secret_manager_secret.cloudsql_password.id
  secret_data = var.cloudsql_password
}
