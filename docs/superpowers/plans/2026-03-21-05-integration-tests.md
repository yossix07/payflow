# Plan 5: Integration Tests

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Create end-to-end integration tests that verify full saga flows against the real Docker Compose environment (all services + LocalStack).

**Architecture:** Go test files in `tests/integration/` that make HTTP calls to services and assert on DynamoDB state via LocalStack. Tests use the Go `testing` package with `TestMain` for setup. Each test runs a complete saga flow and verifies the final state across all services.

**Tech Stack:** Go testing, AWS SDK v2 (for DynamoDB assertions), HTTP client, SSE client

**Spec:** `docs/superpowers/specs/2026-03-21-local-test-deploy-visualization-design.md`

**Depends on:** Plan 1 (npm task runner for env:up), Plan 3 (SSE migration)

---

### Task 1: Create integration test module

**Files:**
- Create: `tests/integration/go.mod`
- Modify: `go.work`

- [ ] **Step 1: Create go.mod for integration tests**

```bash
mkdir -p tests/integration
cd tests/integration
go mod init saas-platform/tests/integration
go get github.com/aws/aws-sdk-go-v2/config
go get github.com/aws/aws-sdk-go-v2/service/dynamodb
go get github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue
go get github.com/aws/aws-sdk-go-v2/credentials
```

- [ ] **Step 2: Add to go.work**

Modify `go.work` to add:

```
use ./tests/integration
```

- [ ] **Step 3: Verify workspace compiles**

Run: `go build ./...`
Expected: All modules compile

- [ ] **Step 4: Commit**

```bash
git add tests/integration/go.mod tests/integration/go.sum go.work
git commit -m "add integration test module to Go workspace"
```

---

### Task 2: Create test helpers

**Files:**
- Create: `tests/integration/helpers_test.go`

- [ ] **Step 1: Write test helpers**

```go
package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

var (
	paymentServiceURL      = envOrDefault("PAYMENT_SERVICE_URL", "http://localhost:8081")
	walletServiceURL       = envOrDefault("WALLET_SERVICE_URL", "http://localhost:8083")
	ledgerServiceURL       = envOrDefault("LEDGER_SERVICE_URL", "http://localhost:8082")
	notificationServiceURL = envOrDefault("NOTIFICATION_SERVICE_URL", "http://localhost:8085")
	localstackEndpoint     = envOrDefault("LOCALSTACK_ENDPOINT", "http://localhost:4566")
)

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func newDynamoDBClient(t *testing.T) *dynamodb.Client {
	t.Helper()
	cfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion("us-east-1"),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("test", "test", "")),
	)
	if err != nil {
		t.Fatalf("failed to load AWS config: %v", err)
	}

	return dynamodb.NewFromConfig(cfg, func(o *dynamodb.Options) {
		o.BaseEndpoint = aws.String(localstackEndpoint)
	})
}

func createPayment(t *testing.T, userID string, amount float64) string {
	t.Helper()
	body, _ := json.Marshal(map[string]interface{}{
		"user_id": userID,
		"amount":  amount,
	})

	req, _ := http.NewRequest("POST", paymentServiceURL+"/payments", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", fmt.Sprintf("test-%d", time.Now().UnixNano()))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to create payment: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 201, got %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	return result["payment_id"].(string)
}

func creditWallet(t *testing.T, userID string, amount float64) {
	t.Helper()
	body, _ := json.Marshal(map[string]interface{}{
		"amount": amount,
	})

	resp, err := http.Post(walletServiceURL+"/wallets/"+userID+"/credit", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("failed to credit wallet: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 from credit, got %d", resp.StatusCode)
	}
}

func getPaymentState(t *testing.T, paymentID string) string {
	t.Helper()
	db := newDynamoDBClient(t)

	out, err := db.GetItem(context.Background(), &dynamodb.GetItemInput{
		TableName: aws.String("payments"),
		Key: map[string]types.AttributeValue{
			"payment_id": &types.AttributeValueMemberS{Value: paymentID},
		},
	})
	if err != nil {
		t.Fatalf("failed to get payment: %v", err)
	}

	var result map[string]interface{}
	attributevalue.UnmarshalMap(out.Item, &result)
	return fmt.Sprintf("%v", result["state"])
}

func waitForPaymentState(t *testing.T, paymentID string, wantStates []string, timeout time.Duration) string {
	t.Helper()
	deadline := time.Now().Add(timeout)
	want := make(map[string]bool)
	for _, s := range wantStates {
		want[s] = true
	}

	for time.Now().Before(deadline) {
		state := getPaymentState(t, paymentID)
		if want[state] {
			return state
		}
		time.Sleep(500 * time.Millisecond)
	}

	t.Fatalf("payment %s did not reach states %v within %s (last state: %s)",
		paymentID, wantStates, timeout, getPaymentState(t, paymentID))
	return ""
}

func getWalletBalance(t *testing.T, userID string) float64 {
	t.Helper()
	resp, err := http.Get(walletServiceURL + "/wallets/" + userID)
	if err != nil {
		t.Fatalf("failed to get wallet: %v", err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	return result["balance"].(float64)
}

func getLedgerEntries(t *testing.T, paymentID string) []map[string]interface{} {
	t.Helper()
	resp, err := http.Get(ledgerServiceURL + "/ledger/payment/" + paymentID)
	if err != nil {
		t.Fatalf("failed to get ledger: %v", err)
	}
	defer resp.Body.Close()

	var entries []map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&entries)
	return entries
}
```

