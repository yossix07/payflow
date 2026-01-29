terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
    helm = {
      source  = "hashicorp/helm"
      version = "~> 2.12"
    }
  }
  
  # Backend configuration will be added in Phase 2
  # backend "s3" {}
}

provider "aws" {
  region = "us-east-1"
  
  default_tags {
    tags = {
      Project     = "SaaS-Cluster"
      Environment = "dev"
      ManagedBy   = "Terraform"
    }
  }
}
