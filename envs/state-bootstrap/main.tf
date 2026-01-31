# =============================================================================
# MAIN CONFIGURATION - State Backend Resources
# =============================================================================
# This file creates the AWS resources needed to store Terraform state remotely:
#   1. S3 Bucket - stores the actual state file (terraform.tfstate)
#   2. DynamoDB Table - prevents concurrent modifications (locking)
#
# WHAT IS TERRAFORM STATE?
# ------------------------
# When you run `terraform apply`, Terraform needs to know what already exists
# so it can determine what to create, update, or delete. This info is stored
# in a "state file" (terraform.tfstate).
#
# By default, state is stored locally. But for teams (or even solo devs with
# multiple machines), you need REMOTE STATE so everyone sees the same state.
# =============================================================================


# =============================================================================
# RANDOM ID - For Unique Bucket Names
# =============================================================================
# S3 bucket names must be GLOBALLY unique across ALL of AWS (all accounts,
# all regions, everyone in the world). So we append a random suffix.
#
# byte_length = 4 means 4 random bytes = 8 hex characters (e.g., "a1b2c3d4")
# =============================================================================

resource "random_id" "suffix" {
  byte_length = 4
}


# =============================================================================
# LOCALS - Computed Values
# =============================================================================
# Locals are like variables, but they're computed from other values.
# Use them to avoid repeating complex expressions.
# =============================================================================

locals {
  # Combine project name with random suffix for a unique bucket name
  # Example result: "saas-cluster-tfstate-a1b2c3d4"
  bucket_name = "${var.project_name}-tfstate-${random_id.suffix.hex}"
}


# =============================================================================
# S3 BUCKET - Stores Terraform State
# =============================================================================
# This is where your terraform.tfstate file will live.
#
# IMPORTANT SETTINGS:
#   - force_destroy = true: Allows deleting bucket even if it has files
#     (For learning/ephemeral infra. In production, set to false!)
# =============================================================================

resource "aws_s3_bucket" "state" {
  bucket = local.bucket_name

  # force_destroy = true means:
  # "When I run terraform destroy, delete this bucket even if it's not empty"
  # 
  # WHY TRUE HERE?
  # This is learning/ephemeral infrastructure. We want easy cleanup.
  # 
  # IN PRODUCTION: Set to false! You don't want to accidentally delete
  # your state file - that would be VERY bad (you'd lose track of all
  # your infrastructure).
  force_destroy = true
}


# =============================================================================
# S3 BUCKET VERSIONING
# =============================================================================
# Versioning keeps a history of every version of every file in the bucket.
#
# WHY THIS MATTERS FOR STATE:
# If someone accidentally corrupts the state file, you can restore a
# previous version. This has saved many engineers' jobs!
# =============================================================================

resource "aws_s3_bucket_versioning" "state" {
  bucket = aws_s3_bucket.state.id

  versioning_configuration {
    status = "Enabled"
  }
}


# =============================================================================
# S3 BUCKET ENCRYPTION
# =============================================================================
# Encrypts all files at rest using AES-256.
#
# Even though state files don't usually contain passwords, they can contain
# sensitive info like database endpoints, resource IDs, etc.
# Encryption is a security best practice and often required for compliance.
# =============================================================================

resource "aws_s3_bucket_server_side_encryption_configuration" "state" {
  bucket = aws_s3_bucket.state.id

  rule {
    apply_server_side_encryption_by_default {
      # AES256 = AWS-managed keys (free, simple)
      # For higher security, use KMS (aws:kms) with your own key
      sse_algorithm = "AES256"
    }
  }
}


# =============================================================================
# S3 PUBLIC ACCESS BLOCK
# =============================================================================
# This is CRITICAL for security.
# 
# By default, S3 buckets can be made public (accidentally or intentionally).
# This block says "NO - never allow public access, no matter what".
#
# Many data breaches have happened due to misconfigured public S3 buckets.
# Always enable this for sensitive data like Terraform state.
# =============================================================================

resource "aws_s3_bucket_public_access_block" "state" {
  bucket = aws_s3_bucket.state.id

  # Block any ACLs (Access Control Lists) that would make objects public
  block_public_acls = true

  # Ignore any public ACLs that might exist
  ignore_public_acls = true

  # Block bucket policies that would allow public access
  block_public_policy = true

  # Restrict cross-account access even if bucket policy allows it
  restrict_public_buckets = true
}


# =============================================================================
# DYNAMODB TABLE - State Locking
# =============================================================================
# 
# THE PROBLEM:
# What happens if two people run `terraform apply` at the same time?
# They could both try to modify the same resources, causing conflicts
# and potentially corrupting the state file.
#
# THE SOLUTION: Locking
# Before Terraform modifies state, it writes a "lock" record to DynamoDB.
# If someone else tries to run Terraform, they see the lock and wait.
#
# BILLING MODE:
# - PAY_PER_REQUEST: You pay only for what you use (best for low traffic)
# - PROVISIONED: You pay for reserved capacity (better for high traffic)
#
# For Terraform locking, PAY_PER_REQUEST is ideal - locks are infrequent.
# =============================================================================

resource "aws_dynamodb_table" "locks" {
  name         = "${var.project_name}-locks"
  billing_mode = "PAY_PER_REQUEST"

  # Terraform expects a table with a partition key named "LockID"
  # This is a fixed requirement - don't change it!
  hash_key = "LockID"

  attribute {
    name = "LockID"
    type = "S" # S = String
  }

  # Optional: Add a tag for easy identification
  tags = {
    Purpose = "Terraform state locking"
  }
}
