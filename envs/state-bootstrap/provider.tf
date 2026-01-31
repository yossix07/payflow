# =============================================================================
# PROVIDER CONFIGURATION
# =============================================================================
# This file tells Terraform:
#   1. What version of Terraform is required
#   2. Which "providers" (plugins) to download
#   3. How to configure those providers
#
# CONCEPT: Providers
# ------------------
# Terraform doesn't know how to talk to AWS, Azure, or GCP by itself.
# It uses "providers" - plugins that translate your .tf code into API calls.
# Think of providers as drivers: just like your computer needs a driver to 
# talk to a printer, Terraform needs a provider to talk to AWS.
# =============================================================================

terraform {
  # -------------------------------------------------------------------------
  # REQUIRED VERSION
  # -------------------------------------------------------------------------
  # This ensures everyone on the team uses a compatible Terraform version.
  # ">= 1.0" means "version 1.0 or higher".
  # 
  # WHY THIS MATTERS:
  # Terraform syntax can change between versions. Locking the version
  # prevents "works on my machine" problems.
  # -------------------------------------------------------------------------
  required_version = ">= 1.0"

  # -------------------------------------------------------------------------
  # REQUIRED PROVIDERS
  # -------------------------------------------------------------------------
  # Here we declare which providers we need and their versions.
  # 
  # source  = Where to download from (default: Terraform Registry)
  # version = Version constraint (~ means "compatible with")
  #           "~> 5.0" means any 5.x version (5.0, 5.1, 5.99) but NOT 6.0
  # -------------------------------------------------------------------------
  required_providers {
    # AWS Provider - lets us create AWS resources (S3, DynamoDB, EC2, etc.)
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }

    # Random Provider - generates random values (for unique bucket names)
    random = {
      source  = "hashicorp/random"
      version = "~> 3.0"
    }
  }

  # -------------------------------------------------------------------------
  # BACKEND CONFIGURATION (intentionally missing!)
  # -------------------------------------------------------------------------
  # NOTICE: There is NO "backend" block here. This is intentional!
  # 
  # THE CHICKEN-AND-EGG PROBLEM:
  # - We need an S3 bucket to store Terraform state
  # - But we need Terraform to create the S3 bucket
  # - So how can we store state in a bucket that doesn't exist yet?
  #
  # SOLUTION:
  # This bootstrap environment uses LOCAL STATE (a file on your computer).
  # After we create the bucket, our main environments (like envs/dev) 
  # will use that bucket as their backend.
  # -------------------------------------------------------------------------
}

# =============================================================================
# AWS PROVIDER CONFIGURATION
# =============================================================================
# This block configures HOW Terraform connects to AWS.
#
# AUTHENTICATION:
# Terraform uses your AWS credentials in this order:
#   1. Environment variables: AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY
#   2. Shared credentials file: ~/.aws/credentials (from `aws configure`)
#   3. IAM role (if running on EC2 or ECS)
#
# We don't hardcode credentials here - that would be a security risk!
# =============================================================================

provider "aws" {
  # The AWS region where resources will be created
  # us-east-1 is often cheapest and has all services available
  region = var.aws_region

  # -------------------------------------------------------------------------
  # DEFAULT TAGS
  # -------------------------------------------------------------------------
  # These tags are automatically added to EVERY resource we create.
  # 
  # WHY TAGS MATTER:
  #   - Cost tracking: Filter your AWS bill by project
  #   - Organization: Know what each resource is for
  #   - Automation: Scripts can find resources by tag
  #   - Compliance: Many companies require tagging policies
  # -------------------------------------------------------------------------
  default_tags {
    tags = {
      Project   = "SaaS-Cluster"
      ManagedBy = "Terraform"
      Purpose   = "State-Bootstrap"
    }
  }
}
