output "cluster_name" {
  value = aws_eks_cluster.this.name
}

output "kubeconfig_command" {
  value = "aws eks update-kubeconfig --region ${var.region} --name ${aws_eks_cluster.this.name}"
}

output "database_endpoint" {
  value = var.enable_database ? aws_db_instance.postgres[0].endpoint : null
}

output "database_secret_arn" {
  description = "Secrets Manager secret holding the generated master password."
  value       = var.enable_database ? aws_db_instance.postgres[0].master_user_secret[0].secret_arn : null
}

output "redis_endpoint" {
  value = var.enable_cache ? aws_elasticache_replication_group.redis[0].primary_endpoint_address : null
}

output "queue_url" {
  value = var.enable_queue ? aws_sqs_queue.jobs[0].url : null
}

output "dead_letter_queue_url" {
  value = var.enable_queue ? aws_sqs_queue.jobs_dlq[0].url : null
}
