# Scalability & Consistency Improvements — Design Spec

**Date**: 2026-03-21
**Scope**: Production-grade at moderate scale (~1K-10K req/sec) with room for future high-scale (100K+)
**Cost strategy**: Code fixes applied to all environments; `envs/prod/` Terraform created but not deployed; dev stays ~$5/month
**Saga timeout approach**: DynamoDB TTL + scheduled scanner (upgradeable to SQS delay queues later)

---

## Section 1: Data Correctness

### 1a. Wallet Optimistic Locking

**Problem**: `GetWallet()` → modify balance → `UpdateWallet()` is a classic TOCTOU race. Two concurrent requests can silently lose money.

**Fix**:
- Add a `version` integer attribute to the wallets DynamoDB table
- Every `UpdateWallet` call uses `ConditionExpression: "version = :expected_version"` and sets `version = version + 1`
- On `ConditionalCheckFailedException`: retry with a fresh read (up to 3 retries with jitter)
- **Retry placement**: The retry logic wraps the entire read-modify-write sequence at the handler/consumer level (not inside `UpdateWallet` alone), because the caller must re-read the wallet to get fresh state. Add a helper function (e.g., `updateWalletWithRetry(ctx, repo, userID, modifyFn)`) that encapsulates: GetWallet → apply `modifyFn` → UpdateWallet with version check → retry on conflict.
- Applies to:
  - `apps/wallet-service/internal/handlers/wallet.go` (CreditWallet, DebitWallet)
  - `apps/wallet-service/internal/handlers/consumer.go` (ReserveFunds, ReleaseFunds)
  - `apps/wallet-service/internal/repository/dynamodb.go` (UpdateWallet method)
- Same pattern applied to the reservations table

**Atomic reservation + debit**: In `handleReserveFunds`, both `CreateReservation` and `UpdateWallet` (deduct balance) must be retried together as a unit. On retry, `CreateReservation` must use `attribute_not_exists(reservation_id)` conditional put so it is idempotent — if the reservation already exists from a prior attempt, skip creation and only retry the balance deduction. This prevents orphaned reservations on optimistic lock failure.

**DynamoDB schema change**: Add `version` (Number) attribute. No table recreation needed — DynamoDB is schemaless for non-key attributes. Existing records without `version` treated as version 0.

### 1b. Outbox Worker Error Handling

**Problem**: If `SendMessage` to SQS fails, the outbox worker logs the error but still marks the message as published. Events are silently lost.

**Fix**:
- Do NOT call `MarkAsPublished` if `SendMessage` fails
- Add retry with exponential backoff (3 attempts, base 100ms) before giving up on a message
- Add a `retry_count` integer field to outbox records
- On each failed attempt, increment `retry_count`
- After max retries (configurable, default 5), mark message as `failed` instead of `published` (poison pill)
- Log at ERROR level for failed messages with full context (message_id, queue_url, error)
- **Fan-out partial failure**: The outbox worker broadcasts to multiple queues. Track per-queue success for each message. On retry, only re-send to queues that failed — do not re-send to queues that already succeeded. Only call `MarkAsPublished` when ALL queues have been successfully sent to. This relies on downstream consumers being idempotent (Section 1c covers gateway; payment-service and notification-service consumers already handle duplicates via state checks or are inherently idempotent for notifications).
- Applies to:
  - `apps/payment-service/pkg/outbox/worker.go`
  - `apps/wallet-service/pkg/outbox/worker.go`
  - `apps/gateway-service/src/workers/outboxWorker.js`

### 1c. Gateway-Service Idempotency

**Problem**: Gateway-service has no deduplication. With 2 replicas polling the same SQS queue, the same payment can be processed twice within the 30s visibility window.

**Fix**:
- The `gateway_transactions` table uses `transaction_id` as its hash key, not `payment_id`. The `payment_id` is only accessible via the `payment-index` GSI, which makes a query-then-put non-atomic.
- Instead, add a dedicated deduplication record: before processing, do a `PutItem` to a `gateway_idempotency` table (keyed by `payment_id`) with `ConditionExpression: "attribute_not_exists(payment_id)"`. This mirrors the pattern already used by payment-service's idempotency table.
- If the condition fails (duplicate), skip processing and delete the SQS message.
- Add the `gateway_idempotency` table to `modules/messaging/main.tf` (simple table: hash key `payment_id` (S), TTL on `expires_at`).
- Applies to:
  - `apps/gateway-service/src/consumers/eventConsumer.js`
  - `apps/gateway-service/src/repository/transactionRepository.js` (or new dedup repository)
  - `modules/messaging/main.tf` (new table)

