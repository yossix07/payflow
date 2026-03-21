# Ledger Service

Immutable accounting log for all payment events in the system.

## Architecture

- **Pattern**: Event-driven append-only log
- **Database**: DynamoDB (ledger table - write-only)
- **Message Queue**: AWS SQS
- **Language**: Go 1.18

## Features

- ✅ Immutable event log (append-only)
- ✅ Records all payment lifecycle events
- ✅ Audit trail for financial transactions
- ✅ Query by payment ID
- ✅ Real-time event tracking

## API Endpoints

### Get Recent Entries

```bash
GET /ledger?limit=100

Response:
[
  {
    "entry_id": "uuid",
    "payment_id": "payment-123",
    "event_type": "PaymentStarted",
    "amount": 99.99,
    "user_id": "user-456",
    "description": "Payment initiated",
    "timestamp": "2024-01-01T00:00:00Z"
  }
]
```

### Get Entries by Payment

```bash
GET /ledger/payment/{payment_id}

Response: [array of ledger entries]
```

### Health Check

```bash
GET /healthz
```

## Environment Variables

| Variable       | Description           | Example          |
| -------------- | --------------------- | ---------------- |
| `AWS_REGION`   | AWS region            | `us-east-1`      |
| `QUEUE_URL`    | SQS queue URL         | `https://sqs...` |
| `LEDGER_TABLE` | DynamoDB ledger table | `dev-ledger`     |

## Event Handling

**Consumes (records all events):**

- `PaymentStarted`
- `FundsReserved`
- `InsufficientFunds`
- `PaymentSucceeded`
- `PaymentFailed`

**Produces:** None (read-only log)

## Ledger Entry Structure

```json
{
  "entry_id": "unique-uuid",
  "payment_id": "payment-123",
  "event_type": "PaymentSucceeded",
  "amount": 99.99,
  "user_id": "user-456",
  "description": "Payment processed successfully (txn: txn-789)",
  "timestamp": "2024-01-01T00:00:00.000Z"
}
```

## Immutability

- **Append-only**: Entries are never updated or deleted
- **Audit trail**: Complete history of all payment events
- **No outbox**: Ledger only consumes events, never produces them

## Local Development

```bash
go mod download

export AWS_REGION=us-east-1
export QUEUE_URL=https://sqs.us-east-1.amazonaws.com/xxx/dev-payment-events
export LEDGER_TABLE=dev-ledger

go run main.go
```

## Docker Build

```bash
docker build -t ledger-service .
```

## Kubernetes Deployment

```bash
kubectl apply -f k8s/deployment.yaml
```
