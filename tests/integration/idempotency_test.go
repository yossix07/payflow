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

	if paymentID1 != paymentID2 {
		t.Errorf("idempotency failed: got different payment IDs: %s vs %s", paymentID1, paymentID2)
	}
}
