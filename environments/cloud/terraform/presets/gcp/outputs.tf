output "cluster_name" {
  value = google_container_cluster.this.name
}

output "kubeconfig_command" {
  value = "gcloud container clusters get-credentials ${google_container_cluster.this.name} --region ${var.region} --project ${var.project_id}"
}

output "database_connection_name" {
  value = var.enable_database ? google_sql_database_instance.postgres[0].connection_name : null
}

output "redis_host" {
  value = var.enable_cache ? google_redis_instance.cache[0].host : null
}

output "queue_topic" {
  value = var.enable_queue ? google_pubsub_topic.jobs[0].id : null
}

output "dead_letter_topic" {
  value = var.enable_queue ? google_pubsub_topic.jobs_dlq[0].id : null
}

output "app_service_account" {
  description = "Service account the application should run as (Workload Identity)."
  value       = google_service_account.app.email
}
