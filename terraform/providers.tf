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
