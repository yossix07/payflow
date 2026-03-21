#!/bin/bash

echo "Initializing LocalStack resources..."

# Create SQS queues (one per service, outbox workers fan-out to all)
awslocal sqs create-queue --queue-name payment-queue
awslocal sqs create-queue --queue-name ledger-queue
awslocal sqs create-queue --queue-name wallet-queue
awslocal sqs create-queue --queue-name gateway-queue
awslocal sqs create-queue --queue-name notification-queue

# Create DynamoDB Tables

# Payments
awslocal dynamodb create-table \
    --table-name payments \
    --attribute-definitions AttributeName=payment_id,AttributeType=S \
    --key-schema AttributeName=payment_id,KeyType=HASH \
    --billing-mode PAY_PER_REQUEST

# Idempotency
awslocal dynamodb create-table \
    --table-name idempotency \
    --attribute-definitions AttributeName=idempotency_key,AttributeType=S \
    --key-schema AttributeName=idempotency_key,KeyType=HASH \
    --billing-mode PAY_PER_REQUEST

# Outboxes (Need GSI for published-index)
# services: payment, wallet, gateway
awslocal dynamodb create-table \
    --table-name payment-outbox \
    --attribute-definitions \
        AttributeName=message_id,AttributeType=S \
        AttributeName=published,AttributeType=N \
    --key-schema AttributeName=message_id,KeyType=HASH \
    --global-secondary-indexes \
        "[{\"IndexName\": \"published-index\",\"KeySchema\":[{\"AttributeName\":\"published\",\"KeyType\":\"HASH\"}],\"Projection\":{\"ProjectionType\":\"ALL\"}}]" \
    --billing-mode PAY_PER_REQUEST

awslocal dynamodb create-table \
    --table-name wallet-outbox \
    --attribute-definitions \
        AttributeName=message_id,AttributeType=S \
        AttributeName=published,AttributeType=N \
    --key-schema AttributeName=message_id,KeyType=HASH \
    --global-secondary-indexes \
        "[{\"IndexName\": \"published-index\",\"KeySchema\":[{\"AttributeName\":\"published\",\"KeyType\":\"HASH\"}],\"Projection\":{\"ProjectionType\":\"ALL\"}}]" \
    --billing-mode PAY_PER_REQUEST

awslocal dynamodb create-table \
    --table-name gateway-outbox \
    --attribute-definitions \
        AttributeName=message_id,AttributeType=S \
        AttributeName=published,AttributeType=N \
    --key-schema AttributeName=message_id,KeyType=HASH \
    --global-secondary-indexes \
        "[{\"IndexName\": \"published-index\",\"KeySchema\":[{\"AttributeName\":\"published\",\"KeyType\":\"HASH\"}],\"Projection\":{\"ProjectionType\":\"ALL\"}}]" \
    --billing-mode PAY_PER_REQUEST

# Ledger
awslocal dynamodb create-table \
    --table-name ledger \
    --attribute-definitions AttributeName=entry_id,AttributeType=S \
    --key-schema AttributeName=entry_id,KeyType=HASH \
    --billing-mode PAY_PER_REQUEST

# Wallets
awslocal dynamodb create-table \
    --table-name wallets \
    --attribute-definitions AttributeName=user_id,AttributeType=S \
    --key-schema AttributeName=user_id,KeyType=HASH \
    --billing-mode PAY_PER_REQUEST

# Reservations
awslocal dynamodb create-table \
    --table-name reservations \
    --attribute-definitions \
        AttributeName=reservation_id,AttributeType=S \
        AttributeName=payment_id,AttributeType=S \
    --key-schema AttributeName=reservation_id,KeyType=HASH \
    --global-secondary-indexes \
        "[{\"IndexName\": \"payment-index\",\"KeySchema\":[{\"AttributeName\":\"payment_id\",\"KeyType\":\"HASH\"}],\"Projection\":{\"ProjectionType\":\"ALL\"}}]" \
    --billing-mode PAY_PER_REQUEST

# Transactions
awslocal dynamodb create-table \
    --table-name transactions \
    --attribute-definitions AttributeName=transaction_id,AttributeType=S \
    --key-schema AttributeName=transaction_id,KeyType=HASH \
    --billing-mode PAY_PER_REQUEST

# Gateway Idempotency
awslocal dynamodb create-table \
    --table-name gateway-idempotency \
    --attribute-definitions AttributeName=payment_id,AttributeType=S \
    --key-schema AttributeName=payment_id,KeyType=HASH \
    --billing-mode PAY_PER_REQUEST

echo "LocalStack resources initialized!"
