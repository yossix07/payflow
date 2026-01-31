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
  
  backend "s3" {
    bucket         = "saas-cluster-tfstate-467341cd"
    key            = "dev/terraform.tfstate"
    region         = "us-east-1"
    dynamodb_table = "saas-cluster-locks"
    encrypt        = true
  }
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
