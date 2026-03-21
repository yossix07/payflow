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
	body, _ := json.Marshal(map[string]interface{}{"amount": amount})
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
	t.Fatalf("payment %s did not reach states %v within %s", paymentID, wantStates, timeout)
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
