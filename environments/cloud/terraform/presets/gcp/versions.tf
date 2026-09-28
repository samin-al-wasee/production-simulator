terraform {
  required_version = ">= 1.6"

  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 6.14"
    }
  }
}

provider "google" {
  project = var.project_id
  region  = var.region

  # Applied to every resource that supports labels; environments/cloud/
  # cost-guard.yaml rejects plans that lack them.
  default_labels = {
    forgelab-experiment = var.name
    forgelab-owner      = var.owner
    forgelab-ttl-hours  = tostring(var.experiment_ttl_hours)
  }
}
