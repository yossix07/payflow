# =============================================================================
# OUTPUTS
# =============================================================================
# Outputs are values that Terraform displays after `terraform apply`.
# They're useful for:
#   1. Displaying important info to the user
#   2. Passing values to other Terraform configurations
#   3. Accessing values via `terraform output` command
#
# SYNTAX:
#   output "name" {
#     value       = <expression>
#     description = "Human-readable description"
#     sensitive   = true/false  # Hide value in logs if sensitive
#   }
# =============================================================================


output "state_bucket_name" {
  description = "Name of the S3 bucket storing Terraform state"
  value       = aws_s3_bucket.state.bucket
}

output "state_bucket_arn" {
  description = "ARN (Amazon Resource Name) of the state bucket"
  value       = aws_s3_bucket.state.arn
}

output "lock_table_name" {
  description = "Name of the DynamoDB table used for state locking"
  value       = aws_dynamodb_table.locks.name
}

output "lock_table_arn" {
  description = "ARN of the DynamoDB lock table"
  value       = aws_dynamodb_table.locks.arn
}


# =============================================================================
# BACKEND CONFIG OUTPUT
# =============================================================================
# This is the most useful output!
# It generates the exact configuration you need to paste into your
# envs/dev/provider.tf file.
#
# After running `terraform apply`, run:
#   terraform output -raw backend_config
#
# Then copy the output into your provider.tf file.
# =============================================================================

output "backend_config" {
  description = "Copy this into your envs/dev/provider.tf terraform {} block"
  value       = <<-EOT

    # ==========================================================================
    # COPY EVERYTHING BELOW INTO YOUR terraform {} BLOCK IN envs/dev/provider.tf
    # ==========================================================================
    backend "s3" {
      bucket         = "${aws_s3_bucket.state.bucket}"
      key            = "dev/terraform.tfstate"
      region         = "${var.aws_region}"
      dynamodb_table = "${aws_dynamodb_table.locks.name}"
      encrypt        = true
    }
  EOT
}


# =============================================================================
# NEXT STEPS OUTPUT
# =============================================================================
# A friendly reminder of what to do after the bootstrap is complete
# =============================================================================

output "next_steps" {
  description = "Instructions for what to do after bootstrapping"
  value       = <<-EOT

    ============================================================================
    SUCCESS! Your Terraform state backend is ready.
    ============================================================================

    WHAT WAS CREATED:
      - S3 Bucket: ${aws_s3_bucket.state.bucket}
      - DynamoDB Table: ${aws_dynamodb_table.locks.name}

    NEXT STEPS:
      1. Run: terraform output -raw backend_config
      2. Copy the output into envs/dev/provider.tf inside the terraform {} block
      3. Navigate to envs/dev: cd ../dev
      4. Initialize with remote backend: terraform init

    IMPORTANT:
      - The state for THIS bootstrap is stored locally (terraform.tfstate)
      - You can safely commit the bootstrap state to git (it's not secret)
      - The state for your MAIN infrastructure will be stored in S3

    ============================================================================
  EOT
}
