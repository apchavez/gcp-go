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

    ip_configuration {
      ssl_mode = "ENCRYPTED_ONLY" # reject unencrypted client connections
    }

    database_flags {
      name  = "log_connections"
      value = "on"
    }
  }

  deletion_protection = false
}

# deletion_policy = "ABANDON" on both resources below: once real data/schema exists (after
# cmd/worker's self-migration on boot - see db/migration/migration.go), Terraform's normal
# destroy tries to run actual `DROP DATABASE`/`DROP USER` calls against Cloud SQL, which
# Postgres rejects - "database is being accessed by other users" (lingering connections) and
# "role ... cannot be dropped because some objects depend on it" (the migrated table is owned
# by this user). ABANDON just drops these two sub-resources from Terraform state without
# calling the Cloud SQL API for them; the data is still fully destroyed a moment later when
# google_sql_database_instance.appointments itself is torn down (deletion_protection = false
# above), which doesn't go through Postgres-level DROP semantics at all. Found 2026-07-27/28
# during interview-demo teardown, right after the self-migration fix made this the first
# `terraform destroy` to ever run against an instance with a real schema in it.
resource "google_sql_database" "appointments" {
  project  = var.gcp_project_id
  name     = "appointments"
  instance = google_sql_database_instance.appointments.name

  deletion_policy = "ABANDON"
}

resource "google_sql_user" "app" {
  project  = var.gcp_project_id
  name     = "appointments_app"
  instance = google_sql_database_instance.appointments.name
  password = var.cloudsql_password

  deletion_policy = "ABANDON"
}
