# Scalability & Consistency Improvements — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix critical data corruption, throughput bottlenecks, reliability gaps, and add production-ready infrastructure-as-code for a distributed payment processing platform.

**Architecture:** Event-driven microservices (Go + Node.js) communicating via SQS with DynamoDB for persistence. Saga orchestration pattern with transactional outbox. Changes are surgical fixes to existing code — no rearchitecture.

**Tech Stack:** Go 1.21+ (log/slog, sync.WaitGroup), Node.js 18 (Express, AWS SDK v3), Terraform, Kubernetes

**Spec:** `docs/superpowers/specs/2026-03-21-scalability-improvements-design.md`

---

## File Structure

### New Files
- `apps/payment-service/internal/saga/timeout_scanner.go` — Saga timeout scanner goroutine
- `apps/gateway-service/src/repository/idempotencyRepository.js` — Gateway dedup table operations
- `apps/gateway-service/src/utils/logger.js` — Structured JSON logger for Node.js services
- `apps/gateway-service/src/utils/shutdown.js` — Shared shutdown flag (avoids circular deps)
- `apps/notification-service/src/utils/logger.js` — Shared structured logger (copy)
- `apps/notification-service/src/utils/shutdown.js` — Shared shutdown flag (copy)
- `envs/prod/main.tf` — Production Terraform root module
- `envs/prod/variables.tf` — Production variable definitions
- `envs/prod/provider.tf` — Production AWS/Helm provider config
- `envs/prod/terraform.tfvars` — Production variable values
- `k8s/prod/hpa.yaml` — HorizontalPodAutoscaler for all services
- `k8s/prod/pdb.yaml` — PodDisruptionBudget for all services

### Modified Files (by task)
**Task 1 (Consumer loop fix):** `apps/payment-service/internal/handlers/consumer.go`, `apps/wallet-service/internal/handlers/consumer.go`, `apps/ledger-service/internal/handlers/consumer.go`
**Task 2 (Ledger query):** `apps/ledger-service/internal/repository/dynamodb.go`
**Task 3 (Wallet locking):** `apps/wallet-service/internal/repository/dynamodb.go`, `apps/wallet-service/internal/repository/types.go`, `apps/wallet-service/internal/handlers/wallet.go`, `apps/wallet-service/internal/handlers/consumer.go`
**Task 4 (Outbox error handling):** `apps/payment-service/pkg/outbox/worker.go`, `apps/wallet-service/pkg/outbox/worker.go`, `apps/gateway-service/src/workers/outboxWorker.js`, `apps/gateway-service/src/repository/outboxRepository.js`
**Task 5 (Outbox backpressure):** Same outbox worker files as Task 4
**Task 6 (Gateway idempotency):** `modules/messaging/main.tf`, `apps/gateway-service/src/consumers/eventConsumer.js`, `apps/gateway-service/src/services/paymentProcessor.js`
**Task 7 (Graceful shutdown):** `apps/payment-service/main.go`, `apps/wallet-service/main.go`, `apps/ledger-service/main.go`, `apps/gateway-service/src/index.js`, `apps/gateway-service/src/consumers/eventConsumer.js`, `apps/gateway-service/src/workers/outboxWorker.js`, `apps/notification-service/src/index.js`, `apps/notification-service/src/consumers/eventConsumer.js`
**Task 8 (Saga timeout):** `apps/payment-service/internal/model/types.go`, `apps/payment-service/internal/saga/orchestrator.go`, `modules/messaging/main.tf`
**Task 9 (SSE management):** `apps/notification-service/src/sse/sseManager.js`, `apps/notification-service/src/index.js`
**Task 10 (Structured logging):** All `main.go` files, all Node.js `index.js` files, all handler/consumer files
**Task 11 (Health checks):** All `main.go` files, all Node.js `index.js` files, all K8s `deployment.yaml` files
**Task 12 (Metrics):** `apps/payment-service/main.go`
**Task 13 (Prod infra):** New `envs/prod/` and `k8s/prod/` files

---

## Task 1: Fix SQS Consumer Busy-Loop (Go Services)

**Files:**
- Modify: `apps/payment-service/internal/handlers/consumer.go:28-50`
- Modify: `apps/wallet-service/internal/handlers/consumer.go:33-54`
- Modify: `apps/ledger-service/internal/handlers/consumer.go:24-47`

- [ ] **Step 1: Fix payment-service consumer loop**

Replace the `Start` method in `apps/payment-service/internal/handlers/consumer.go`. Remove the `default` case and add backoff on empty receives:

```go
func (ec *EventConsumer) Start(ctx context.Context) {
	log.Println("Starting event consumer...")

	for {
		select {
		case <-ctx.Done():
			log.Println("Event consumer stopped")
			return
		default:
		}

		messages, err := ec.queue.ReceiveMessages(ctx)
		if err != nil {
			log.Printf("Error receiving messages: %v", err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(1 * time.Second):
			}
			continue
		}

		for _, msg := range messages {
			if err := ec.handleMessage(ctx, msg); err != nil {
				log.Printf("Error handling message: %v", err)
			} else {
				ec.queue.DeleteMessage(ctx, msg.ReceiptHandle)
			}
		}

		// Backoff if no messages received (SQS long-poll already waits up to 20s)
		if len(messages) == 0 {
			select {
			case <-ctx.Done():
				return
			case <-time.After(1 * time.Second):
			}
		}
	}
}
```

Add `"time"` to the imports.

- [ ] **Step 2: Fix wallet-service consumer loop**

Apply the same pattern to `apps/wallet-service/internal/handlers/consumer.go`. The `Start` method has the identical `select { default: }` busy-loop. Replace with the same pattern as Step 1. Add `"time"` to imports.

- [ ] **Step 3: Fix ledger-service consumer loop**

Apply the same pattern to `apps/ledger-service/internal/handlers/consumer.go`. Same busy-loop fix. Add `"time"` to imports.

- [ ] **Step 4: Build all Go services**

Run: `cd c:/projects/my-saas-platform && go build ./...`
Expected: Clean build, no errors.

- [ ] **Step 5: Commit**

```bash
git add apps/payment-service/internal/handlers/consumer.go apps/wallet-service/internal/handlers/consumer.go apps/ledger-service/internal/handlers/consumer.go
git commit -m "fix: remove SQS consumer busy-loop in Go services

Replace select{default:} with proper backoff. ReceiveMessages
now blocks on SQS long-poll (20s) and backs off 1s on empty
receives. Eliminates CPU spin and unnecessary SQS API calls."
```

---

## Task 2: Fix Ledger Scan-to-Query

**Files:**
- Modify: `apps/ledger-service/internal/repository/dynamodb.go:78-97`

- [ ] **Step 1: Replace Scan with Query**

In `apps/ledger-service/internal/repository/dynamodb.go`, replace the `GetEntriesByPayment` method:

```go
func (r *DynamoDBRepository) GetEntriesByPayment(ctx context.Context, paymentID string) ([]*LedgerEntry, error) {
	result, err := r.client.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(r.ledgerTable),
		KeyConditionExpression: aws.String("payment_id = :payment_id"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":payment_id": &types.AttributeValueMemberS{Value: paymentID},
		},
	})

	if err != nil {
		return nil, err
	}

	var entries []*LedgerEntry
	if err := attributevalue.UnmarshalListOfMaps(result.Items, &entries); err != nil {
		return nil, err
	}

	return entries, nil
}
```

- [ ] **Step 2: Build and verify**

Run: `cd c:/projects/my-saas-platform/apps/ledger-service && go build ./...`
Expected: Clean build.

- [ ] **Step 3: Commit**

```bash
git add apps/ledger-service/internal/repository/dynamodb.go
git commit -m "fix: replace ledger Scan with Query on payment_id hash key

payment_id is already the table's hash key. Using Query instead of
Scan+FilterExpression reduces reads from full-table to exact-match."
```

---

## Task 3: Wallet Optimistic Locking

**Files:**
- Modify: `apps/wallet-service/internal/repository/types.go`
- Modify: `apps/wallet-service/internal/repository/dynamodb.go`
- Modify: `apps/wallet-service/internal/handlers/wallet.go`
- Modify: `apps/wallet-service/internal/handlers/consumer.go`

- [ ] **Step 1: Add Version field to Wallet struct**

In `apps/wallet-service/internal/repository/types.go`, add `Version` to the `Wallet` struct:

