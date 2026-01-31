# =============================================================================
# VARIABLES
# =============================================================================
# Variables make your Terraform code reusable and configurable.
# Instead of hardcoding values, we define them here and reference them
# elsewhere with: var.variable_name
#
# WAYS TO SET VARIABLES (in order of precedence):
#   1. Command line: terraform apply -var="aws_region=eu-west-1"
#   2. .tfvars file: terraform apply -var-file="prod.tfvars"
#   3. Environment: TF_VAR_aws_region=eu-west-1
#   4. Default value (defined below)
#
# TIP: Never put secrets in variables with defaults. Use environment
# variables or a secrets manager instead.
# =============================================================================

variable "aws_region" {
  description = "The AWS region where resources will be created"
  type        = string
  default     = "us-east-1"

  # -------------------------------------------------------------------------
  # VALIDATION (optional but recommended)
  # -------------------------------------------------------------------------
  # You can add rules to catch mistakes early.
  # This prevents typos like "us-eats-1" from causing confusing errors.
  # -------------------------------------------------------------------------
  validation {
    condition     = can(regex("^(us|eu|ap|sa|ca|me|af)-", var.aws_region))
    error_message = "Must be a valid AWS region (e.g., us-east-1, eu-west-1)."
  }
}

variable "project_name" {
  description = "Name prefix for all resources (used in bucket name, tags, etc.)"
  type        = string
  default     = "saas-cluster"

  validation {
    # S3 bucket names must be lowercase, 3-63 chars, no underscores
    condition     = can(regex("^[a-z0-9][a-z0-9-]{1,61}[a-z0-9]$", var.project_name))
    error_message = "Project name must be lowercase alphanumeric with hyphens, 3-63 characters."
  }
}
