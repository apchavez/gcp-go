resource "google_cloud_run_v2_service" "api" {
  project             = var.gcp_project_id
  name                = "clinic-scheduling-api-${var.environment}"
  location            = var.gcp_region
  deletion_protection = false # dev/portfolio project - Terraform must be free to replace this service

  # Wait for the secret accessor grants (iam.tf) to propagate - this service references
  # jwt_secret and sendgrid_api_key and has no slow upstream dependency, so it can
  # otherwise be created before the IAM binding is visible to the Cloud Run control plane.
  depends_on = [time_sleep.secret_iam_propagation]

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

  # Wait for the secret accessor grants (iam.tf) to propagate - see api service for why.
  depends_on = [time_sleep.secret_iam_propagation]

  template {
    service_account = google_service_account.clinic_scheduling.email

    containers {
      image = "gcr.io/${var.gcp_project_id}/clinic-scheduling-worker:${var.image_tag}"

      env {
        name  = "GCP_PROJECT_ID"
        value = var.gcp_project_id
      }
      # Cloud SQL Auth Proxy sidecar-less connection via the native Cloud Run v2 Cloud SQL
      # volume (mounted below) - stage A's Persist call writes the relational side here.
      env {
        name  = "CLOUDSQL_SOCKET_DIR"
        value = "/cloudsql/${google_sql_database_instance.appointments.connection_name}"
      }
      env {
        name  = "CLOUDSQL_USER"
        value = google_sql_user.app.name
      }
      env {
        name  = "CLOUDSQL_DB"
        value = google_sql_database.appointments.name
      }
      env {
        name = "CLOUDSQL_PASSWORD"
        value_source {
          secret_key_ref {
            secret  = google_secret_manager_secret.cloudsql_password.secret_id
            version = "latest"
          }
        }
      }

      volume_mounts {
        name       = "cloudsql"
        mount_path = "/cloudsql"
      }
    }

    volumes {
      name = "cloudsql"
      cloud_sql_instance {
        instances = [google_sql_database_instance.appointments.connection_name]
      }
    }
  }
}

# Stage B of the confirmation flow (see pubsub.tf/eventarc.tf): Eventarc invokes this
# service off the "appointment-confirmed" topic instead of a plain push subscription,
# mirroring the AWS sibling's confirmAppointment λ. Marks Firestore COMPLETED + notifies.
resource "google_cloud_run_v2_service" "confirm" {
  project             = var.gcp_project_id
  name                = "clinic-scheduling-confirm-${var.environment}"
  location            = var.gcp_region
  deletion_protection = false # dev/portfolio project - Terraform must be free to replace this service

  # Wait for the secret accessor grants (iam.tf) to propagate - see api service for why.
  depends_on = [time_sleep.secret_iam_propagation]

  template {
    service_account = google_service_account.clinic_scheduling.email

    containers {
      image = "gcr.io/${var.gcp_project_id}/clinic-scheduling-confirm:${var.image_tag}"

      env {
        name  = "GCP_PROJECT_ID"
        value = var.gcp_project_id
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

# The Eventarc trigger (eventarc.tf) authenticates to the confirm service as this same
# service account - needs its own invoker binding, same pattern as worker_pubsub_invoker.
resource "google_cloud_run_v2_service_iam_member" "confirm_eventarc_invoker" {
  project  = var.gcp_project_id
  location = var.gcp_region
  name     = google_cloud_run_v2_service.confirm.name
  role     = "roles/run.invoker"
  member   = "serviceAccount:${google_service_account.clinic_scheduling.email}"
}