---

## Section 2: Throughput & Efficiency

### 2a. SQS Consumer Loop Fix (Go Services)

**Problem**: The `select { default: ReceiveMessages() }` pattern never blocks, creating a tight CPU-burning busy-loop that also hammers SQS API limits.

**Fix**:
- Remove the `default` case from the `select` block
- After `ReceiveMessages` returns no messages: wait 1s before next poll (backoff)
- After `ReceiveMessages` returns messages: poll again immediately (maintain throughput)
- The existing 20s long-poll (`WaitTimeSeconds: 20`) on SQS handles the "no messages" case efficiently — the backoff is a safety net
- Applies to:
  - `apps/payment-service/internal/handlers/consumer.go`
  - `apps/wallet-service/internal/handlers/consumer.go`
  - `apps/ledger-service/internal/handlers/consumer.go`

### 2b. Ledger Scan to Query

**Problem**: `GetEntriesByPayment` uses `Scan` + `FilterExpression` even though `payment_id` is already the hash key. This reads every item in the table.

**Fix**:
- Replace `Scan` with `Query` using `KeyConditionExpression: "payment_id = :pid"`
- Single method change in `apps/ledger-service/internal/repository/dynamodb.go`
- 1000x cost reduction at scale (reads only matching items instead of full table)

### 2c. Outbox Polling Backpressure

**Problem**: 100ms fixed poll interval is too aggressive when idle and creates unnecessary DynamoDB load.

**Fix**:
- Base interval: 500ms (up from 100ms)
- Adaptive polling:
  - If batch returns full (10 items) → poll again immediately (drain burst)
  - If batch returns partial → use base interval (500ms)
  - If batch returns empty → exponential backoff up to 5s
  - Reset to base interval on next non-empty batch
- **Go implementation note**: The current Go workers use `time.NewTicker` which has a fixed interval. Switch to `time.NewTimer` with `Reset()` to support variable intervals.
- Applies to:
  - `apps/payment-service/pkg/outbox/worker.go`
  - `apps/wallet-service/pkg/outbox/worker.go`
  - `apps/gateway-service/src/workers/outboxWorker.js`

---

## Section 3: Reliability

### 3a. Saga Timeout & Compensation

**Problem**: If a service crashes mid-saga, payments get stuck in non-terminal states forever with no detection or recovery.

**Fix**:
- Ensure `created_at` and `updated_at` timestamps exist on payment records
- New `SagaTimeoutScanner` goroutine in payment-service:
  - Runs every 30s
  - Queries for payments in non-terminal states (`PENDING`, `PROCESSING`, `FUNDS_RESERVED`) with `updated_at` older than 60s
  - For stuck payments: update state to `TIMED_OUT`, publish `ReleaseFunds` compensation event to wallet queue
- Add a DynamoDB GSI on `state` (hash key, type S) + `updated_at` (range key, type S — ISO-8601 format for correct lexicographic ordering) to the payments table. Both attributes must be declared in the Terraform `aws_dynamodb_table.payments` resource's `attribute` blocks. Add a `TIMED_OUT` constant to `apps/payment-service/internal/model/types.go`.
- The scanner uses conditional writes to prevent race conditions with normal saga progression: `ConditionExpression: "state = :expected_state"` — only transition if state hasn't changed since read.
- **Late arrival handling**: `HandlePaymentSucceeded` and `HandlePaymentFailed` in the saga orchestrator must check if the payment is already in `TIMED_OUT` state and skip processing if so. A late gateway success after timeout creates a known inconsistency (gateway records SUCCESS, payment records TIMED_OUT) — this is acceptable and logged for manual reconciliation.

**Future upgrade path**: Replace polling scanner with SQS delay queues for near-real-time timeout detection. The compensation logic stays the same.

### 3b. Graceful Shutdown (Go Services)

**Problem**: Goroutines are fire-and-forget with a 5s shutdown timeout. In-flight messages are lost on pod eviction.

**Fix**:
- Change `context.Background()` to `context.WithCancel(context.Background())` in each service's `main()`. The cancel function is called in the shutdown handler.
- Add `sync.WaitGroup` to track all worker goroutines (outbox worker, event consumer, saga timeout scanner)
- On SIGTERM: cancel context, then `wg.Wait()` with 10s deadline
- Workers check `ctx.Done()` between loop iterations and exit cleanly
- Applies to: `apps/payment-service/main.go`, `apps/wallet-service/main.go`, `apps/ledger-service/main.go`

### 3c. Graceful Shutdown (Node.js Services)

