# Gateway Service

Mock payment gateway service that simulates external payment processing.

## Architecture

- **Pattern**: Event-driven mock processor
- **Database**: DynamoDB (gateway_transactions, gateway_outbox tables)
- **Message Queue**: AWS SQS
- **Language**: Node.js 18

## Features

- ✅ Mock payment processing with configurable success rate (default 80%)
- ✅ Simulated processing delay (500-2500ms)
- ✅ Transaction logging
- ✅ Transactional Outbox pattern
- ✅ Event-driven communication

## API Endpoints

### Health Check

```bash
GET /healthz

Response:
{
  "service": "Gateway Service",
  "status": "healthy",
  "timestamp": "2024-01-01T00:00:00.000Z"
}
```

## Environment Variables

| Variable             | Description                    | Example                    |
| -------------------- | ------------------------------ | -------------------------- |
| `AWS_REGION`         | AWS region                     | `us-east-1`                |
| `QUEUE_URL`          | SQS queue URL                  | `https://sqs...`           |
| `TRANSACTIONS_TABLE` | DynamoDB transactions table    | `dev-gateway-transactions` |
| `OUTBOX_TABLE`       | DynamoDB outbox table          | `dev-gateway-outbox`       |
| `SUCCESS_RATE`       | Payment success rate (0.0-1.0) | `0.8`                      |
| `PORT`               | HTTP port                      | `8080`                     |

## Event Handling

**Consumes:**

- `ProcessPayment` - Processes a payment through the mock gateway

**Produces:**

- `PaymentSucceeded` - Payment was successful
- `PaymentFailed` - Payment failed (gateway declined)

## Payment Processing Flow

1. Receives `ProcessPayment` event with payment details
2. Simulates processing delay (random 500-2500ms)
3. Randomly succeeds or fails based on `SUCCESS_RATE`
4. Saves transaction record with gateway response
5. Publishes `PaymentSucceeded` or `PaymentFailed` event

## Mock Gateway Behavior

**Success (80% by default):**

- Gateway code: `00`
- Message: `Approved`
- Publishes `PaymentSucceeded` with `transaction_id`

**Failure (20% by default):**

- Gateway code: `E01`
- Message: `Declined by issuer`
- Publishes `PaymentFailed` with reason

## Transaction Record

Saved to DynamoDB:

```json
{
  "transaction_id": "uuid",
  "payment_id": "payment-123",
  "user_id": "user-456",
  "amount": 99.99,
  "status": "SUCCESS" | "FAILED",
  "gateway_response": {
    "code": "00",
    "message": "Approved"
  },
  "created_at": "2024-01-01T00:00:00.000Z"
}
```

## Local Development

```bash
npm install

export AWS_REGION=us-east-1
export QUEUE_URL=https://sqs.us-east-1.amazonaws.com/xxx/dev-payment-events
export TRANSACTIONS_TABLE=dev-gateway-transactions
export OUTBOX_TABLE=dev-gateway-outbox
export SUCCESS_RATE=0.8

npm start
```

## Docker Build

```bash
docker build -t gateway-service .
```

## Kubernetes Deployment

```bash
kubectl apply -f k8s/deployment.yaml
```

## Adjusting Success Rate

For testing failure scenarios:

```bash
# 50% success rate
export SUCCESS_RATE=0.5

# Always succeed
export SUCCESS_RATE=1.0

# Always fail
export SUCCESS_RATE=0.0
```
