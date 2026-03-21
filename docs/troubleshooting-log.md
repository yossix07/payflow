# Troubleshooting Log

A running log of bugs encountered in this project, their root causes, and how they were resolved.

---

## 1. Payment creation returns 500 "Failed to create payment"

**Date:** 2026-03-21

**Symptom:** `POST /payments` returned HTTP 500 with body "Failed to create payment". The payment-service logs showed `Publishing event: PaymentStarted` but then the request failed.

**Root Cause:** The `OutboxMessage` struct in `apps/payment-service/pkg/outbox/interface.go` had no `dynamodbav` struct tags:

```go
// BROKEN — fields marshal as PascalCase (MessageID, EventType, etc.)
type OutboxMessage struct {
    MessageID string
    EventType string
    Payload   string
    Published bool
    CreatedAt string
}
```

The AWS SDK's `attributevalue.MarshalMap()` uses field names as-is when no tags are present, producing DynamoDB attribute names like `MessageID`. But the DynamoDB table was created with `message_id` as the partition key (snake_case). DynamoDB rejected the PutItem because the required key `message_id` was missing from the item.

The actual error was: `ValidationException: One of the required keys was not given a value`

This error wasn't visible initially because the payment handler caught it generically without logging.

**Solution:**
1. Added `dynamodbav` struct tags to match the table schema:
```go
type OutboxMessage struct {
    MessageID string `dynamodbav:"message_id"`
    EventType string `dynamodbav:"event_type"`
    Payload   string `dynamodbav:"payload"`
    Published int    `dynamodbav:"published"`
    CreatedAt string `dynamodbav:"created_at"`
}
```
2. Changed `Published` from `bool` to `int` because the DynamoDB table's GSI (`published-index`) uses a Number (`N`) type, not Boolean. A Go `bool` would serialize as `{"BOOL": false}` instead of `{"N": "0"}`.
3. Added error logging to the payment handler so future errors are visible in logs.
4. Applied the same fix to wallet-service's `OutboxMessage`.

**Lesson:** Always verify that Go struct tags match DynamoDB table schemas. The `attributevalue.MarshalMap` function silently uses PascalCase field names when tags are missing — it won't error, but the resulting item won't have the keys DynamoDB expects.

---

## 2. Dashboard UI shows no events after triggering payments

**Date:** 2026-03-21

**Symptom:** Clicking "Trigger Payment" in the dashboard returned 201 (payment created), but nothing appeared in the event log, saga panel, or topology animation. The SSE connection was established (connection status showed "Connected"), but no events flowed through.

**Root Cause:** An SQS queue routing problem. The architecture has 5 separate SQS queues (one per service: `payment-queue`, `wallet-queue`, `gateway-queue`, `ledger-queue`, `notification-queue`). Each service's outbox worker was publishing events **only to its own queue**. Each service's consumer was reading **only from its own queue**.

This meant:
- Payment-service published `PaymentStarted` and `ReserveFunds` to `payment-queue`
- Wallet-service was reading from `wallet-queue` — never saw the `ReserveFunds` event
- Notification-service was reading from `notification-queue` — never saw any events
- The saga was stuck after the first step because events couldn't cross service boundaries

**Why it wasn't caught earlier:** The original docker-compose.yml was committed before the services were fully wired. The queue names looked correct individually, but the cross-service routing was never tested end-to-end until the dashboard visualization made the gap visible.

**Solution:** Implemented **outbox fan-out** — each outbox worker now broadcasts events to ALL service queues, not just its own. Each service still consumes from its own queue and ignores events it doesn't handle (via `switch`/`default` in the consumer).

Changes:
1. **Go outbox worker** (`payment-service`, `wallet-service`): Changed `Worker.queue` from a single `queue.QueueClient` to `Worker.queues []queue.QueueClient`. The `processOutbox` method now loops over all queues for each message.
2. **Node.js outbox worker** (`gateway-service`): Added `BROADCAST_QUEUE_URLS` env var support. The worker sends each message to all queue URLs in the list.
3. **docker-compose.yml**: Added `BROADCAST_QUEUE_URLS` env var to payment-service, wallet-service, and gateway-service, listing all 5 queue URLs.
4. **init-localstack.sh**: Restored all 5 individual queues (was briefly changed to a single shared queue, which caused competing-consumer problems).

**Lesson:** In an event-driven microservices architecture with per-service queues, you need an explicit fan-out mechanism. Options are:
- **SNS + SQS fan-out** (production pattern): one SNS topic, each service subscribes its queue
- **Application-level fan-out** (what we did): outbox workers publish to all queues
- **Single shared queue** (doesn't work with SQS): competing consumers mean each message goes to only one service

The application-level fan-out is pragmatic for local dev but doesn't scale. For production, migrate to SNS fan-out.

---

## 3. Shell injection vulnerability in open-browser.js

**Date:** 2026-03-21

**Symptom:** Code review flagged that `scripts/open-browser.js` passed user input (URL from argv) directly into a shell command via `exec()`.

**Root Cause:**
```javascript
// VULNERABLE — URL is interpolated into shell command
const cmd = process.platform === "win32" ? `start ${url}` : ...;
exec(cmd);
```

A malicious URL like `http://example.com; rm -rf /` would be interpreted by the shell.

**Solution:** Switched from `exec` (which spawns a shell) to `execFile` (which bypasses the shell and passes arguments directly to the executable):

```javascript
const { execFile } = require("child_process");
const openers = {
  win32: ["cmd", ["/c", "start", url]],
  darwin: ["open", [url]],
  linux: ["xdg-open", [url]],
};
const [bin, args] = openers[process.platform] || openers.linux;
execFile(bin, args, ...);
```

**Lesson:** Never interpolate user input into shell commands. Use `execFile` or `spawn` (which don't invoke a shell) instead of `exec`. This applies even for internal tooling — it's a bad habit that gets copied.

---

## 4. Integration test teardown skipped on environment startup failure

**Date:** 2026-03-21

**Symptom:** Code review found that `scripts/test-integration.js` would leave Docker containers running if `env-up.js` failed.

**Root Cause:** The original code had separate try/catch blocks:
```javascript
try {
  execSync("node scripts/env-up.js", ...);  // If this fails...
} catch (err) {
  process.exit(1);  // ...exit immediately, skipping teardown below
}
// Teardown only runs if env-up succeeded
execSync("docker compose down -v", ...);
```

**Solution:** Wrapped both setup and test in a single try/finally:
```javascript
try {
  execSync("node scripts/env-up.js", ...);
  execSync("go test ...", ...);
} catch (err) {
  testFailed = true;
} finally {
  // Always runs, even if env-up fails partway through
  try { execSync("docker compose down -v", ...); } catch (e) { ... }
}
```

**Lesson:** Any script that creates resources (containers, infra) must guarantee cleanup in a `finally` block. Don't put cleanup after the happy path — it won't run on the failure path.