**Problem**: `process.exit(0)` on SIGTERM kills in-flight work immediately.

**Fix**:
- Add a `shuttingDown` flag (module-level boolean)
- On SIGTERM: set flag, stop accepting new SSE connections, stop SQS polling
- All `while(true)` loops (in both `eventConsumer.js` and `outboxWorker.js`) check the flag and break when set
- Wait for in-flight SQS message processing to complete (up to 10s)
- Then exit
- Applies to: `apps/gateway-service/src/index.js`, `apps/gateway-service/src/consumers/eventConsumer.js`, `apps/gateway-service/src/workers/outboxWorker.js`, `apps/notification-service/src/index.js`, `apps/notification-service/src/consumers/eventConsumer.js`

### 3d. SSE Connection Management

**Problem**: No connection limits, no zombie cleanup, potential memory leak as connections accumulate.

**Fix**:
- Max connection limit: configurable via env var, default 1000. Return 503 when limit reached.
- Heartbeat: send `:ping` comment every 30s. If `res.write()` fails, remove from Set (zombie cleanup).
- Idle timeout: configurable via env var, default 15 minutes. "Idle" means no successful write (heartbeats reset the timer, so only truly dead connections are affected).
- Applies to: `apps/notification-service/src/sse/sseManager.js`

---

## Section 4: Observability

### 4a. Structured Logging

**Problem**: `log.Printf` and `console.log` produce unstructured output that's hard to search/filter in production.

**Fix**:
- Go services: replace `log.Printf` with `log/slog` (stdlib since Go 1.21). Set default logger in `main()` via `slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))`, then use package-level `slog.Info`/`slog.Error` calls throughout. Structured JSON output with `timestamp`, `level`, `service`, `payment_id`, `error` fields.
- Node.js services: small wrapper around `JSON.stringify` for structured JSON output. Same fields. No new dependencies.
- Applies to all services.

### 4b. Health Check Enrichment

**Problem**: `/healthz` returns 200 unconditionally. A pod with broken DynamoDB connectivity still receives traffic.

**Fix**:
- `/healthz` (liveness): stays simple — "process is alive". Used by K8s liveness probe.
- `/readyz` (readiness): checks DynamoDB and SQS connectivity. Returns 200 if healthy, 503 if dependencies unreachable. Used by K8s readiness probe.
- Update all K8s deployment manifests: `readinessProbe.httpGet.path: /readyz`
- Applies to all services.

### 4c. Lightweight Metrics Endpoint

**Problem**: No visibility into system health beyond logs.

**Fix**:
- Add `/metrics` JSON endpoint to payment-service returning:
  - `outbox_queue_depth`: count of unpublished outbox messages
  - `stuck_sagas`: count of payments in non-terminal state older than 60s
  - `messages_published_total`: counter of successfully published messages
  - `messages_failed_total`: counter of failed publishes
