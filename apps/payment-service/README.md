# Payment Service

Event-driven payment processing service using the Saga orchestration pattern.

## Architecture

- **Pattern**: Saga Orchestrator
- **Database**: DynamoDB (payments, idempotency, outbox tables)
- **Message Queue**: AWS SQS
- **Language**: Go 1.21

## Features

- ✅ Saga state machine with 7 states
- ✅ Transactional Outbox pattern for reliable event publishing
- ✅ Idempotency protection via `Idempotency-Key` header
- ✅ Event-driven communication with other services
- ✅ Background workers for outbox processing and event consumption

## API Endpoints

### Create Payment

```bash
POST /payments
Headers:
  Idempotency-Key: <unique-key>
  Content-Type: application/json

Body:
{
  "user_id": "user-123",
  "amount": 99.99
}
```

### Get Payment

```bash
GET /payments/{payment_id}
```

### Health Check

```bash
GET /healthz
```

## Environment Variables

| Variable            | Description                    | Example                   |
| ------------------- | ------------------------------ | ------------------------- |
| `AWS_REGION`        | AWS region                     | `us-east-1`               |
| `QUEUE_URL`         | SQS queue URL                  | `https://sqs...`          |
| `PAYMENTS_TABLE`    | DynamoDB table for payments    | `dev-payments`            |
| `IDEMPOTENCY_TABLE` | DynamoDB table for idempotency | `dev-payment-idempotency` |
| `OUTBOX_TABLE`      | DynamoDB table for outbox      | `dev-payment-outbox`      |

## Local Development

```bash
# Install dependencies
go mod download

# Run the service
export AWS_REGION=us-east-1
export QUEUE_URL=https://sqs.us-east-1.amazonaws.com/xxx/dev-payment-events
export PAYMENTS_TABLE=dev-payments
export IDEMPOTENCY_TABLE=dev-payment-idempotency
export OUTBOX_TABLE=dev-payment-outbox

go run main.go
```

## Docker Build

```bash
docker build -t payment-service .
```

## Kubernetes Deployment

```bash
kubectl apply -f k8s/deployment.yaml
```

## Saga Flow

1. **PENDING** → User creates payment
2. **PENDING** → Publishes `ReserveFunds` event
3. **FUNDS_RESERVED** → Receives `FundsReserved` event
4. **PROCESSING** → Publishes `ProcessPayment` event
5. **COMPLETED** → Receives `PaymentSucceeded` event
6. **FAILED** → Receives `PaymentFailed` or `InsufficientFunds` event

## Event Types

**Produced:**

- `PaymentStarted`
- `ReserveFunds`
- `SendNotification`

**Consumed:**

- `FundsReserved`
- `InsufficientFunds`
- `PaymentSucceeded`
- `PaymentFailed`
