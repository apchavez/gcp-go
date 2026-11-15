# GCP API Gateway is the direct equivalent of the AWS sibling's API Gateway HTTP API
# throttle (burst 50 / rate 25 rps) and the Azure sibling's API Management
# `rate-limit-by-key` policy - a managed gateway layer in front of compute enforcing a
# request quota, added without removing direct access to the `api` Cloud Run service
# (existing tests/CI/Postman keep hitting it directly; the gateway is the additional,
# recommended path).
#
# In beta as of the current `hashicorp/google` provider line - requires the
# `google-beta` provider alias declared in providers.tf/versions.tf.

resource "google_project_service" "apigateway" {
  project            = var.gcp_project_id
  service            = "apigateway.googleapis.com"
  disable_on_destroy = false
}

resource "google_project_service" "servicemanagement" {
  project            = var.gcp_project_id
  service            = "servicemanagement.googleapis.com"
  disable_on_destroy = false
}

resource "google_project_service" "servicecontrol" {
  project            = var.gcp_project_id
  service            = "servicecontrol.googleapis.com"
  disable_on_destroy = false
}

resource "google_api_gateway_api" "appointments" {
  provider = google-beta
  project  = var.gcp_project_id
  api_id   = "clinic-scheduling-${var.environment}"

  depends_on = [google_project_service.apigateway]
}

resource "google_api_gateway_api_config" "appointments" {
  provider = google-beta
  project  = var.gcp_project_id
  api      = google_api_gateway_api.appointments.api_id

  # A fixed api_config_id doesn't work with create_before_destroy: the GCP API rejects
  # creating a new config while the old one still holds that exact ID (409 "already
  # exists"), so any future change to this resource (e.g. the auth fix above) fails
  # apply outright. A prefix lets the provider suffix each revision with a content hash,
  # so the new config gets a distinct ID, gets created, the gateway is repointed to it,
  # and only then is the old config destroyed - true zero-downtime replacement. Hit and
  # fixed 2026-07-27 (run 30312705473) in the same pass as the header-overwrite fix above.
  api_config_id_prefix = "clinic-scheduling-config-${var.environment}-"

  # No backend_config/service-account here on purpose: when API Gateway is given a backend
  # service account, ESPv2 replaces the inbound `Authorization` header with a Google-signed
  # ID token for the Cloud Run invocation (moving the caller's original header to
  # `X-Forwarded-Authorization` instead) - which broke this app's own HS256 bearer-token
  # check (`internal/infrastructure/auth/guard.go` only ever reads `Authorization`), causing
  # every gateway-routed request to fail with a generic 403 "Access denied" even with a
  # valid, unexpired token. Found and fixed 2026-07-27 during interview-demo E2E verification.
  # The `api` Cloud Run service already grants `allUsers` invoker (cloudrun.tf) and enforces
  # its own JWT auth in-application, so no gateway->backend IAM auth is needed here.

  openapi_documents {
    document {
      path = "api-gateway-openapi.yaml"
      contents = base64encode(templatefile("${path.module}/api-gateway-openapi.yaml.tftpl", {
        backend_address = google_cloud_run_v2_service.api.uri
        environment     = var.environment
      }))
    }
  }

  lifecycle {
    create_before_destroy = true
  }

  depends_on = [
    google_project_service.servicemanagement,
    google_project_service.servicecontrol,
  ]
}

resource "google_api_gateway_gateway" "appointments" {
  provider   = google-beta
  project    = var.gcp_project_id
  region     = var.gcp_region
  api_config = google_api_gateway_api_config.appointments.id
  gateway_id = "clinic-scheduling-gw-${var.environment}"
}

