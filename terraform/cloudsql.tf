# Cloud SQL for PostgreSQL - the relational side that only ProcessAppointment writes to,
# mirroring the AWS sibling's RDS MySQL / Azure sibling's Azure SQL.

resource "google_sql_database_instance" "appointments" {
  project          = var.gcp_project_id
  name             = "clinic-scheduling-${var.environment}"
  region           = var.gcp_region
  database_version = "POSTGRES_16"

  settings {
    edition           = "ENTERPRISE"  # db-f1-micro is only valid under Enterprise edition, not the new default Enterprise Plus
    tier              = "db-f1-micro" # smallest tier - portfolio project, not production load
    availability_type = "ZONAL"

    backup_configuration {
      enabled = true
    }
  }

  deletion_protection = false
}

resource "google_sql_database" "appointments" {
  project  = var.gcp_project_id
  name     = "appointments"
  instance = google_sql_database_instance.appointments.name
}

resource "google_sql_user" "app" {
  project  = var.gcp_project_id
  name     = "appointments_app"
  instance = google_sql_database_instance.appointments.name
  password = var.cloudsql_password
}
