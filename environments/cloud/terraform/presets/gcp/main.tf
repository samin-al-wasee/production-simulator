data "google_project" "this" {}

locals {
  apis = toset([
    "compute.googleapis.com",
    "container.googleapis.com",
    "sqladmin.googleapis.com",
    "redis.googleapis.com",
    "pubsub.googleapis.com",
    "servicenetworking.googleapis.com",
  ])
}

resource "google_project_service" "apis" {
  for_each           = local.apis
  service            = each.value
  disable_on_destroy = false
}

resource "google_compute_network" "this" {
  name                    = var.name
  auto_create_subnetworks = false

  depends_on = [google_project_service.apis]
}

resource "google_compute_subnetwork" "nodes" {
  name          = "${var.name}-nodes"
  ip_cidr_range = "10.10.0.0/20"
  region        = var.region
  network       = google_compute_network.this.id

  secondary_ip_range {
    range_name    = "pods"
    ip_cidr_range = "10.20.0.0/16"
  }

  secondary_ip_range {
    range_name    = "services"
    ip_cidr_range = "10.30.0.0/20"
  }
}

resource "google_service_account" "nodes" {
  account_id   = "${var.name}-nodes"
  display_name = "ForgeLab GKE nodes (${var.name})"
}

resource "google_service_account" "app" {
  account_id   = "${var.name}-app"
  display_name = "ForgeLab application workload identity (${var.name})"
}

resource "google_container_cluster" "this" {
  name     = var.name
  location = var.region

  network    = google_compute_network.this.id
  subnetwork = google_compute_subnetwork.nodes.id

  # The default pool is replaced by the managed pool below.
  remove_default_node_pool = true
  initial_node_count       = 1
  deletion_protection      = false

  ip_allocation_policy {
    cluster_secondary_range_name  = "pods"
    services_secondary_range_name = "services"
  }

  workload_identity_config {
    workload_pool = "${var.project_id}.svc.id.goog"
  }

  resource_labels = {
    forgelab-experiment = var.name
    forgelab-owner      = var.owner
    forgelab-ttl-hours  = tostring(var.experiment_ttl_hours)
  }
}

resource "google_container_node_pool" "workers" {
  name       = "${var.name}-workers"
  location   = var.region
  cluster    = google_container_cluster.this.name
  node_count = var.node_count

  node_config {
    machine_type    = var.node_machine_type
    spot            = var.use_spot
    service_account = google_service_account.nodes.email
    oauth_scopes    = ["https://www.googleapis.com/auth/cloud-platform"]

    labels = {
      forgelab-experiment = var.name
      forgelab-owner      = var.owner
      forgelab-ttl-hours  = tostring(var.experiment_ttl_hours)
    }
  }
}
