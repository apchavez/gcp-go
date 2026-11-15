output "api_url" {
  description = "Public URL of the Cloud Run API service"
  value       = google_cloud_run_v2_service.api.uri
}

output "worker_url" {
  description = "URL of the Cloud Run worker service (Pub/Sub push target)"
  value       = google_cloud_run_v2_service.worker.uri
}

output "confirm_url" {
  description = "URL of the Cloud Run confirm service (Eventarc target, stage B of the confirmation flow)"
  value       = google_cloud_run_v2_service.confirm.uri
}

output "api_gateway_url" {
  description = "Public URL of the API Gateway in front of the api Cloud Run service (quota-limited, equivalent to AWS's throttled API Gateway stage)"
  value       = "https://${google_api_gateway_gateway.appointments.default_hostname}"
}

output "cloudsql_connection_name" {
  description = "Cloud SQL instance connection name for Cloud SQL Auth Proxy / connectors"
  value       = google_sql_database_instance.appointments.connection_name
}
