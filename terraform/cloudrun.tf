resource "google_cloud_run_v2_service" "api" {
  project             = var.gcp_project_id
  name                = "clinic-scheduling-api-${var.environment}"
  location            = var.gcp_region
  deletion_protection = false # dev/portfolio project - Terraform must be free to replace this service

  template {
    service_account = google_service_account.clinic_scheduling.email

    containers {
      image = "gcr.io/${var.gcp_project_id}/clinic-scheduling-api:${var.image_tag}"

      env {
        name  = "GCP_PROJECT_ID"
        value = var.gcp_project_id
      }
      env {
        name = "JWT_SECRET"
        value_source {
          secret_key_ref {
            secret  = google_secret_manager_secret.jwt_secret.secret_id
            version = "latest"
          }
        }
      }
      env {
        name = "SENDGRID_API_KEY"
        value_source {
          secret_key_ref {
            secret  = google_secret_manager_secret.sendgrid_api_key.secret_id
            version = "latest"
          }
        }
      }
      env {
        name  = "NOTIFIER_FROM_ADDRESS"
        value = var.notifier_from_address
      }
    }
  }
}

resource "google_cloud_run_v2_service" "worker" {
  project             = var.gcp_project_id
  name                = "clinic-scheduling-worker-${var.environment}"
  location            = var.gcp_region
  deletion_protection = false # dev/portfolio project - Terraform must be free to replace this service

  template {
    service_account = google_service_account.clinic_scheduling.email

    containers {
      image = "gcr.io/${var.gcp_project_id}/clinic-scheduling-worker:${var.image_tag}"

      env {
        name  = "GCP_PROJECT_ID"
        value = var.gcp_project_id
      }
    }
  }
}

# Cloud Run services default to requiring authenticated invocations - Pub/Sub push
# subscriptions authenticate via an OIDC token signed by this same service account.
resource "google_cloud_run_v2_service_iam_member" "worker_pubsub_invoker" {
  project  = var.gcp_project_id
  location = var.gcp_region
  name     = google_cloud_run_v2_service.worker.name
  role     = "roles/run.invoker"
  member   = "serviceAccount:${google_service_account.clinic_scheduling.email}"
}

resource "google_cloud_run_v2_service_iam_member" "api_public_invoker" {
  project  = var.gcp_project_id
  location = var.gcp_region
  name     = google_cloud_run_v2_service.api.name
  role     = "roles/run.invoker"
  member   = "allUsers" # the API enforces its own JWT auth in-application, same model as the AWS/Azure siblings
}
