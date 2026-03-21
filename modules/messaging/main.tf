# =============================================================================
# MODULE: MESSAGING
# =============================================================================
# This module creates the messaging infrastructure for the payment processing
# system, including SQS queues and DynamoDB tables for each service.
# =============================================================================

# -----------------------------------------------------------------------------
# SQS: Main Event Queue
# -----------------------------------------------------------------------------
# Single queue with event-type routing. All services publish and consume from
# this queue, filtering by event type.
# -----------------------------------------------------------------------------

resource "aws_sqs_queue" "payment_events_dlq" {
  name                      = "${var.env}-payment-events-dlq"
  message_retention_seconds = 1209600 # 14 days - keep failed messages for debugging

  tags = {
    Name        = "${var.env}-payment-events-dlq"
    Environment = var.env
    ManagedBy   = "Terraform"
    Purpose     = "Dead letter queue for failed payment events"
  }
}

resource "aws_sqs_queue" "payment_events" {
  name                       = "${var.env}-payment-events"
  visibility_timeout_seconds = 30
  message_retention_seconds  = 345600 # 4 days
  receive_wait_time_seconds  = 20     # Long polling for efficiency

  redrive_policy = jsonencode({
    deadLetterTargetArn = aws_sqs_queue.payment_events_dlq.arn
    maxReceiveCount     = 3 # Move to DLQ after 3 failed attempts
  })

  tags = {
    Name        = "${var.env}-payment-events"
    Environment = var.env
    ManagedBy   = "Terraform"
    Purpose     = "Main event bus for payment processing"
  }
}

# -----------------------------------------------------------------------------
# DynamoDB: Payment Service Tables
# -----------------------------------------------------------------------------

resource "aws_dynamodb_table" "payments" {
  name         = "${var.env}-payments"
  billing_mode = "PAY_PER_REQUEST" # On-demand for variable load
  hash_key     = "payment_id"

  attribute {
    name = "payment_id"
    type = "S"
  }

  attribute {
    name = "state"
    type = "S"
  }

  attribute {
    name = "updated_at"
    type = "S"
  }

  global_secondary_index {
    name            = "state-updated-index"
    hash_key        = "state"
    range_key       = "updated_at"
    projection_type = "ALL"
  }

  # TTL for automatic cleanup of old completed payments (optional)
  ttl {
    attribute_name = "expires_at"
    enabled        = true
  }

  tags = {
    Name        = "${var.env}-payments"
    Environment = var.env
    ManagedBy   = "Terraform"
    Service     = "payment-service"
  }
}

resource "aws_dynamodb_table" "payment_idempotency" {
  name         = "${var.env}-payment-idempotency"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "idempotency_key"

  attribute {
    name = "idempotency_key"
    type = "S"
  }

  # TTL for automatic cleanup (24 hours)
  ttl {
    attribute_name = "expires_at"
    enabled        = true
  }

  tags = {
    Name        = "${var.env}-payment-idempotency"
    Environment = var.env
    ManagedBy   = "Terraform"
    Service     = "payment-service"
  }
}

resource "aws_dynamodb_table" "payment_outbox" {
  name         = "${var.env}-payment-outbox"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "message_id"

  attribute {
    name = "message_id"
    type = "S"
  }

  attribute {
    name = "published"
    type = "N"
  }

  # GSI for querying unpublished messages
  global_secondary_index {
    name            = "published-index"
    hash_key        = "published"
    projection_type = "ALL"
  }

  tags = {
    Name        = "${var.env}-payment-outbox"
    Environment = var.env
    ManagedBy   = "Terraform"
    Service     = "payment-service"
  }
}

# -----------------------------------------------------------------------------
# DynamoDB: Wallet Service Tables
# -----------------------------------------------------------------------------

resource "aws_dynamodb_table" "wallets" {
  name         = "${var.env}-wallets"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "user_id"

  attribute {
    name = "user_id"
    type = "S"
  }

  tags = {
    Name        = "${var.env}-wallets"
    Environment = var.env
    ManagedBy   = "Terraform"
    Service     = "wallet-service"
  }
}

resource "aws_dynamodb_table" "reservations" {
  name         = "${var.env}-reservations"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "reservation_id"

  attribute {
    name = "reservation_id"
    type = "S"
  }

  attribute {
    name = "payment_id"
    type = "S"
  }

  # GSI to lookup reservation by payment
  global_secondary_index {
    name            = "payment-index"
    hash_key        = "payment_id"
    projection_type = "ALL"
  }

  # TTL for expired reservations
  ttl {
    attribute_name = "expires_at"
    enabled        = true
  }

  tags = {
    Name        = "${var.env}-reservations"
    Environment = var.env
    ManagedBy   = "Terraform"
    Service     = "wallet-service"
  }
}

resource "aws_dynamodb_table" "wallet_outbox" {
  name         = "${var.env}-wallet-outbox"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "message_id"

  attribute {
    name = "message_id"
    type = "S"
  }

  attribute {
    name = "published"
    type = "N"
  }

  global_secondary_index {
    name            = "published-index"
    hash_key        = "published"
    projection_type = "ALL"
  }

  tags = {
    Name        = "${var.env}-wallet-outbox"
    Environment = var.env
    ManagedBy   = "Terraform"
    Service     = "wallet-service"
  }
}

# -----------------------------------------------------------------------------
# DynamoDB: Gateway Service Tables
# -----------------------------------------------------------------------------

