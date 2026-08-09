terraform {
  backend "gcs" {
    bucket = "clinic-scheduling-gcp-dev-tfstate"
    prefix = "terraform/state"
  }
}

provider "google" {
  project = var.gcp_project_id
  region  = var.gcp_region
}

# google_api_gateway_* resources (api_gateway.tf) are still beta-only in this provider
# line - requires the google-beta provider alias in addition to the GA provider above.
provider "google-beta" {
  project = var.gcp_project_id
  region  = var.gcp_region
}
