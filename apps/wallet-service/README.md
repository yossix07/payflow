# Wallet Service

User balance management and fund reservation service for payment processing.

## Architecture

- **Pattern**: Event-driven participant (saga participant)
- **Database**: DynamoDB (wallets, reservations, outbox tables)
- **Message Queue**: AWS SQS
- **Language**: Go 1.18

## Features

- ✅ User wallet/balance management
- ✅ Fund reservation logic
- ✅ Insufficient funds detection
- ✅ Compensation actions (release funds)
- ✅ Transactional Outbox pattern
- ✅ Event-driven communication

## API Endpoints

### Get Wallet

```bash
GET /wallets/{user_id}

Response:
{
  "user_id": "user-123",
  "balance": 1000.00
}
```

### Credit Wallet

```bash
POST /wallets/{user_id}/credit
Content-Type: application/json

Body:
{
  "amount": 500.00
}

Response:
{
  "user_id": "user-123",
  "balance": 1500.00
}
```

### Health Check

```bash
GET /healthz
```

## Environment Variables

| Variable             | Description                     | Example             |
| -------------------- | ------------------------------- | ------------------- |
| `AWS_REGION`         | AWS region                      | `us-east-1`         |
| `QUEUE_URL`          | SQS queue URL                   | `https://sqs...`    |
| `WALLETS_TABLE`      | DynamoDB table for wallets      | `dev-wallets`       |
| `RESERVATIONS_TABLE` | DynamoDB table for reservations | `dev-reservations`  |
| `OUTBOX_TABLE`       | DynamoDB table for outbox       | `dev-wallet-outbox` |

## Event Handling

**Consumes:**

- `ReserveFunds` - Attempts to reserve funds for a payment
- `ReleaseFunds` - Compensation action to release reserved funds

**Produces:**

- `FundsReserved` - Funds successfully reserved
- `InsufficientFunds` - Not enough balance
- `FundsReleased` - Funds released back to wallet

## Fund Reservation Flow

1. Receives `ReserveFunds` event with payment ID, user ID, and amount
2. Checks wallet balance
3. If sufficient:
   - Creates reservation record (with 24h TTL)
   - Deducts amount from wallet balance
   - Publishes `FundsReserved` event
4. If insufficient:
   - Publishes `InsufficientFunds` event

## Compensation Flow

1. Receives `ReleaseFunds` event (e.g., payment gateway failure)
2. Finds active reservation by payment ID
3. Returns reserved amount to wallet balance
4. Marks reservation as RELEASED
5. Publishes `FundsReleased` event

## Local Development

```bash
go mod download

export AWS_REGION=us-east-1
export QUEUE_URL=https://sqs.us-east-1.amazonaws.com/xxx/dev-payment-events
export WALLETS_TABLE=dev-wallets
export RESERVATIONS_TABLE=dev-reservations
export OUTBOX_TABLE=dev-wallet-outbox

go run main.go
```

## Docker Build

```bash
docker build -t wallet-service .
```

## Kubernetes Deployment

```bash
kubectl apply -f k8s/deployment.yaml
```