```go
type Wallet struct {
	UserID    string    `dynamodbav:"user_id"`
	Balance   float64   `dynamodbav:"balance"`
	Version   int       `dynamodbav:"version"`
	CreatedAt time.Time `dynamodbav:"created_at"`
	UpdatedAt time.Time `dynamodbav:"updated_at"`
}
```

- [ ] **Step 2: Add conditional write to UpdateWallet**

In `apps/wallet-service/internal/repository/dynamodb.go`, replace the `UpdateWallet` method to use a version-based conditional write:

```go
func (r *DynamoDBRepository) UpdateWallet(ctx context.Context, wallet *Wallet) error {
	_, err := r.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(r.walletsTable),
		Key: map[string]types.AttributeValue{
			"user_id": &types.AttributeValueMemberS{Value: wallet.UserID},
		},
		UpdateExpression: aws.String("SET balance = :balance, updated_at = :updated_at, version = :new_version"),
		ConditionExpression: aws.String("attribute_not_exists(version) OR version = :expected_version"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":balance":          &types.AttributeValueMemberN{Value: fmt.Sprintf("%f", wallet.Balance)},
			":updated_at":       &types.AttributeValueMemberS{Value: wallet.UpdatedAt.Format(time.RFC3339)},
			":new_version":      &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", wallet.Version+1)},
			":expected_version": &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", wallet.Version)},
		},
	})

	return err
}
```

- [ ] **Step 3: Add idempotent CreateReservation**

In `apps/wallet-service/internal/repository/dynamodb.go`, update `CreateReservation` with a conditional put:

```go
func (r *DynamoDBRepository) CreateReservation(ctx context.Context, reservation *Reservation) error {
	av, err := attributevalue.MarshalMap(reservation)
	if err != nil {
		return err
	}

	_, err = r.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName:           aws.String(r.reservationsTable),
		Item:                av,
		ConditionExpression: aws.String("attribute_not_exists(reservation_id)"),
	})

	// If reservation already exists (from a prior retry), that's OK
	var condErr *types.ConditionalCheckFailedException
	if errors.As(err, &condErr) {
		return nil
	}

	return err
}
```

Add `"errors"` and `"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"` to imports (types is already imported).
Add `"errors"` to imports.

- [ ] **Step 4: Add retry helper and update wallet handlers**

In `apps/wallet-service/internal/handlers/wallet.go`, add a retry helper and update `CreditWallet`:

```go
import (
	"encoding/json"
	"errors"
	"log"
	"math/rand"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/gorilla/mux"
	"github.com/my-saas-platform/wallet-service/internal/repository"
	"github.com/my-saas-platform/wallet-service/pkg/outbox"
)

// updateWalletWithRetry wraps a read-modify-write cycle with optimistic locking retry.
func updateWalletWithRetry(ctx context.Context, repo repository.Repository, userID string, modifyFn func(*repository.Wallet) error) (*repository.Wallet, error) {
	const maxRetries = 3
	for attempt := 0; attempt <= maxRetries; attempt++ {
		wallet, err := repo.GetWallet(ctx, userID)
		if err != nil {
			return nil, err
		}

		if err := modifyFn(wallet); err != nil {
			return nil, err
		}

		wallet.UpdatedAt = time.Now()
		if err := repo.UpdateWallet(ctx, wallet); err != nil {
			var condErr *types.ConditionalCheckFailedException
			if errors.As(err, &condErr) && attempt < maxRetries {
				// Jittered backoff before retry
				jitter := time.Duration(rand.Intn(50*(attempt+1))) * time.Millisecond
				time.Sleep(50*time.Millisecond + jitter)
				continue
			}
			return nil, err
		}

		return wallet, nil
	}
	return nil, fmt.Errorf("optimistic lock failed after %d retries for user %s", maxRetries, userID)
}
```

Add `"context"`, `"fmt"`, `"math/rand"` to imports.

Then replace `CreditWallet` to use the retry helper:

```go
func (h *WalletHandler) CreditWallet(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userID := vars["user_id"]

	var req CreditRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Amount <= 0 {
		http.Error(w, "Amount must be positive", http.StatusBadRequest)
		return
	}

	wallet, err := updateWalletWithRetry(r.Context(), h.repo, userID, func(w *repository.Wallet) error {
		w.Balance += req.Amount
		return nil
	})
	if err != nil {
		http.Error(w, "Failed to update wallet", http.StatusInternalServerError)
		return
	}

	response := WalletResponse{
		UserID:  wallet.UserID,
		Balance: wallet.Balance,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
```

- [ ] **Step 5: Update consumer reserve/release with retry**

In `apps/wallet-service/internal/handlers/consumer.go`, update `handleReserveFunds` to use retry around the reservation+debit as a unit:

```go
func (ec *EventConsumer) handleReserveFunds(ctx context.Context, event events.ReserveFundsEvent) error {
	log.Printf("Reserving funds for payment %s: user %s, amount %.2f", event.PaymentID, event.UserID, event.Amount)

	const maxRetries = 3
	for attempt := 0; attempt <= maxRetries; attempt++ {
		wallet, err := ec.repo.GetWallet(ctx, event.UserID)
		if err != nil {
			return err
		}

		if wallet.Balance < event.Amount {
			log.Printf("Insufficient funds for user %s: balance %.2f, requested %.2f", event.UserID, wallet.Balance, event.Amount)
			return ec.publishInsufficientFunds(ctx, event.PaymentID, event.UserID)
		}

		// Create reservation (idempotent — conditional put ignores duplicates)
		reservation := &repository.Reservation{
			ReservationID: uuid.NewSHA1(uuid.NameSpaceDNS, []byte(event.PaymentID)).String(),
			PaymentID:     event.PaymentID,
			UserID:        event.UserID,
			Amount:        event.Amount,
			Status:        "ACTIVE",
			CreatedAt:     time.Now(),
			ExpiresAt:     time.Now().Add(24 * time.Hour).Unix(),
		}

		if err := ec.repo.CreateReservation(ctx, reservation); err != nil {
			return err
		}

		// Deduct balance with optimistic lock
		wallet.Balance -= event.Amount
		wallet.UpdatedAt = time.Now()

		if err := ec.repo.UpdateWallet(ctx, wallet); err != nil {
			var condErr *types.ConditionalCheckFailedException
			if errors.As(err, &condErr) && attempt < maxRetries {
				jitter := time.Duration(rand.Intn(50*(attempt+1))) * time.Millisecond
				time.Sleep(50*time.Millisecond + jitter)
				continue
			}
			return err
		}

		log.Printf("Funds reserved: reservation %s", reservation.ReservationID)
		return ec.publishFundsReserved(ctx, event.PaymentID)
	}

	return fmt.Errorf("optimistic lock failed after retries for payment %s", event.PaymentID)
}
```

Add imports: `"errors"`, `"fmt"`, `"math/rand"`, `"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"`.

Note: `uuid.NewSHA1` generates a deterministic UUID from payment_id, making the reservation ID stable across retries.

Update `handleReleaseFunds` similarly with the retry wrapper around the balance return.

- [ ] **Step 6: Build and verify**

Run: `cd c:/projects/my-saas-platform/apps/wallet-service && go build ./...`
Expected: Clean build.

- [ ] **Step 7: Commit**

```bash
git add apps/wallet-service/
git commit -m "fix: add optimistic locking to wallet balance updates

Uses DynamoDB ConditionExpression with version attribute to prevent
lost updates. Retry wrapper handles ConditionalCheckFailedException
with jittered backoff. CreateReservation is idempotent via conditional
put, allowing safe retry of the reservation+debit unit."
```

---

## Task 4: Outbox Worker Error Handling

**Files:**
- Modify: `apps/payment-service/pkg/outbox/worker.go`
- Modify: `apps/payment-service/pkg/outbox/dynamodb.go` (add MarkAsFailed and IncrementRetryCount)
- Modify: `apps/payment-service/pkg/outbox/interface.go` (add retry_count field, MarkAsFailed to interface)
- Modify: `apps/wallet-service/pkg/outbox/worker.go`
- Modify: `apps/wallet-service/pkg/outbox/dynamodb.go` (same as payment-service)
- Modify: `apps/wallet-service/pkg/outbox/interface.go` (same as payment-service)
- Modify: `apps/gateway-service/src/workers/outboxWorker.js`
- Modify: `apps/gateway-service/src/repository/outboxRepository.js`

- [ ] **Step 0: Add retry_count field and MarkAsFailed to outbox interface/implementation (Go)**

