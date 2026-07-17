output "api_url" {
  description = "Public URL of the Cloud Run API service"
  value       = google_cloud_run_v2_service.api.uri
}

output "worker_url" {
  description = "URL of the Cloud Run worker service (Pub/Sub push target)"
  value       = google_cloud_run_v2_service.worker.uri
}

output "cloudsql_connection_name" {
  description = "Cloud SQL instance connection name for Cloud SQL Auth Proxy / connectors"
  value       = google_sql_database_instance.appointments.connection_name
}