- Simple JSON, not Prometheus format (that's a future B-level concern)
- Dashboard can poll this endpoint for display

---

## Section 5: Infrastructure-as-Code (`envs/prod/`)

### 5a. Production Terraform Environment

- New `envs/prod/` directory mirroring `envs/dev/` structure
- Reuses existing modules (`network`, `compute`, `messaging`) with production-appropriate variable values
- No module duplication — only different `terraform.tfvars`

### 5b. Key Differences from Dev

| Concern | Dev (`envs/dev/`) | Prod (`envs/prod/`) |
|---------|-------------------|---------------------|
| NAT Gateway | Single AZ (cost saving) | One per AZ (2 total, HA) |
| EKS node instances | t3.medium | t3.large |
| EKS node scaling | min 2, max 3 | min 3, max 10 |
| DynamoDB billing | PAY_PER_REQUEST | PAY_PER_REQUEST (handles up to 40K RCU/WCU per table, sufficient for target scale; switch to provisioned only if cost optimization is needed) |
| Node groups | Single managed group | Separate system + workload groups |

### 5c. Kubernetes Production Manifests

- Stored in `k8s/prod/` overlay directory alongside existing dev manifests
- HorizontalPodAutoscaler per service: min 2 replicas, max 10, target 70% CPU
- PodDisruptionBudget per service: `minAvailable: 1`
- Readiness probes updated to `/readyz`
- Notification-service: min 2 replicas (eliminates SPOF)

### 5d. Cluster Autoscaler

- Helm release for `cluster-autoscaler` added in `envs/prod/main.tf`
- Not included in dev (cost saving, not needed for local testing)

### 5e. Documented but Not Implemented (Future B-Level)

The following are documented as "next steps" in this spec but are out of scope:
- Service mesh (Istio/Linkerd) for traffic management and circuit breaking
- Prometheus + Grafana monitoring stack
- Multi-region deployment and disaster recovery
- Redis/ElastiCache layer for read caching
- SQS FIFO queues for strict ordering
- SQS delay queues for saga timeouts (upgrade from polling scanner)

---

## Files Affected (Summary)

**Wallet-service** (optimistic locking, consumer fix, outbox, shutdown):
- `apps/wallet-service/internal/repository/dynamodb.go`
- `apps/wallet-service/internal/handlers/wallet.go`
- `apps/wallet-service/internal/handlers/consumer.go`
- `apps/wallet-service/pkg/outbox/worker.go`
- `apps/wallet-service/main.go`

**Payment-service** (consumer fix, outbox, saga timeout, shutdown, metrics):
- `apps/payment-service/internal/handlers/consumer.go`
- `apps/payment-service/internal/saga/orchestrator.go`
- `apps/payment-service/pkg/outbox/worker.go`
- `apps/payment-service/main.go`
- `apps/payment-service/internal/repository/dynamodb.go` (new GSI query for stuck sagas)
- New: `apps/payment-service/internal/saga/timeout_scanner.go`

**Ledger-service** (query fix, consumer fix, shutdown):
- `apps/ledger-service/internal/repository/dynamodb.go`
- `apps/ledger-service/internal/handlers/consumer.go`
- `apps/ledger-service/main.go`

**Gateway-service** (idempotency, outbox, shutdown):
- `apps/gateway-service/src/consumers/eventConsumer.js`
- `apps/gateway-service/src/repository/transactionRepository.js` (or new dedup repository)
- `apps/gateway-service/src/workers/outboxWorker.js`
- `apps/gateway-service/src/index.js`

**Notification-service** (SSE management, shutdown):
- `apps/notification-service/src/sse/sseManager.js`
- `apps/notification-service/src/consumers/eventConsumer.js`
- `apps/notification-service/src/index.js`

**All services** (structured logging, health checks):
- All handler/main files updated for `slog`/structured logging
- All `main.go`/`index.js` updated with `/readyz` endpoint
- All K8s deployment manifests updated for readiness probes

**Infrastructure**:
- `modules/messaging/main.tf` (new GSI on payments table for saga timeout scanner, new `gateway_idempotency` table)
- `apps/payment-service/internal/model/types.go` (add `TIMED_OUT` state constant)
- New: `envs/prod/` directory (main.tf, variables.tf, provider.tf, terraform.tfvars)
- New: `k8s/prod/` directory (HPA, PDB, production deployment overlays)

---

## Implementation Order

Dependencies between sections dictate the following order:

1. **Quick wins (independent, no dependencies)**:
   - 2a: SQS consumer loop fix (all Go services)
   - 2b: Ledger Scan → Query
   - 1a: Wallet optimistic locking

2. **Outbox improvements (1b before 2c)**:
   - 1b: Outbox error handling (fan-out partial failure, retry, poison pill)
   - 2c: Outbox adaptive polling (builds on same code as 1b)

3. **Gateway idempotency (independent, requires new DynamoDB table)**:
   - 1c: Gateway-service idempotency (new table in Terraform + consumer code)

4. **Graceful shutdown (before saga timeout — scanner needs lifecycle management)**:
   - 3b: Go graceful shutdown (context.WithCancel, WaitGroup)
   - 3c: Node.js graceful shutdown (shuttingDown flag)

5. **Saga timeout (depends on 3b for goroutine lifecycle)**:
   - 3a: Saga timeout scanner (new GSI, new goroutine, TIMED_OUT state, late arrival handling)

6. **SSE hardening (independent)**:
   - 3d: SSE connection management

7. **Observability (builds on all code changes)**:
   - 4a: Structured logging (touch all services)
   - 4b: Health check enrichment (/readyz)
   - 4c: Metrics endpoint

8. **Infrastructure (independent of code, can be done in parallel)**:
   - 5a-5d: envs/prod/ Terraform + k8s/prod/ manifests

---

## Known Limitations (Out of Scope)

- **float64 for money**: Wallet `Balance` and payment `Amount` use `float64` throughout. This is a known precision risk for financial systems. Flagged as a future improvement (switch to integer cents or `decimal` type) but out of scope for this spec.
- **Single-region**: No multi-region DR. Acceptable for portfolio project.
- **No distributed tracing**: Correlation IDs exist (payment_id) but no OpenTelemetry or X-Ray integration.