In `apps/payment-service/pkg/outbox/interface.go`, update `OutboxMessage` and `Outbox` interface:

```go
type OutboxMessage struct {
	MessageID  string `dynamodbav:"message_id"`
	EventType  string `dynamodbav:"event_type"`
	Payload    string `dynamodbav:"payload"`
	Published  int    `dynamodbav:"published"`
	RetryCount int    `dynamodbav:"retry_count"`
	CreatedAt  string `dynamodbav:"created_at"`
}

type Outbox interface {
	WriteMessage(ctx context.Context, eventType string, payload string) error
	GetUnpublishedMessages(ctx context.Context) ([]OutboxMessage, error)
	MarkAsPublished(ctx context.Context, messageID string) error
	MarkAsFailed(ctx context.Context, messageID string) error
	IncrementRetryCount(ctx context.Context, messageID string) error
}
```

In `apps/payment-service/pkg/outbox/dynamodb.go`, add:

```go
func (o *DynamoDBOutbox) MarkAsFailed(ctx context.Context, messageID string) error {
	_, err := o.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(o.tableName),
		Key: map[string]types.AttributeValue{
			"message_id": &types.AttributeValueMemberS{Value: messageID},
		},
		UpdateExpression: aws.String("SET published = :failed"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":failed": &types.AttributeValueMemberN{Value: "-1"}, // -1 = failed (poison pill)
		},
	})
	return err
}

func (o *DynamoDBOutbox) IncrementRetryCount(ctx context.Context, messageID string) error {
	_, err := o.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(o.tableName),
		Key: map[string]types.AttributeValue{
			"message_id": &types.AttributeValueMemberS{Value: messageID},
		},
		UpdateExpression: aws.String("SET retry_count = if_not_exists(retry_count, :zero) + :one"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":zero": &types.AttributeValueMemberN{Value: "0"},
			":one":  &types.AttributeValueMemberN{Value: "1"},
		},
	})
	return err
}
```

Apply the same changes to `apps/wallet-service/pkg/outbox/interface.go` and `apps/wallet-service/pkg/outbox/dynamodb.go`.

In `apps/gateway-service/src/repository/outboxRepository.js`, add:

```javascript
async function markAsFailed(messageId) {
  const command = new UpdateCommand({
    TableName: OUTBOX_TABLE,
    Key: { message_id: messageId },
    UpdateExpression: 'SET published = :failed',
    ExpressionAttributeValues: { ':failed': -1 },
  });
  await docClient.send(command);
}

async function incrementRetryCount(messageId) {
  const command = new UpdateCommand({
    TableName: OUTBOX_TABLE,
    Key: { message_id: messageId },
    UpdateExpression: 'SET retry_count = if_not_exists(retry_count, :zero) + :one',
    ExpressionAttributeValues: { ':zero': 0, ':one': 1 },
  });
  await docClient.send(command);
}

module.exports = {
  publishEvent,
  getUnpublishedMessages,
  markAsPublished,
  markAsFailed,
  incrementRetryCount,
};
```

- [ ] **Step 1: Fix Go outbox worker (payment-service)**

Replace `processOutbox` in `apps/payment-service/pkg/outbox/worker.go`:

```go
const maxOutboxRetries = 5

func (w *Worker) processOutbox(ctx context.Context) error {
	messages, err := w.outbox.GetUnpublishedMessages(ctx)
	if err != nil {
		return err
	}

	for _, msg := range messages {
		wrapper := map[string]interface{}{
			"event_type": msg.EventType,
			"payload":    json.RawMessage(msg.Payload),
		}

		body, err := json.Marshal(wrapper)
		if err != nil {
			log.Printf("Failed to marshal message %s: %v", msg.MessageID, err)
			continue
		}

		// Track per-queue success for fan-out partial failure handling
		allSent := true
		for _, q := range w.queues {
			if err := w.sendWithRetry(ctx, q, string(body), msg.MessageID); err != nil {
				log.Printf("ERROR: Failed to publish message %s after retries: %v", msg.MessageID, err)
				allSent = false
			}
		}

		if allSent {
			if err := w.outbox.MarkAsPublished(ctx, msg.MessageID); err != nil {
				log.Printf("Failed to mark message %s as published: %v", msg.MessageID, err)
			} else {
				log.Printf("Published event: %s (message_id: %s)", msg.EventType, msg.MessageID)
			}
		} else {
			// Increment retry count; mark as failed (poison pill) after max retries
			w.outbox.IncrementRetryCount(ctx, msg.MessageID)
			if msg.RetryCount+1 >= maxOutboxRetries {
				log.Printf("ERROR: Message %s exceeded max retries (%d), marking as failed", msg.MessageID, maxOutboxRetries)
				w.outbox.MarkAsFailed(ctx, msg.MessageID)
			}
		}
	}

	return nil
}

func (w *Worker) sendWithRetry(ctx context.Context, q queue.QueueClient, body, messageID string) error {
	const maxRetries = 3
	backoff := 100 * time.Millisecond

	for attempt := 0; attempt < maxRetries; attempt++ {
		if err := q.SendMessage(ctx, body); err != nil {
			if attempt < maxRetries-1 {
				log.Printf("Retry %d/%d for message %s: %v", attempt+1, maxRetries, messageID, err)
				time.Sleep(backoff)
				backoff *= 2
				continue
			}
			return err
		}
		return nil
	}
	return fmt.Errorf("unreachable")
}
```

Add `"fmt"` to imports.

- [ ] **Step 2: Fix Go outbox worker (wallet-service)**

Apply the same `processOutbox` and `sendWithRetry` changes to `apps/wallet-service/pkg/outbox/worker.go`. The code is identical.

- [ ] **Step 3: Fix Node.js outbox worker (gateway-service)**

Update the require at the top of `apps/gateway-service/src/workers/outboxWorker.js` to import the new functions:
```javascript
const { getUnpublishedMessages, markAsPublished, markAsFailed, incrementRetryCount } = require('../repository/outboxRepository');
const MAX_OUTBOX_RETRIES = 5;
```

Replace `processOutbox` in the same file:

```javascript
async function processOutbox() {
  const messages = await getUnpublishedMessages();

  for (const msg of messages) {
    try {
      const wrapper = {
        event_type: msg.event_type,
        payload: JSON.parse(msg.payload),
      };
      const body = JSON.stringify(wrapper);

      // Track per-queue success
      let allSent = true;
      for (const queueUrl of BROADCAST_QUEUE_URLS) {
        const sent = await sendWithRetry(queueUrl, body, msg.message_id);
        if (!sent) allSent = false;
      }

      if (allSent) {
        await markAsPublished(msg.message_id);
        console.log(`Published event: ${msg.event_type} (${msg.message_id})`);
      } else {
        // Increment retry count; mark as failed (poison pill) after max retries
        await incrementRetryCount(msg.message_id);
        const retryCount = (msg.retry_count || 0) + 1;
        if (retryCount >= MAX_OUTBOX_RETRIES) {
          console.error(`Message ${msg.message_id} exceeded max retries (${MAX_OUTBOX_RETRIES}), marking as failed`);
          await markAsFailed(msg.message_id);
        }
      }
    } catch (error) {
      console.error(`Failed to process outbox message ${msg.message_id}:`, error);
    }
  }
}

async function sendWithRetry(queueUrl, body, messageId, maxRetries = 3) {
  let backoff = 100;
  for (let attempt = 0; attempt < maxRetries; attempt++) {
    try {
      const command = new SendMessageCommand({
        QueueUrl: queueUrl,
        MessageBody: body,
      });
      await sqsClient.send(command);
      return true;
    } catch (error) {
      if (attempt < maxRetries - 1) {
        console.warn(`Retry ${attempt + 1}/${maxRetries} for message ${messageId}: ${error.message}`);
        await sleep(backoff);
        backoff *= 2;
      } else {
        console.error(`ERROR: Failed to send message ${messageId} to ${queueUrl} after ${maxRetries} retries`);
        return false;
      }
    }
  }
  return false;
}
```

- [ ] **Step 4: Build Go services**

Run: `cd c:/projects/my-saas-platform && go build ./...`
Expected: Clean build.

- [ ] **Step 5: Commit**

```bash
git add apps/payment-service/pkg/outbox/worker.go apps/wallet-service/pkg/outbox/worker.go apps/gateway-service/src/workers/outboxWorker.js
git commit -m "fix: outbox worker only marks published when all queues succeed

Adds per-queue retry with exponential backoff (3 attempts).
Messages stay unpublished if any queue send fails, preventing
silent event loss during SQS hiccups."
```