resource "aws_dynamodb_table" "gateway_transactions" {
  name         = "${var.env}-gateway-transactions"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "transaction_id"

  attribute {
    name = "transaction_id"
    type = "S"
  }

  attribute {
    name = "payment_id"
    type = "S"
  }

  global_secondary_index {
    name            = "payment-index"
    hash_key        = "payment_id"
    projection_type = "ALL"
  }

  tags = {
    Name        = "${var.env}-gateway-transactions"
    Environment = var.env
    ManagedBy   = "Terraform"
    Service     = "gateway-service"
  }
}

resource "aws_dynamodb_table" "gateway_outbox" {
  name         = "${var.env}-gateway-outbox"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "message_id"

  attribute {
    name = "message_id"
    type = "S"
  }

  attribute {
    name = "published"
    type = "N"
  }

  global_secondary_index {
    name            = "published-index"
    hash_key        = "published"
    projection_type = "ALL"
  }

  tags = {
    Name        = "${var.env}-gateway-outbox"
    Environment = var.env
    ManagedBy   = "Terraform"
    Service     = "gateway-service"
  }
}

resource "aws_dynamodb_table" "gateway_idempotency" {
  name         = "${var.env}-gateway-idempotency"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "payment_id"

  attribute {
    name = "payment_id"
    type = "S"
  }

  ttl {
    attribute_name = "expires_at"
    enabled        = true
  }

  tags = {
    Name        = "${var.env}-gateway-idempotency"
    Environment = var.env
    ManagedBy   = "Terraform"
    Service     = "gateway-service"
  }
}

# -----------------------------------------------------------------------------
# DynamoDB: Ledger Service Tables
# -----------------------------------------------------------------------------

resource "aws_dynamodb_table" "ledger" {
  name         = "${var.env}-ledger"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "payment_id"
  range_key    = "entry_id"

  attribute {
    name = "payment_id"
    type = "S"
  }

  attribute {
    name = "entry_id"
    type = "S"
  }

  # Ledger entries are immutable - no TTL

  tags = {
    Name        = "${var.env}-ledger"
    Environment = var.env
    ManagedBy   = "Terraform"
    Service     = "ledger-service"
  }
}

resource "aws_dynamodb_table" "ledger_outbox" {
  name         = "${var.env}-ledger-outbox"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "message_id"

  attribute {
    name = "message_id"
    type = "S"
  }

  attribute {
    name = "published"
    type = "N"
  }

  global_secondary_index {
    name            = "published-index"
    hash_key        = "published"
    projection_type = "ALL"
  }

  tags = {
    Name        = "${var.env}-ledger-outbox"
    Environment = var.env
    ManagedBy   = "Terraform"
    Service     = "ledger-service"
  }
}

# -----------------------------------------------------------------------------
# IAM: Service Access Policy
# -----------------------------------------------------------------------------
# Policy document that grants services access to SQS and DynamoDB.
# This will be attached to EKS node roles or service-specific roles.
# -----------------------------------------------------------------------------

data "aws_caller_identity" "current" {}
data "aws_region" "current" {}

resource "aws_iam_policy" "messaging_access" {
  name        = "${var.env}-messaging-access"
  description = "Allows services to access SQS queues and DynamoDB tables"

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Sid    = "SQSAccess"
        Effect = "Allow"
        Action = [
          "sqs:SendMessage",
          "sqs:ReceiveMessage",
          "sqs:DeleteMessage",
          "sqs:GetQueueAttributes",
          "sqs:GetQueueUrl"
        ]
        Resource = [
          aws_sqs_queue.payment_events.arn,
          aws_sqs_queue.payment_events_dlq.arn
        ]
      },
      {
        Sid    = "DynamoDBAccess"
        Effect = "Allow"
        Action = [
          "dynamodb:PutItem",
          "dynamodb:GetItem",
          "dynamodb:UpdateItem",
          "dynamodb:DeleteItem",
          "dynamodb:Query",
          "dynamodb:Scan",
          "dynamodb:BatchWriteItem",
          "dynamodb:BatchGetItem"
        ]
        Resource = [
          aws_dynamodb_table.payments.arn,
          "${aws_dynamodb_table.payments.arn}/index/*",
          aws_dynamodb_table.payment_idempotency.arn,
          aws_dynamodb_table.payment_outbox.arn,
          "${aws_dynamodb_table.payment_outbox.arn}/index/*",
          aws_dynamodb_table.wallets.arn,
          aws_dynamodb_table.reservations.arn,
          "${aws_dynamodb_table.reservations.arn}/index/*",
          aws_dynamodb_table.wallet_outbox.arn,
          "${aws_dynamodb_table.wallet_outbox.arn}/index/*",
          aws_dynamodb_table.gateway_transactions.arn,
          "${aws_dynamodb_table.gateway_transactions.arn}/index/*",
          aws_dynamodb_table.gateway_outbox.arn,
          "${aws_dynamodb_table.gateway_outbox.arn}/index/*",
          aws_dynamodb_table.gateway_idempotency.arn,
          aws_dynamodb_table.ledger.arn,
          aws_dynamodb_table.ledger_outbox.arn,
          "${aws_dynamodb_table.ledger_outbox.arn}/index/*"
        ]
      }
    ]
  })

  tags = {
    Name        = "${var.env}-messaging-access"
    Environment = var.env
    ManagedBy   = "Terraform"
  }
}
