variable "gcp_project_id" {
  description = "GCP project ID to deploy into"
  type        = string
}

variable "gcp_region" {
  description = "GCP region for Cloud Run, Pub/Sub, and Cloud SQL"
  type        = string
  default     = "us-central1"
}

variable "environment" {
  description = "Deployment environment name (dev/test/prod)"
  type        = string
  default     = "dev"
}

variable "jwt_secret" {
  description = "HS256 shared secret for JWT verification"
  type        = string
  sensitive   = true
}

variable "sendgrid_api_key" {
  description = "SendGrid API key for email notifications (empty disables notifications)"
  type        = string
  sensitive   = true
  default     = ""
}

variable "cloudsql_password" {
  description = "Password for the Cloud SQL appointments_app user"
  type        = string
  sensitive   = true
}

variable "notifier_from_address" {
  description = "From address used for SendGrid notifications"
  type        = string
  default     = "no-reply@clinic-scheduling.example.com"
}

variable "image_tag" {
  description = "Container image tag to deploy (the CI-built commit SHA, or 'latest' for manual applies against an image built and pushed by hand)"
  type        = string
  default     = "latest"
}