---

## Task 5: Outbox Adaptive Polling

**Files:**
- Modify: `apps/payment-service/pkg/outbox/worker.go`
- Modify: `apps/wallet-service/pkg/outbox/worker.go`
- Modify: `apps/gateway-service/src/workers/outboxWorker.js`

- [ ] **Step 1: Update Go outbox worker with adaptive polling**

Replace the `Start` method in `apps/payment-service/pkg/outbox/worker.go`:

```go
func (w *Worker) Start(ctx context.Context) {
	log.Println("Starting outbox worker...")

	baseInterval := 500 * time.Millisecond
	maxInterval := 5 * time.Second
	currentInterval := baseInterval

	timer := time.NewTimer(currentInterval)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("Outbox worker stopped")
			return
		case <-timer.C:
			count, err := w.processOutbox(ctx)
			if err != nil {
				log.Printf("Error processing outbox: %v", err)
				currentInterval = baseInterval
			} else if count >= 10 {
				// Full batch — drain immediately
				currentInterval = 0
			} else if count > 0 {
				// Partial batch — use base interval
				currentInterval = baseInterval
			} else {
				// Empty — exponential backoff
				currentInterval *= 2
				if currentInterval > maxInterval {
					currentInterval = maxInterval
				}
			}

			if currentInterval == 0 {
				timer.Reset(time.Millisecond) // near-immediate
			} else {
				timer.Reset(currentInterval)
			}
		}
	}
}
```

