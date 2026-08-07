# Cloud Trace is GCP's equivalent of AWS X-Ray / Azure Application Insights - all 3
# services (api/worker/confirm) export spans here via internal/infrastructure/tracing.
# "roles/cloudtrace.agent" is the minimal role that lets a service account write trace
# spans (read/query access is a separate "roles/cloudtrace.user", not needed here).

resource "google_project_service" "cloudtrace" {
  project            = var.gcp_project_id
  service            = "cloudtrace.googleapis.com"
  disable_on_destroy = false # other resources may depend on the API staying enabled; avoid an accidental disable on a routine destroy
}

resource "google_project_iam_member" "cloudtrace_agent" {
  project = var.gcp_project_id
  role    = "roles/cloudtrace.agent"
  member  = "serviceAccount:${google_service_account.clinic_scheduling.email}"

  depends_on = [google_project_service.cloudtrace]
}
