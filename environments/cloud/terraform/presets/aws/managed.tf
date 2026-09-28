# Managed-service variants of the local-preset components. Each block is
# optional (enable_* variables); nothing here is required by the cluster.

resource "aws_security_group" "data" {
  count       = var.enable_database || var.enable_cache ? 1 : 0
  name        = "${var.name}-data"
  description = "Data services reachable from inside the VPC only"
  vpc_id      = aws_vpc.this.id

  ingress {
    description = "PostgreSQL from the VPC"
    from_port   = 5432
    to_port     = 5432
    protocol    = "tcp"
    cidr_blocks = [aws_vpc.this.cidr_block]
  }

  ingress {
    description = "Redis from the VPC"
    from_port   = 6379
    to_port     = 6379
    protocol    = "tcp"
    cidr_blocks = [aws_vpc.this.cidr_block]
  }
}

# --- PostgreSQL (local: components/databases/postgresql) ---

resource "aws_db_subnet_group" "this" {
  count      = var.enable_database ? 1 : 0
  name       = var.name
  subnet_ids = aws_subnet.data[*].id
}

resource "aws_db_instance" "postgres" {
  count                  = var.enable_database ? 1 : 0
  identifier             = "${var.name}-postgres"
  engine                 = "postgres"
  engine_version         = "16"
  instance_class         = var.db_instance_class
  allocated_storage      = 20
  storage_type           = "gp3"
  db_name                = "forgelab"
  username               = "forgelab"
  db_subnet_group_name   = aws_db_subnet_group.this[0].name
  vpc_security_group_ids = [aws_security_group.data[0].id]

  # RDS generates the master password and keeps it in AWS Secrets Manager, so
  # no credential appears in variables or in git.
  manage_master_user_password = true

  publicly_accessible     = false
  multi_az                = false
  backup_retention_period = 1
  skip_final_snapshot     = true
  deletion_protection     = false
}

# --- Redis (local: components/databases/redis) ---

resource "aws_elasticache_subnet_group" "this" {
  count      = var.enable_cache ? 1 : 0
  name       = var.name
  subnet_ids = aws_subnet.data[*].id
}

resource "aws_elasticache_replication_group" "redis" {
  count                      = var.enable_cache ? 1 : 0
  replication_group_id       = "${var.name}-redis"
  description                = "ForgeLab cache"
  engine                     = "redis"
  engine_version             = "7.1"
  node_type                  = var.cache_node_type
  num_cache_clusters         = 1
  subnet_group_name          = aws_elasticache_subnet_group.this[0].name
  security_group_ids         = [aws_security_group.data[0].id]
  at_rest_encryption_enabled = true
  transit_encryption_enabled = true
}

# --- Queue with dead-letter queue (local: components/messaging/rabbitmq) ---

resource "aws_sqs_queue" "jobs_dlq" {
  count                     = var.enable_queue ? 1 : 0
  name                      = "${var.name}-work-jobs-dlq"
  message_retention_seconds = 1209600
  sqs_managed_sse_enabled   = true
}

resource "aws_sqs_queue" "jobs" {
  count                   = var.enable_queue ? 1 : 0
  name                    = "${var.name}-work-jobs"
  sqs_managed_sse_enabled = true

  redrive_policy = jsonencode({
    deadLetterTargetArn = aws_sqs_queue.jobs_dlq[0].arn
    maxReceiveCount     = var.queue_max_receive_count
  })
}

# --- Budget (cost guard) ---

resource "aws_budgets_budget" "experiment" {
  name         = "${var.name}-monthly"
  budget_type  = "COST"
  limit_amount = tostring(var.monthly_budget_usd)
  limit_unit   = "USD"
  time_unit    = "MONTHLY"

  cost_filter {
    name   = "TagKeyValue"
    values = [format("user:forgelab-experiment$%s", var.name)]
  }

  dynamic "notification" {
    for_each = var.budget_alert_email == "" ? [] : [1]
    content {
      comparison_operator        = "GREATER_THAN"
      threshold                  = 80
      threshold_type             = "PERCENTAGE"
      notification_type          = "ACTUAL"
      subscriber_email_addresses = [var.budget_alert_email]
    }
  }
}
