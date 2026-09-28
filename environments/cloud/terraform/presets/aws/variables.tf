variable "name" {
  description = "Experiment name; prefixes every resource."
  type        = string
  default     = "forgelab"
}

variable "owner" {
  description = "Person or team responsible for the experiment (required for cost tracking)."
  type        = string

  validation {
    condition     = length(trimspace(var.owner)) > 0
    error_message = "owner must not be empty."
  }
}

variable "region" {
  type    = string
  default = "us-east-1"
}

variable "experiment_ttl_hours" {
  description = "How long the experiment may live before it must be destroyed. Tagged on every resource."
  type        = number
  default     = 8

  validation {
    condition     = var.experiment_ttl_hours >= 1 && var.experiment_ttl_hours <= 72
    error_message = "experiment_ttl_hours must be between 1 and 72."
  }
}

variable "monthly_budget_usd" {
  description = "AWS Budgets limit for resources tagged with this experiment."
  type        = number
  default     = 50

  validation {
    condition     = var.monthly_budget_usd > 0 && var.monthly_budget_usd <= 500
    error_message = "monthly_budget_usd must be between 1 and 500."
  }
}

variable "budget_alert_email" {
  description = "Email notified at 80% of the budget. Leave empty to create the budget without notifications."
  type        = string
  default     = ""
}

variable "node_instance_type" {
  type    = string
  default = "t3.medium"

  validation {
    condition     = contains(["t3.small", "t3.medium"], var.node_instance_type)
    error_message = "node_instance_type must be t3.small or t3.medium (cost guard)."
  }
}

variable "node_count" {
  type    = number
  default = 2

  validation {
    condition     = var.node_count >= 1 && var.node_count <= 3
    error_message = "node_count must be between 1 and 3 (cost guard)."
  }
}

variable "use_spot" {
  description = "Run worker nodes on Spot capacity."
  type        = bool
  default     = true
}

variable "enable_database" {
  description = "Managed PostgreSQL (Amazon RDS)."
  type        = bool
  default     = true
}

variable "db_instance_class" {
  type    = string
  default = "db.t4g.micro"

  validation {
    condition     = contains(["db.t4g.micro", "db.t4g.small"], var.db_instance_class)
    error_message = "db_instance_class must be db.t4g.micro or db.t4g.small (cost guard)."
  }
}

variable "enable_cache" {
  description = "Managed Redis (Amazon ElastiCache)."
  type        = bool
  default     = true
}

variable "cache_node_type" {
  type    = string
  default = "cache.t4g.micro"

  validation {
    condition     = contains(["cache.t4g.micro", "cache.t4g.small"], var.cache_node_type)
    error_message = "cache_node_type must be cache.t4g.micro or cache.t4g.small (cost guard)."
  }
}

variable "enable_queue" {
  description = "Managed queue with a dead-letter queue (Amazon SQS), the managed counterpart of the RabbitMQ topology."
  type        = bool
  default     = true
}

variable "queue_max_receive_count" {
  description = "Deliveries before a message is dead-lettered (matches x-delivery-limit in the local preset)."
  type        = number
  default     = 3
}
