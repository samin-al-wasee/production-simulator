variable "project_id" {
  description = "Google Cloud project that will hold the experiment. Use a dedicated project."
  type        = string
}

variable "name" {
  description = "Experiment name; prefixes every resource. Lowercase letters, digits, and hyphens."
  type        = string
  default     = "forgelab"

  validation {
    condition     = can(regex("^[a-z][a-z0-9-]{1,24}$", var.name))
    error_message = "name must be 2-25 lowercase letters, digits, or hyphens, starting with a letter."
  }
}

variable "owner" {
  description = "Person or team responsible for the experiment. Lowercase letters, digits, hyphens, underscores (label rules)."
  type        = string

  validation {
    condition     = can(regex("^[a-z0-9_-]{1,63}$", var.owner))
    error_message = "owner must be 1-63 characters of lowercase letters, digits, hyphens, or underscores."
  }
}

variable "region" {
  type    = string
  default = "us-central1"
}

variable "experiment_ttl_hours" {
  description = "How long the experiment may live before it must be destroyed. Labelled on every resource."
  type        = number
  default     = 8

  validation {
    condition     = var.experiment_ttl_hours >= 1 && var.experiment_ttl_hours <= 72
    error_message = "experiment_ttl_hours must be between 1 and 72."
  }
}

variable "billing_account" {
  description = "Billing account ID. When set, a budget with alerts is created for the project."
  type        = string
  default     = ""
}

variable "monthly_budget_usd" {
  type    = number
  default = 50

  validation {
    condition     = var.monthly_budget_usd > 0 && var.monthly_budget_usd <= 500
    error_message = "monthly_budget_usd must be between 1 and 500."
  }
}

variable "node_machine_type" {
  type    = string
  default = "e2-medium"

  validation {
    condition     = contains(["e2-small", "e2-medium"], var.node_machine_type)
    error_message = "node_machine_type must be e2-small or e2-medium (cost guard)."
  }
}

variable "node_count" {
  description = "Nodes per zone in the regional cluster (a regional cluster runs this many in each of three zones)."
  type        = number
  default     = 1

  validation {
    condition     = var.node_count >= 1 && var.node_count <= 2
    error_message = "node_count must be 1 or 2 (cost guard)."
  }
}

variable "use_spot" {
  type    = bool
  default = true
}

variable "enable_database" {
  description = "Managed PostgreSQL (Cloud SQL)."
  type        = bool
  default     = true
}

variable "db_tier" {
  type    = string
  default = "db-f1-micro"

  validation {
    condition     = contains(["db-f1-micro", "db-g1-small"], var.db_tier)
    error_message = "db_tier must be db-f1-micro or db-g1-small (cost guard)."
  }
}

variable "enable_cache" {
  description = "Managed Redis (Memorystore)."
  type        = bool
  default     = true
}

variable "enable_queue" {
  description = "Managed messaging with a dead-letter topic (Pub/Sub), the managed counterpart of the RabbitMQ topology."
  type        = bool
  default     = true
}

variable "queue_max_delivery_attempts" {
  description = "Deliveries before a message is dead-lettered (matches x-delivery-limit in the local preset)."
  type        = number
  default     = 5

  validation {
    condition     = var.queue_max_delivery_attempts >= 5 && var.queue_max_delivery_attempts <= 100
    error_message = "Pub/Sub requires max_delivery_attempts between 5 and 100."
  }
}
