# Managed-service variants of the local-preset components. Each block is
# optional (enable_* variables); nothing here is required by the cluster.

# --- PostgreSQL (local: components/databases/postgresql) ---

resource "google_compute_global_address" "private_services" {
  count         = var.enable_database ? 1 : 0
  name          = "${var.name}-private-services"
  purpose       = "VPC_PEERING"
  address_type  = "INTERNAL"
  prefix_length = 20
  network       = google_compute_network.this.id
}

resource "google_service_networking_connection" "private_services" {
  count                   = var.enable_database ? 1 : 0
  network                 = google_compute_network.this.id
  service                 = "servicenetworking.googleapis.com"
  reserved_peering_ranges = [google_compute_global_address.private_services[0].name]

  depends_on = [google_project_service.apis]
}

resource "google_sql_database_instance" "postgres" {
  count               = var.enable_database ? 1 : 0
  name                = "${var.name}-postgres"
  region              = var.region
  database_version    = "POSTGRES_16"
  deletion_protection = false

  settings {
    tier              = var.db_tier
    edition           = "ENTERPRISE"
    availability_type = "ZONAL"
    disk_size         = 10
    disk_type         = "PD_SSD"

    # No public address; reachable only through the peered VPC.
    ip_configuration {
      ipv4_enabled    = false
      private_network = google_compute_network.this.id
    }

    # Passwordless access: the application authenticates as its Google
    # service account, so no database credential exists to leak.
    database_flags {
      name  = "cloudsql.iam_authentication"
      value = "on"
    }

    user_labels = {
      forgelab-experiment = var.name
      forgelab-owner      = var.owner
      forgelab-ttl-hours  = tostring(var.experiment_ttl_hours)
    }
  }

  depends_on = [google_service_networking_connection.private_services]
}

resource "google_sql_database" "forgelab" {
  count    = var.enable_database ? 1 : 0
  name     = "forgelab"
  instance = google_sql_database_instance.postgres[0].name
}

resource "google_sql_user" "app" {
  count    = var.enable_database ? 1 : 0
  name     = trimsuffix(google_service_account.app.email, ".gserviceaccount.com")
  instance = google_sql_database_instance.postgres[0].name
  type     = "CLOUD_IAM_SERVICE_ACCOUNT"
}

resource "google_project_iam_member" "app_sql_client" {
  count   = var.enable_database ? 1 : 0
  project = var.project_id
  role    = "roles/cloudsql.instanceUser"
  member  = "serviceAccount:${google_service_account.app.email}"
}

# --- Redis (local: components/databases/redis) ---

resource "google_redis_instance" "cache" {
  count              = var.enable_cache ? 1 : 0
  name               = "${var.name}-redis"
  tier               = "BASIC"
  memory_size_gb     = 1
  region             = var.region
  redis_version      = "REDIS_7_0"
  authorized_network = google_compute_network.this.id

  labels = {
    forgelab-experiment = var.name
    forgelab-owner      = var.owner
    forgelab-ttl-hours  = tostring(var.experiment_ttl_hours)
  }

  depends_on = [google_project_service.apis]
}

# --- Messaging with dead-letter topic (local: components/messaging/rabbitmq) ---

resource "google_pubsub_topic" "jobs_dlq" {
  count = var.enable_queue ? 1 : 0
  name  = "${var.name}-work-jobs-dlq"

  depends_on = [google_project_service.apis]
}

resource "google_pubsub_subscription" "jobs_dlq" {
  count = var.enable_queue ? 1 : 0
  name  = "${var.name}-work-jobs-dlq"
  topic = google_pubsub_topic.jobs_dlq[0].id
}

resource "google_pubsub_topic" "jobs" {
  count = var.enable_queue ? 1 : 0
  name  = "${var.name}-work-jobs"

  depends_on = [google_project_service.apis]
}

resource "google_pubsub_subscription" "jobs" {
  count = var.enable_queue ? 1 : 0
  name  = "${var.name}-work-jobs"
  topic = google_pubsub_topic.jobs[0].id

  dead_letter_policy {
    dead_letter_topic     = google_pubsub_topic.jobs_dlq[0].id
    max_delivery_attempts = var.queue_max_delivery_attempts
  }
}

# Pub/Sub needs permission to publish dead letters and to acknowledge from
# the source subscription.
resource "google_pubsub_topic_iam_member" "dlq_publisher" {
  count  = var.enable_queue ? 1 : 0
  topic  = google_pubsub_topic.jobs_dlq[0].name
  role   = "roles/pubsub.publisher"
  member = "serviceAccount:service-${data.google_project.this.number}@gcp-sa-pubsub.iam.gserviceaccount.com"
}

resource "google_pubsub_subscription_iam_member" "jobs_subscriber" {
  count        = var.enable_queue ? 1 : 0
  subscription = google_pubsub_subscription.jobs[0].name
  role         = "roles/pubsub.subscriber"
  member       = "serviceAccount:service-${data.google_project.this.number}@gcp-sa-pubsub.iam.gserviceaccount.com"
}

# --- Budget (cost guard) ---

resource "google_billing_budget" "experiment" {
  count           = var.billing_account == "" ? 0 : 1
  billing_account = var.billing_account
  display_name    = "${var.name}-monthly"

  budget_filter {
    projects = ["projects/${data.google_project.this.number}"]
  }

  amount {
    specified_amount {
      currency_code = "USD"
      units         = tostring(var.monthly_budget_usd)
    }
  }

  threshold_rules {
    threshold_percent = 0.5
  }

  threshold_rules {
    threshold_percent = 0.8
  }

  threshold_rules {
    threshold_percent = 1.0
  }
}
