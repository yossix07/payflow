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
	creditWallet(t, userID, 1000.0)

	balance := getWalletBalance(t, userID)
	if balance != 1000.0 {
		t.Fatalf("expected balance 1000, got %f", balance)
	}

	paymentID := createPayment(t, userID, 50.0)
	t.Logf("Created payment: %s", paymentID)

	finalState := waitForPaymentState(t, paymentID, []string{"COMPLETED", "FAILED"}, 30*time.Second)
	t.Logf("Payment final state: %s", finalState)

	entries := getLedgerEntries(t, paymentID)
	if len(entries) == 0 {
		t.Error("expected ledger entries for payment, got none")
	}
	t.Logf("Ledger entries: %d", len(entries))

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
	paymentID := createPayment(t, userID, 500.0)
	t.Logf("Created payment: %s", paymentID)

	finalState := waitForPaymentState(t, paymentID, []string{"FAILED"}, 30*time.Second)
	if finalState != "FAILED" {
		t.Errorf("expected FAILED, got %s", finalState)
	}

	entries := getLedgerEntries(t, paymentID)
	if len(entries) == 0 {
		t.Error("expected ledger entries for failed payment")
	}
}