- [ ] **Step 2: Commit**

```bash
git add tests/integration/helpers_test.go
git commit -m "add integration test helpers for service and DynamoDB assertions"
```

---

### Task 3: Happy path saga flow test

**Files:**
- Create: `tests/integration/saga_flow_test.go`

- [ ] **Step 1: Write happy path test**

```go
package integration

import (
	"testing"
	"time"
)

func TestSagaFlow_HappyPath(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	userID := "integ-test-happy"

	// Seed wallet with enough funds
	creditWallet(t, userID, 1000.0)

	// Verify wallet balance
	balance := getWalletBalance(t, userID)
	if balance != 1000.0 {
		t.Fatalf("expected balance 1000, got %f", balance)
	}

	// Create payment
	paymentID := createPayment(t, userID, 50.0)
	t.Logf("Created payment: %s", paymentID)

	// Wait for saga to complete (either COMPLETED or FAILED — gateway has 80% success rate)
	finalState := waitForPaymentState(t, paymentID, []string{"COMPLETED", "FAILED"}, 30*time.Second)
	t.Logf("Payment final state: %s", finalState)

	// Verify ledger has entries for this payment
	entries := getLedgerEntries(t, paymentID)
	if len(entries) == 0 {
		t.Error("expected ledger entries for payment, got none")
	}
	t.Logf("Ledger entries: %d", len(entries))

	// If completed, wallet should have reduced balance
	if finalState == "COMPLETED" {
		balance = getWalletBalance(t, userID)
		if balance >= 1000.0 {
			t.Errorf("expected balance < 1000 after payment, got %f", balance)
		}
	}
}

func TestSagaFlow_InsufficientFunds(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	userID := "integ-test-broke"

	// Don't seed wallet — balance is 0

	// Create payment that requires funds
	paymentID := createPayment(t, userID, 500.0)
	t.Logf("Created payment: %s", paymentID)

	// Should fail due to insufficient funds
	finalState := waitForPaymentState(t, paymentID, []string{"FAILED"}, 30*time.Second)

	if finalState != "FAILED" {
		t.Errorf("expected FAILED, got %s", finalState)
	}

	// Ledger should record the failure
	entries := getLedgerEntries(t, paymentID)
	if len(entries) == 0 {
		t.Error("expected ledger entries for failed payment")
	}
}
```

- [ ] **Step 2: Commit**

```bash
git add tests/integration/saga_flow_test.go
git commit -m "add integration tests for happy path and insufficient funds saga flows"
```

---

### Task 4: Idempotency test

**Files:**
- Create: `tests/integration/idempotency_test.go`

- [ ] **Step 1: Write idempotency test**

```go
package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"
)

func TestIdempotency_DuplicateRequest(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	userID := "integ-test-idempotent"
	creditWallet(t, userID, 1000.0)

	body, _ := json.Marshal(map[string]interface{}{
		"user_id": userID,
		"amount":  75.0,
	})

	idempotencyKey := "test-idem-key-001"

	// First request
	req1, _ := http.NewRequest("POST", paymentServiceURL+"/payments", bytes.NewReader(body))
	req1.Header.Set("Content-Type", "application/json")
	req1.Header.Set("Idempotency-Key", idempotencyKey)

	resp1, err := http.DefaultClient.Do(req1)
	if err != nil {
		t.Fatalf("first request failed: %v", err)
	}
	defer resp1.Body.Close()

	var result1 map[string]interface{}
	json.NewDecoder(resp1.Body).Decode(&result1)
	paymentID1 := result1["payment_id"].(string)

	// Second request with same idempotency key
	body2, _ := json.Marshal(map[string]interface{}{
		"user_id": userID,
		"amount":  75.0,
	})
	req2, _ := http.NewRequest("POST", paymentServiceURL+"/payments", bytes.NewReader(body2))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Idempotency-Key", idempotencyKey)

	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatalf("second request failed: %v", err)
	}
	defer resp2.Body.Close()

	var result2 map[string]interface{}
	json.NewDecoder(resp2.Body).Decode(&result2)
	paymentID2 := result2["payment_id"].(string)

	// Both should return the same payment ID
	if paymentID1 != paymentID2 {
		t.Errorf("idempotency failed: got different payment IDs: %s vs %s", paymentID1, paymentID2)
	}
}
```

- [ ] **Step 2: Commit**

```bash
git add tests/integration/idempotency_test.go
git commit -m "add integration test for idempotency"
```

---

### Task 5: Run integration tests end-to-end

- [ ] **Step 1: Start environment**

Run: `npm run env:up`
Expected: All services healthy

- [ ] **Step 2: Run integration tests**

Run: `go test -v -timeout 120s ./tests/integration/...`
Expected: All tests pass

- [ ] **Step 3: Tear down**

Run: `npm run env:down`

- [ ] **Step 4: Test via npm script**

Run: `npm run test:integration`
Expected: Environment starts, tests run, environment tears down. All pass.

- [ ] **Step 5: Commit any fixes**

```bash
git add -A tests/integration/
git commit -m "finalize integration tests"
```
