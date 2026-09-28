terraform {
  required_version = ">= 1.6"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.80"
    }
  }
}

provider "aws" {
  region = var.region

  # Every resource carries these tags; environments/cloud/cost-guard.yaml
  # rejects plans that lack them.
  default_tags {
    tags = {
      "forgelab-experiment" = var.name
      "forgelab-owner"      = var.owner
      "forgelab-ttl-hours"  = tostring(var.experiment_ttl_hours)
    }
  }
}