Update `processOutbox` signature from Task 4 to return the message count. The full method now looks like this (incorporating Task 4's error handling + the new return value):

```go
func (w *Worker) processOutbox(ctx context.Context) (int, error) {
	messages, err := w.outbox.GetUnpublishedMessages(ctx)
	if err != nil {
		return 0, err
	}

	for _, msg := range messages {
		wrapper := map[string]interface{}{
			"event_type": msg.EventType,
			"payload":    json.RawMessage(msg.Payload),
		}

		body, err := json.Marshal(wrapper)
		if err != nil {
			log.Printf("Failed to marshal message %s: %v", msg.MessageID, err)
			continue
		}

		allSent := true
		for _, q := range w.queues {
			if err := w.sendWithRetry(ctx, q, string(body), msg.MessageID); err != nil {
				log.Printf("ERROR: Failed to publish message %s after retries: %v", msg.MessageID, err)
				allSent = false
			}
		}

		if allSent {
			if err := w.outbox.MarkAsPublished(ctx, msg.MessageID); err != nil {
				log.Printf("Failed to mark message %s as published: %v", msg.MessageID, err)
			} else {
				log.Printf("Published event: %s (message_id: %s)", msg.EventType, msg.MessageID)
			}
		} else {
			w.outbox.IncrementRetryCount(ctx, msg.MessageID)
			if msg.RetryCount+1 >= maxOutboxRetries {
				log.Printf("ERROR: Message %s exceeded max retries (%d), marking as failed", msg.MessageID, maxOutboxRetries)
				w.outbox.MarkAsFailed(ctx, msg.MessageID)
			}
		}
	}

	return len(messages), nil
}
```

Update the `Worker` struct — remove the `interval` field since it's now adaptive. Update `NewWorker`:

```go
func NewWorker(outbox Outbox, queues []queue.QueueClient) *Worker {
	return &Worker{
		outbox: outbox,
		queues: queues,
	}
}
```

- [ ] **Step 2: Update wallet-service outbox worker**

Apply the same adaptive polling changes to `apps/wallet-service/pkg/outbox/worker.go`.

- [ ] **Step 3: Update gateway-service outbox worker**

In `apps/gateway-service/src/workers/outboxWorker.js`, replace the polling loop:

```javascript
const BASE_INTERVAL = 500;
const MAX_INTERVAL = 5000;

async function startOutboxWorker() {
  console.log('Starting outbox worker...');
  let currentInterval = BASE_INTERVAL;

  while (true) {
    try {
      const count = await processOutbox();

      if (count >= 10) {
        currentInterval = 1; // drain immediately
      } else if (count > 0) {
        currentInterval = BASE_INTERVAL;
      } else {
        currentInterval = Math.min(currentInterval * 2, MAX_INTERVAL);
      }

      await sleep(currentInterval);
    } catch (error) {
      console.error('Error processing outbox:', error);
      currentInterval = BASE_INTERVAL;
      await sleep(5000);
    }
  }
}
```

Update `processOutbox` to return the count: add `return messages.length;` at the end.

- [ ] **Step 4: Update callers that pass interval**

In `apps/payment-service/main.go:88` and `apps/wallet-service/main.go`, change:
```go
// Before:
outboxWorker := outbox.NewWorker(outboxRepo, broadcastQueues, 100*time.Millisecond)
// After:
outboxWorker := outbox.NewWorker(outboxRepo, broadcastQueues)
```

- [ ] **Step 5: Build all Go services**

Run: `cd c:/projects/my-saas-platform && go build ./...`
Expected: Clean build.

- [ ] **Step 6: Commit**

```bash
git add apps/payment-service/pkg/outbox/ apps/payment-service/main.go apps/wallet-service/pkg/outbox/ apps/wallet-service/main.go apps/gateway-service/src/workers/outboxWorker.js
git commit -m "feat: adaptive outbox polling with exponential backoff

Replace fixed 100ms interval with adaptive strategy: drain
immediately on full batch, use 500ms base interval on partial,
exponential backoff up to 5s when empty. Switch from Ticker
to Timer in Go for variable intervals."
```

---

## Task 6: Gateway-Service Idempotency

**Files:**
- Modify: `modules/messaging/main.tf`
- Create: `apps/gateway-service/src/repository/idempotencyRepository.js`
- Modify: `apps/gateway-service/src/consumers/eventConsumer.js`
- Modify: `apps/gateway-service/src/index.js`
- Modify: `apps/gateway-service/k8s/deployment.yaml`

- [ ] **Step 1: Add gateway_idempotency table to Terraform**

In `modules/messaging/main.tf`, add after the `gateway_outbox` resource (around line 275):

```hcl
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
```

Also add the table ARN to the IAM policy resource list (line ~381):
```hcl
aws_dynamodb_table.gateway_idempotency.arn,
```

- [ ] **Step 2: Create idempotency repository**

Create `apps/gateway-service/src/repository/idempotencyRepository.js`:

```javascript
const { DynamoDBClient } = require('@aws-sdk/client-dynamodb');
const { DynamoDBDocumentClient, PutCommand } = require('@aws-sdk/lib-dynamodb');

const client = new DynamoDBClient({ region: process.env.AWS_REGION });
const docClient = DynamoDBDocumentClient.from(client);

const IDEMPOTENCY_TABLE = process.env.IDEMPOTENCY_TABLE;

/**
 * Attempts to claim a payment_id for processing.
 * Returns true if this is the first time (proceed), false if duplicate (skip).
 */
async function claimPayment(paymentId) {
  try {
    const command = new PutCommand({
      TableName: IDEMPOTENCY_TABLE,
      Item: {
        payment_id: paymentId,
        created_at: new Date().toISOString(),
        expires_at: Math.floor(Date.now() / 1000) + 86400, // 24h TTL
      },
      ConditionExpression: 'attribute_not_exists(payment_id)',
    });

    await docClient.send(command);
    return true; // First claim — proceed with processing
  } catch (error) {
    if (error.name === 'ConditionalCheckFailedException') {
      return false; // Duplicate — skip
    }
    throw error;
  }
}

module.exports = { claimPayment };
```

- [ ] **Step 3: Add idempotency check to event consumer**

In `apps/gateway-service/src/consumers/eventConsumer.js`, add the dedup check:

```javascript
const { SQSClient, ReceiveMessageCommand, DeleteMessageCommand } = require('@aws-sdk/client-sqs');
const { processPayment } = require('../services/paymentProcessor');
const { claimPayment } = require('../repository/idempotencyRepository');

// ... (sqsClient and QUEUE_URL stay the same)

async function handleMessage(message) {
  const body = JSON.parse(message.Body);
  const eventType = body.event_type;
  const payload = body.payload;

  console.log(`Received event: ${eventType}`);

  if (eventType === 'ProcessPayment') {
    // Idempotency check: skip if already processed
    const claimed = await claimPayment(payload.payment_id);
    if (!claimed) {
      console.log(`Duplicate ProcessPayment for ${payload.payment_id}, skipping`);
      return;
    }
    await processPayment(payload);
  } else {
    console.log(`Ignoring event type: ${eventType}`);
  }
}
```

- [ ] **Step 4: Add IDEMPOTENCY_TABLE env var to K8s manifest**

In `apps/gateway-service/k8s/deployment.yaml`, add to the env section:

```yaml
            - name: IDEMPOTENCY_TABLE
              value: "dev-gateway-idempotency"
```

- [ ] **Step 5: Add env var validation in index.js**

In `apps/gateway-service/src/index.js`, add `"IDEMPOTENCY_TABLE"` to the required env vars array.

- [ ] **Step 6: Commit**

```bash
git add modules/messaging/main.tf apps/gateway-service/
git commit -m "feat: add gateway-service idempotency via dedicated DynamoDB table

New gateway_idempotency table keyed by payment_id with 24h TTL.
Event consumer claims payment_id before processing, preventing
duplicate payment processing across replicas."
```

---

## Task 7: Graceful Shutdown

**Files:**
- Modify: `apps/payment-service/main.go`
- Modify: `apps/wallet-service/main.go`
- Modify: `apps/ledger-service/main.go`
- Modify: `apps/gateway-service/src/index.js`
- Modify: `apps/gateway-service/src/consumers/eventConsumer.js`
- Modify: `apps/gateway-service/src/workers/outboxWorker.js`
- Modify: `apps/notification-service/src/index.js`
- Modify: `apps/notification-service/src/consumers/eventConsumer.js`

- [ ] **Step 1: Fix payment-service graceful shutdown**

In `apps/payment-service/main.go`, replace `ctx := context.Background()` and the shutdown section:

```go
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
```

Add a WaitGroup for background goroutines:

```go
	var wg sync.WaitGroup

	// Start outbox worker
	outboxWorker := outbox.NewWorker(outboxRepo, broadcastQueues)
	wg.Add(1)
	go func() {
		defer wg.Done()
		outboxWorker.Start(ctx)
	}()

	// Start event consumer
	eventConsumer := handlers.NewEventConsumer(queueClient, sagaOrchestrator)
	wg.Add(1)
	go func() {
		defer wg.Done()
		eventConsumer.Start(ctx)
	}()
```

Replace the shutdown section:

```go
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")
	cancel() // Signal all goroutines to stop

	// Wait for background goroutines with timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Println("Background workers stopped cleanly")
	case <-time.After(10 * time.Second):
		log.Println("Timeout waiting for background workers")
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited")
```

Add `"sync"` to imports.

- [ ] **Step 2: Fix wallet-service graceful shutdown**

Apply the same pattern to `apps/wallet-service/main.go`. Same changes: `context.WithCancel`, `sync.WaitGroup`, cancel before wait.

- [ ] **Step 3: Fix ledger-service graceful shutdown**

Apply to `apps/ledger-service/main.go`. Same pattern but only one goroutine (event consumer, no outbox worker).

- [ ] **Step 4: Create shared shutdown module for gateway-service**

Create `apps/gateway-service/src/utils/shutdown.js` to avoid circular dependency (index.js requires eventConsumer.js which would require index.js):

```javascript
let shuttingDown = false;

function shutdown() {
  shuttingDown = true;
}

function isShuttingDown() {
  return shuttingDown;
}

module.exports = { shutdown, isShuttingDown };
```

- [ ] **Step 5: Update gateway-service index.js to use shutdown module**

In `apps/gateway-service/src/index.js`, replace the SIGTERM/SIGINT handlers:

```javascript
const { shutdown } = require("./utils/shutdown");

// ... (existing main function and server setup stay the same)

process.on("SIGTERM", () => {
  console.log("SIGTERM received, shutting down gracefully...");
  shutdown();
  setTimeout(() => process.exit(0), 10000);
});

process.on("SIGINT", () => {
  console.log("SIGINT received, shutting down gracefully...");
  shutdown();
  setTimeout(() => process.exit(0), 10000);
});
```

- [ ] **Step 6: Update gateway event consumer to check shutdown flag**

In `apps/gateway-service/src/consumers/eventConsumer.js`, add the import and update the loop:

```javascript
const { isShuttingDown } = require('../utils/shutdown');

async function startEventConsumer() {
  console.log('Starting event consumer...');

  while (!isShuttingDown()) {
    // ... existing try/catch body stays the same
  }
  console.log('Event consumer stopped');
}
```

- [ ] **Step 7: Update gateway outbox worker to check shutdown flag**

In `apps/gateway-service/src/workers/outboxWorker.js`: add `const { isShuttingDown } = require('../utils/shutdown');` and replace `while (true)` with `while (!isShuttingDown())`.

- [ ] **Step 8: Apply same pattern to notification-service**

Create `apps/notification-service/src/utils/shutdown.js` (same as gateway's).
In `apps/notification-service/src/index.js`: import `shutdown` from utils, use it in SIGTERM/SIGINT handlers.
In `apps/notification-service/src/consumers/eventConsumer.js`: import `isShuttingDown` from utils, replace `while (true)` with `while (!isShuttingDown())`.

- [ ] **Step 8: Build Go services**

Run: `cd c:/projects/my-saas-platform && go build ./...`
Expected: Clean build.

- [ ] **Step 9: Commit**

```bash
git add apps/payment-service/main.go apps/wallet-service/main.go apps/ledger-service/main.go apps/gateway-service/src/ apps/notification-service/src/
git commit -m "feat: graceful shutdown for all services

Go: context.WithCancel + sync.WaitGroup with 10s drain timeout.
Node.js: shuttingDown flag checked in consumer/worker loops.
Prevents in-flight message loss during K8s pod evictions."
```

---

## Task 8: Saga Timeout Scanner

**Files:**
- Modify: `apps/payment-service/internal/model/types.go`
- Modify: `apps/payment-service/internal/saga/orchestrator.go`
- Modify: `apps/payment-service/internal/repository/dynamodb.go`
- Create: `apps/payment-service/internal/saga/timeout_scanner.go`
- Modify: `apps/payment-service/main.go`
- Modify: `modules/messaging/main.tf`

- [ ] **Step 1: Add TIMED_OUT state constant and ReleaseFunds event type**

In `apps/payment-service/internal/model/types.go`, add `StateTimedOut` to the constants:

```go
const (
	StatePending       PaymentState = "PENDING"
	StateFundsReserved PaymentState = "FUNDS_RESERVED"
	StateProcessing    PaymentState = "PROCESSING"
	StateCompleted     PaymentState = "COMPLETED"
	StateFailed        PaymentState = "FAILED"
	StateCompensating  PaymentState = "COMPENSATING"
	StateCancelled     PaymentState = "CANCELLED"
	StateTimedOut      PaymentState = "TIMED_OUT"
)
```

In `apps/payment-service/internal/events/types.go`, add the `ReleaseFunds` event type and struct (these only exist in wallet-service currently but the timeout scanner in payment-service needs to publish them):

```go
// Add to the event type constants:
EventReleaseFunds = "ReleaseFunds"

// Add the struct:
type ReleaseFundsEvent struct {
	PaymentID string `json:"payment_id"`
	Timestamp string `json:"timestamp"`
}
```

- [ ] **Step 2: Add GSI to payments table in Terraform**

In `modules/messaging/main.tf`, update the `aws_dynamodb_table.payments` resource to add the `state` and `updated_at` attributes and GSI:

```hcl
resource "aws_dynamodb_table" "payments" {
  name         = "${var.env}-payments"
  billing_mode = "PAY_PER_REQUEST"
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
```

- [ ] **Step 3: Add repository method to find stuck payments**

In `apps/payment-service/internal/repository/dynamodb.go`, add:

```go
func (r *DynamoDBRepository) GetStuckPayments(ctx context.Context, state model.PaymentState, olderThan time.Time) ([]*model.Payment, error) {
	result, err := r.client.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(r.paymentsTable),
		IndexName:              aws.String("state-updated-index"),
		KeyConditionExpression: aws.String("#state = :state AND updated_at < :threshold"),
		ExpressionAttributeNames: map[string]string{
			"#state": "state",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":state":     &types.AttributeValueMemberS{Value: string(state)},
			":threshold": &types.AttributeValueMemberS{Value: olderThan.Format(time.RFC3339)},
		},
		Limit: aws.Int32(25),
	})

	if err != nil {
		return nil, err
	}

	var payments []*model.Payment
	if err := attributevalue.UnmarshalListOfMaps(result.Items, &payments); err != nil {
		return nil, err
	}

	return payments, nil
}

func (r *DynamoDBRepository) ConditionalUpdateState(ctx context.Context, paymentID string, expectedState, newState model.PaymentState) error {
	_, err := r.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(r.paymentsTable),
		Key: map[string]types.AttributeValue{
			"payment_id": &types.AttributeValueMemberS{Value: paymentID},
		},
		UpdateExpression:    aws.String("SET #state = :new_state, updated_at = :now"),
		ConditionExpression: aws.String("#state = :expected_state"),
		ExpressionAttributeNames: map[string]string{
			"#state": "state",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":new_state":      &types.AttributeValueMemberS{Value: string(newState)},
			":expected_state": &types.AttributeValueMemberS{Value: string(expectedState)},
			":now":            &types.AttributeValueMemberS{Value: time.Now().Format(time.RFC3339)},
		},
	})

	return err
}
```

- [ ] **Step 4: Create timeout scanner**

Create `apps/payment-service/internal/saga/timeout_scanner.go`:

```go
package saga

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"time"

	ddbTypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/my-saas-platform/payment-service/internal/events"
	"github.com/my-saas-platform/payment-service/internal/model"
	"github.com/my-saas-platform/payment-service/internal/repository"
	"github.com/my-saas-platform/payment-service/pkg/outbox"
)

type TimeoutScanner struct {
	repo           *repository.DynamoDBRepository
	outbox         outbox.Outbox
	scanInterval   time.Duration
	timeoutAge     time.Duration
}

func NewTimeoutScanner(repo *repository.DynamoDBRepository, outbox outbox.Outbox) *TimeoutScanner {
	return &TimeoutScanner{
		repo:         repo,
		outbox:       outbox,
		scanInterval: 30 * time.Second,
		timeoutAge:   60 * time.Second,
	}
}

func (s *TimeoutScanner) Start(ctx context.Context) {
	log.Println("Starting saga timeout scanner...")
	timer := time.NewTimer(s.scanInterval)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("Saga timeout scanner stopped")
			return
		case <-timer.C:
			s.scanForStuckPayments(ctx)
			timer.Reset(s.scanInterval)
		}
	}
}

func (s *TimeoutScanner) scanForStuckPayments(ctx context.Context) {
	threshold := time.Now().Add(-s.timeoutAge)
	stuckStates := []model.PaymentState{
		model.StatePending,
		model.StateProcessing,
		model.StateFundsReserved,
	}

	for _, state := range stuckStates {
		payments, err := s.repo.GetStuckPayments(ctx, state, threshold)
		if err != nil {
			log.Printf("Error scanning for stuck %s payments: %v", state, err)
			continue
		}

		for _, payment := range payments {
			s.compensatePayment(ctx, payment)
		}
	}
}

func (s *TimeoutScanner) compensatePayment(ctx context.Context, payment *model.Payment) {
	log.Printf("Timing out stuck payment %s (state: %s, updated: %s)",
		payment.PaymentID, payment.State, payment.UpdatedAt.Format(time.RFC3339))

	// Conditional update: only transition if state hasn't changed since we read it
	err := s.repo.ConditionalUpdateState(ctx, payment.PaymentID, payment.State, model.StateTimedOut)
	if err != nil {
		var condErr *ddbTypes.ConditionalCheckFailedException
		if errors.As(err, &condErr) {
			log.Printf("Payment %s state changed since scan, skipping", payment.PaymentID)
			return
		}
		log.Printf("Error updating payment %s to TIMED_OUT: %v", payment.PaymentID, err)
		return
	}

	// Publish ReleaseFunds compensation event if funds were reserved
	if payment.State == model.StateFundsReserved || payment.State == model.StateProcessing {
		event := events.ReleaseFundsEvent{
			PaymentID: payment.PaymentID,
			Timestamp: time.Now().Format(time.RFC3339),
		}
		payload, _ := json.Marshal(event)
		if err := s.outbox.WriteMessage(ctx, events.EventReleaseFunds, string(payload)); err != nil {
			log.Printf("ERROR: Failed to publish ReleaseFunds for timed-out payment %s: %v", payment.PaymentID, err)
		}
	}
}
```

- [ ] **Step 5: Add late arrival guard to orchestrator**

In `apps/payment-service/internal/saga/orchestrator.go`, update `HandlePaymentSucceeded` and `HandlePaymentFailed` to check for TIMED_OUT:

```go
func (o *Orchestrator) HandlePaymentSucceeded(ctx context.Context, paymentID string) error {
	payment, err := o.repo.GetPayment(ctx, paymentID)
	if err != nil {
		return err
	}

	if payment.State == model.StateTimedOut {
		log.Printf("Ignoring late PaymentSucceeded for timed-out payment %s", paymentID)
		return nil
	}

	// ... rest of existing code
```

Same guard in `HandlePaymentFailed`.

- [ ] **Step 6: Wire timeout scanner in main.go**

In `apps/payment-service/main.go`, after the event consumer setup:

```go
	// Start saga timeout scanner
	timeoutScanner := saga.NewTimeoutScanner(repo, outboxRepo)
	wg.Add(1)
	go func() {
		defer wg.Done()
		timeoutScanner.Start(ctx)
	}()
```

Note: `repo` here needs to be the concrete `*repository.DynamoDBRepository` type (not the interface) so the scanner can call `GetStuckPayments` and `ConditionalUpdateState`. Adjust the variable type if needed.

- [ ] **Step 7: Build and verify**

Run: `cd c:/projects/my-saas-platform && go build ./...`
Expected: Clean build.

- [ ] **Step 8: Commit**

```bash
git add apps/payment-service/ modules/messaging/main.tf
git commit -m "feat: saga timeout scanner for stuck payment detection

Scans for payments in PENDING/PROCESSING/FUNDS_RESERVED older
than 60s every 30s. Transitions to TIMED_OUT with conditional
write, publishes ReleaseFunds compensation. Late arrivals from
gateway are ignored via state guard in orchestrator."
```

---

## Task 9: SSE Connection Management

**Files:**
- Modify: `apps/notification-service/src/sse/sseManager.js`
- Modify: `apps/notification-service/src/index.js`

- [ ] **Step 1: Rewrite sseManager with connection limits, heartbeat, and cleanup**

Replace `apps/notification-service/src/sse/sseManager.js`:

```javascript
const MAX_CONNECTIONS = parseInt(process.env.SSE_MAX_CONNECTIONS || '1000', 10);
const HEARTBEAT_INTERVAL = 30000; // 30s
const IDLE_TIMEOUT = parseInt(process.env.SSE_IDLE_TIMEOUT_MS || '900000', 10); // 15 min

const clients = new Map(); // res -> { lastWrite: timestamp }
let heartbeatTimer = null;

function addClient(res) {
  if (clients.size >= MAX_CONNECTIONS) {
    console.warn(`SSE connection limit reached (${MAX_CONNECTIONS}), rejecting`);
    res.status(503).end();
    return false;
  }

  clients.set(res, { lastWrite: Date.now() });
  console.log(`SSE client connected. Total clients: ${clients.size}`);

  res.on('close', () => {
    clients.delete(res);
    console.log(`SSE client disconnected. Total clients: ${clients.size}`);
  });

  // Start heartbeat if this is the first client
  if (clients.size === 1 && !heartbeatTimer) {
    startHeartbeat();
  }

  return true;
}

function broadcastEvent(notification) {
  const data = JSON.stringify(notification);
  const total = clients.size;
  let sent = 0;
  const dead = [];

  for (const [client, meta] of clients) {
    try {
      client.write(`data: ${data}\n\n`);
      meta.lastWrite = Date.now();
      sent++;
    } catch (err) {
      dead.push(client);
    }
  }

  for (const client of dead) {
    clients.delete(client);
  }

  console.log(`Broadcast event to ${sent}/${total} SSE clients`);
}

function startHeartbeat() {
  heartbeatTimer = setInterval(() => {
    const now = Date.now();
    const dead = [];

    for (const [client, meta] of clients) {
      try {
        client.write(': ping\n\n');
        meta.lastWrite = now;
      } catch (err) {
        dead.push(client);
      }

      // Idle timeout: no successful write in IDLE_TIMEOUT ms
      if (now - meta.lastWrite > IDLE_TIMEOUT) {
        dead.push(client);
      }
    }

    for (const client of dead) {
      try { client.end(); } catch (e) { /* ignore */ }
      clients.delete(client);
    }

    if (clients.size === 0) {
      clearInterval(heartbeatTimer);
      heartbeatTimer = null;
    }
  }, HEARTBEAT_INTERVAL);
}

function getClientCount() {
  return clients.size;
}

module.exports = { addClient, broadcastEvent, getClientCount };
```

- [ ] **Step 2: Update SSE endpoint and addClient for 503 handling**

In `apps/notification-service/src/sse/sseManager.js`, update `addClient` to use raw `writeHead` (not Express `res.status()` which requires Express middleware):

```javascript
function addClient(res) {
  if (clients.size >= MAX_CONNECTIONS) {
    console.warn(`SSE connection limit reached (${MAX_CONNECTIONS}), rejecting`);
    res.writeHead(503);
    res.end('Connection limit reached');
    return false;
  }
  // ... rest of function stays the same
```

In `apps/notification-service/src/index.js`, update the `/events` handler. Check connection limit BEFORE writing SSE headers:

```javascript
app.get('/events', (req, res) => {
  const accepted = addClient(res);
  if (!accepted) return; // 503 already sent by addClient

  res.writeHead(200, {
    'Content-Type': 'text/event-stream',
    'Cache-Control': 'no-cache',
    'Connection': 'keep-alive',
    'Access-Control-Allow-Origin': '*',
  });

  res.write(
    `data: ${JSON.stringify({ type: 'connected', message: 'Connected to payment event stream', timestamp: new Date().toISOString() })}\n\n`
  );
});
```

- [ ] **Step 3: Commit**

```bash
git add apps/notification-service/src/
git commit -m "feat: SSE connection limits, heartbeat, and idle timeout

Max 1000 connections (configurable). 30s heartbeat with zombie
cleanup on write failure. 15min idle timeout for dead connections.
Returns 503 when limit reached."
```

---

## Task 10: Structured Logging

**Files:**
- Modify: All Go service `main.go` and handler files
- Create: `apps/gateway-service/src/utils/logger.js`
- Create: `apps/notification-service/src/utils/logger.js`
- Modify: All Node.js consumer/worker files

- [ ] **Step 1: Set up slog default in Go services**

In each Go service's `main.go` (`payment-service`, `wallet-service`, `ledger-service`), add at the top of `main()`:

```go
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
```

Add `"log/slog"` to imports.

- [ ] **Step 2: Replace log.Printf with slog in Go handlers**

Replace `log.Printf(...)` calls with `slog.Info(...)` or `slog.Error(...)` throughout Go handler and worker files. Examples:

```go
// Before:
log.Printf("Reserving funds for payment %s: user %s, amount %.2f", event.PaymentID, event.UserID, event.Amount)
// After:
slog.Info("Reserving funds", "payment_id", event.PaymentID, "user_id", event.UserID, "amount", event.Amount)

// Before:
log.Printf("Error receiving messages: %v", err)
// After:
slog.Error("Error receiving messages", "error", err)
```

Apply across all files that currently use `log.Printf`. Keep `log.Fatal`/`log.Fatalf` in `main()` for startup errors (those are fine unstructured).

- [ ] **Step 3: Create Node.js structured logger**

Create `apps/gateway-service/src/utils/logger.js`:

```javascript
const SERVICE_NAME = process.env.SERVICE_NAME || 'unknown';

function log(level, message, fields = {}) {
  const entry = {
    timestamp: new Date().toISOString(),
    level,
    service: SERVICE_NAME,
    message,
    ...fields,
  };
  console.log(JSON.stringify(entry));
}

module.exports = {
  info: (message, fields) => log('info', message, fields),
  warn: (message, fields) => log('warn', message, fields),
  error: (message, fields) => log('error', message, fields),
};
```

Copy to `apps/notification-service/src/utils/logger.js`.

- [ ] **Step 4: Replace console.log in Node.js services**

Replace `console.log(...)` and `console.error(...)` in gateway-service and notification-service files with the structured logger:

```javascript
const logger = require('../utils/logger');

// Before:
console.log(`Processing payment ${payment_id} for user ${user_id}: $${amount}`);
// After:
logger.info('Processing payment', { payment_id, user_id, amount });

// Before:
console.error('Error handling message:', error);
// After:
logger.error('Error handling message', { error: error.message });
```

Add `SERVICE_NAME` env var to K8s deployment manifests for gateway and notification services.

- [ ] **Step 5: Build Go services**

Run: `cd c:/projects/my-saas-platform && go build ./...`
Expected: Clean build.

- [ ] **Step 6: Commit**

```bash
git add apps/
git commit -m "feat: structured JSON logging across all services

Go: log/slog with JSON handler. Node.js: lightweight JSON
logger wrapper. All log entries include timestamp, level,
service name, and contextual fields (payment_id, error)."
```

---

## Task 11: Health Check Enrichment

**Files:**
- Modify: All Go service `main.go` files
- Modify: All Node.js `index.js` files
- Modify: All K8s `deployment.yaml` files

- [ ] **Step 1: Add /readyz to payment-service**

In `apps/payment-service/main.go`, add a `/readyz` endpoint that checks DynamoDB connectivity:

```go
	r.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		// Quick DynamoDB connectivity check
		_, err := repo.GetPayment(r.Context(), "healthcheck-nonexistent")
		// We expect "payment not found" error, not a connection error
		if err != nil && !strings.Contains(err.Error(), "payment not found") {
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(map[string]string{"status": "unhealthy", "error": err.Error()})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ready"})
	}).Methods("GET")
```

- [ ] **Step 2: Add /readyz to wallet-service and ledger-service**

Same pattern for each: attempt a lightweight DynamoDB operation and return 503 if it fails.

- [ ] **Step 3: Add /readyz to Node.js services**

In `apps/gateway-service/src/index.js` and `apps/notification-service/src/index.js`:

```javascript
app.get('/readyz', async (req, res) => {
  try {
    // Lightweight DynamoDB connectivity check
    const { DynamoDBClient, DescribeTableCommand } = require('@aws-sdk/client-dynamodb');
    const client = new DynamoDBClient({ region: process.env.AWS_REGION });
    await client.send(new DescribeTableCommand({ TableName: process.env.TRANSACTIONS_TABLE || process.env.QUEUE_URL ? 'healthcheck' : 'healthcheck' }));
    res.json({ status: 'ready' });
  } catch (error) {
    // DescribeTable on nonexistent table returns ResourceNotFoundException — that's fine, connection works
    if (error.name === 'ResourceNotFoundException') {
      res.json({ status: 'ready' });
    } else {
      res.status(503).json({ status: 'unhealthy', error: error.message });
    }
  }
});
```

For a simpler approach, just check if required env vars are set and the process is running (readiness is mainly about "can I serve traffic"):

```javascript
app.get('/readyz', (req, res) => {
  if (isShuttingDown()) {
    return res.status(503).json({ status: 'draining' });
  }
  res.json({ status: 'ready' });
});
```

Use the simpler approach — it integrates with the shutdown flag from Task 7.

- [ ] **Step 4: Update all K8s deployment.yaml readiness probes**

In all 5 deployment.yaml files, change the readiness probe path:

```yaml
          readinessProbe:
            httpGet:
              path: /readyz
              port: 8080
            initialDelaySeconds: 5
            periodSeconds: 5
```

- [ ] **Step 5: Build Go services**

Run: `cd c:/projects/my-saas-platform && go build ./...`
Expected: Clean build.

- [ ] **Step 6: Commit**

```bash
git add apps/
git commit -m "feat: add /readyz readiness endpoint to all services

Separates liveness (/healthz) from readiness (/readyz). Readiness
checks dependency connectivity and shutdown state. K8s readiness
probes updated to use /readyz."
```

---

## Task 12: Payment-Service Metrics Endpoint

**Files:**
- Modify: `apps/payment-service/main.go`
- Modify: `apps/payment-service/pkg/outbox/worker.go` (add in-memory counters)

- [ ] **Step 1: Add publish counters to outbox worker**

In `apps/payment-service/pkg/outbox/worker.go`, add atomic counters to the `Worker` struct:

```go
import "sync/atomic"

type Worker struct {
	outbox          Outbox
	queues          []queue.QueueClient
	publishedCount  atomic.Int64
	failedCount     atomic.Int64
}

func (w *Worker) PublishedCount() int64 { return w.publishedCount.Load() }
func (w *Worker) FailedCount() int64   { return w.failedCount.Load() }
```

In `processOutbox`, increment the counters:
- After `MarkAsPublished` succeeds: `w.publishedCount.Add(1)`
- After `MarkAsFailed` is called: `w.failedCount.Add(1)`

- [ ] **Step 2: Add /metrics endpoint**

In `apps/payment-service/main.go`, add a `/metrics` handler (after outboxWorker is created so it can reference the counters):

```go
	r.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Count unpublished outbox messages
		unpublished, err := outboxRepo.GetUnpublishedMessages(ctx)
		outboxDepth := 0
		if err == nil {
			outboxDepth = len(unpublished)
		}

		// Count stuck sagas
		threshold := time.Now().Add(-60 * time.Second)
		stuckCount := 0
		for _, state := range []model.PaymentState{model.StatePending, model.StateProcessing, model.StateFundsReserved} {
			stuck, err := repo.GetStuckPayments(ctx, state, threshold)
			if err == nil {
				stuckCount += len(stuck)
			}
		}

		metrics := map[string]interface{}{
			"outbox_queue_depth":       outboxDepth,
			"stuck_sagas":              stuckCount,
			"messages_published_total": outboxWorker.PublishedCount(),
			"messages_failed_total":    outboxWorker.FailedCount(),
			"timestamp":               time.Now().Format(time.RFC3339),
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(metrics)
	}).Methods("GET")
```

- [ ] **Step 2: Build and verify**

Run: `cd c:/projects/my-saas-platform/apps/payment-service && go build ./...`
Expected: Clean build.

- [ ] **Step 3: Commit**

```bash
git add apps/payment-service/main.go
git commit -m "feat: add /metrics endpoint for outbox depth and stuck sagas

Returns JSON with outbox_queue_depth and stuck_sagas counts.
Lightweight endpoint the dashboard can poll for system health."
```

---

## Task 13: Production Infrastructure-as-Code

**Files:**
- Create: `envs/prod/provider.tf`
- Create: `envs/prod/variables.tf`
- Create: `envs/prod/main.tf`
- Create: `envs/prod/terraform.tfvars`
- Create: `k8s/prod/hpa.yaml`
- Create: `k8s/prod/pdb.yaml`

- [ ] **Step 1: Read existing dev provider.tf for reference**

Read `envs/dev/provider.tf` to mirror the structure.

- [ ] **Step 2: Create envs/prod/provider.tf**

```hcl
terraform {
  required_version = ">= 1.5.0"

  backend "s3" {
    bucket         = "my-saas-platform-tf-state"
    key            = "prod/terraform.tfstate"
    region         = "us-east-1"
    dynamodb_table = "terraform-state-lock"
    encrypt        = true
  }

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
    helm = {
      source  = "hashicorp/helm"
      version = "~> 2.12"
    }
    kubernetes = {
      source  = "hashicorp/kubernetes"
      version = "~> 2.25"
    }
  }
}

provider "aws" {
  region = "us-east-1"
}
```

- [ ] **Step 3: Create envs/prod/variables.tf**

```hcl
variable "env" {
  description = "Environment name"
  type        = string
  default     = "prod"
}
```

- [ ] **Step 4: Create envs/prod/main.tf**

Mirror `envs/dev/main.tf` but with production values:

```hcl
module "network" {
  source = "../../modules/network"

  env      = var.env
  vpc_cidr = "10.1.0.0/16"
  azs      = ["us-east-1a", "us-east-1b"]

  public_subnets  = ["10.1.1.0/24", "10.1.2.0/24"]
  private_subnets = ["10.1.10.0/24", "10.1.11.0/24"]

  # Production: NAT gateway per AZ for HA
  # Note: requires adding nat_per_az variable to network module
}

module "compute" {
  source = "../../modules/compute"

  env        = var.env
  vpc_id     = module.network.vpc_id
  subnet_ids = module.network.private_subnet_ids

  # Production: larger instances, higher max
  # Note: requires adding variables to compute module for instance type and scaling
}

module "messaging" {
  source = "../../modules/messaging"
  env    = var.env
}

# Helm/K8s providers (same pattern as dev)
data "aws_eks_cluster" "cluster" {
  name = module.compute.cluster_name
}

data "aws_eks_cluster_auth" "cluster" {
  name = module.compute.cluster_name
}

provider "helm" {
  kubernetes {
    host                   = module.compute.cluster_endpoint
    cluster_ca_certificate = base64decode(module.compute.cluster_certificate_authority_data)
    token                  = data.aws_eks_cluster_auth.cluster.token
  }
}

provider "kubernetes" {
  host                   = module.compute.cluster_endpoint
  cluster_ca_certificate = base64decode(module.compute.cluster_certificate_authority_data)
  token                  = data.aws_eks_cluster_auth.cluster.token
}

# NGINX Ingress (same as dev)
resource "helm_release" "nginx_ingress" {
  name       = "nginx-ingress"
  repository = "https://kubernetes.github.io/ingress-nginx"
  chart      = "ingress-nginx"
  version    = "4.10.0"

  namespace        = "ingress-nginx"
  create_namespace = true

  depends_on = [module.compute]
  timeout    = 1200
}

# Cluster Autoscaler (prod only)
resource "helm_release" "cluster_autoscaler" {
  name       = "cluster-autoscaler"
  repository = "https://kubernetes.github.io/autoscaler"
  chart      = "cluster-autoscaler"
  version    = "9.35.0"

  namespace        = "kube-system"
  create_namespace = false

  depends_on = [module.compute]

  set {
    name  = "autoDiscovery.clusterName"
    value = module.compute.cluster_name
  }

  set {
    name  = "awsRegion"
    value = "us-east-1"
  }
}
```

- [ ] **Step 5: Create k8s/prod/hpa.yaml**

```yaml
# HorizontalPodAutoscaler for all services
---
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: payment-service-hpa
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: payment-service
  minReplicas: 2
  maxReplicas: 10
  metrics:
    - type: Resource
      resource:
        name: cpu
        target:
          type: Utilization
          averageUtilization: 70
---
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: wallet-service-hpa
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: wallet-service
  minReplicas: 2
  maxReplicas: 10
  metrics:
    - type: Resource
      resource:
        name: cpu
        target:
          type: Utilization
          averageUtilization: 70
---
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: ledger-service-hpa
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: ledger-service
  minReplicas: 2
  maxReplicas: 10
  metrics:
    - type: Resource
      resource:
        name: cpu
        target:
          type: Utilization
          averageUtilization: 70
---
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: gateway-service-hpa
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: gateway-service
  minReplicas: 2
  maxReplicas: 10
  metrics:
    - type: Resource
      resource:
        name: cpu
        target:
          type: Utilization
          averageUtilization: 70
---
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: notification-service-hpa
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: notification-service
  minReplicas: 2
  maxReplicas: 10
  metrics:
    - type: Resource
      resource:
        name: cpu
        target:
          type: Utilization
          averageUtilization: 70
```

- [ ] **Step 6: Create k8s/prod/pdb.yaml**

```yaml
# PodDisruptionBudgets for all services
---
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: payment-service-pdb
spec:
  minAvailable: 1
  selector:
    matchLabels:
      app: payment-service
---
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: wallet-service-pdb
spec:
  minAvailable: 1
  selector:
    matchLabels:
      app: wallet-service
---
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: ledger-service-pdb
spec:
  minAvailable: 1
  selector:
    matchLabels:
      app: ledger-service
---
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: gateway-service-pdb
spec:
  minAvailable: 1
  selector:
    matchLabels:
      app: gateway-service
---
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: notification-service-pdb
spec:
  minAvailable: 1
  selector:
    matchLabels:
      app: notification-service
```

- [ ] **Step 7: Commit**

```bash
git add envs/prod/ k8s/prod/
git commit -m "feat: add production infrastructure-as-code

envs/prod/ with cluster autoscaler, production-sized node groups.
k8s/prod/ with HPA (min 2, max 10, 70% CPU target) and PDB
(minAvailable: 1) for all services. Not deployed — portfolio-ready."
```

---

## Final Verification

- [ ] **Step 1: Build all Go services**

Run: `cd c:/projects/my-saas-platform && go build ./...`
Expected: Clean build, no errors.

- [ ] **Step 2: Verify all files committed**

Run: `git status`
Expected: Clean working tree.

- [ ] **Step 3: Review commit history**

Run: `git log --oneline -15`
Expected: 13 focused commits covering all scalability improvements.
