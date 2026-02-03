# =============================================================================
# Outputs for the Messaging Module
# =============================================================================

# -----------------------------------------------------------------------------
# SQS Outputs
# -----------------------------------------------------------------------------

output "payment_events_queue_url" {
  description = "URL of the main payment events SQS queue"
  value       = aws_sqs_queue.payment_events.url
}

output "payment_events_queue_arn" {
  description = "ARN of the main payment events SQS queue"
  value       = aws_sqs_queue.payment_events.arn
}

output "payment_events_dlq_url" {
  description = "URL of the dead letter queue"
  value       = aws_sqs_queue.payment_events_dlq.url
}

output "payment_events_dlq_arn" {
  description = "ARN of the dead letter queue"
  value       = aws_sqs_queue.payment_events_dlq.arn
}

# -----------------------------------------------------------------------------
# DynamoDB Table Names (for service configuration)
# -----------------------------------------------------------------------------

output "payments_table_name" {
  description = "Name of the payments DynamoDB table"
  value       = aws_dynamodb_table.payments.name
}

output "payment_idempotency_table_name" {
  description = "Name of the payment idempotency DynamoDB table"
  value       = aws_dynamodb_table.payment_idempotency.name
}

output "payment_outbox_table_name" {
  description = "Name of the payment outbox DynamoDB table"
  value       = aws_dynamodb_table.payment_outbox.name
}

output "wallets_table_name" {
  description = "Name of the wallets DynamoDB table"
  value       = aws_dynamodb_table.wallets.name
}

output "reservations_table_name" {
  description = "Name of the reservations DynamoDB table"
  value       = aws_dynamodb_table.reservations.name
}

output "wallet_outbox_table_name" {
  description = "Name of the wallet outbox DynamoDB table"
  value       = aws_dynamodb_table.wallet_outbox.name
}

output "gateway_transactions_table_name" {
  description = "Name of the gateway transactions DynamoDB table"
  value       = aws_dynamodb_table.gateway_transactions.name
}

output "gateway_outbox_table_name" {
  description = "Name of the gateway outbox DynamoDB table"
  value       = aws_dynamodb_table.gateway_outbox.name
}

output "ledger_table_name" {
  description = "Name of the ledger DynamoDB table"
  value       = aws_dynamodb_table.ledger.name
}

output "ledger_outbox_table_name" {
  description = "Name of the ledger outbox DynamoDB table"
  value       = aws_dynamodb_table.ledger_outbox.name
}

# -----------------------------------------------------------------------------
# IAM Policy ARN
# -----------------------------------------------------------------------------

output "messaging_access_policy_arn" {
  description = "ARN of the IAM policy for accessing messaging resources"
  value       = aws_iam_policy.messaging_access.arn
}
