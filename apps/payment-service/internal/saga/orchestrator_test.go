package saga

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/payflow/payment-service/internal/model"
)

func TestStartPayment_CreatesPaymentInPendingState(t *testing.T) {
	repo := newMockRepository()
	ob := newMockOutbox()
	orch := NewOrchestrator(repo, ob)

	req := model.CreatePaymentRequest{UserID: "user-1", Amount: 100.0}
	payment, err := orch.StartPayment(context.Background(), req, "idem-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if payment.PaymentID == "" {
		t.Fatal("expected payment ID to be set")
	}
	if payment.UserID != "user-1" {
		t.Errorf("expected user-1, got %s", payment.UserID)
	}
	if payment.Amount != 100.0 {
		t.Errorf("expected 100.0, got %f", payment.Amount)
	}
	msgs := ob.getMessages()
	if len(msgs) < 2 {
		t.Fatalf("expected at least 2 outbox messages, got %d", len(msgs))
	}
	if msgs[0].EventType != "PaymentStarted" {
		t.Errorf("expected PaymentStarted, got %s", msgs[0].EventType)
	}
	if msgs[1].EventType != "ReserveFunds" {
		t.Errorf("expected ReserveFunds, got %s", msgs[1].EventType)
	}
}

func TestHandleFundsReserved_TransitionsToProcessing(t *testing.T) {
	repo := newMockRepository()
	ob := newMockOutbox()
	orch := NewOrchestrator(repo, ob)

	req := model.CreatePaymentRequest{UserID: "user-1", Amount: 50.0}
	payment, _ := orch.StartPayment(context.Background(), req, "idem-2")
	err := orch.HandleFundsReserved(context.Background(), payment.PaymentID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	saved, _ := repo.GetPayment(context.Background(), payment.PaymentID)
	if saved.State != model.StateProcessing {
		t.Errorf("expected state PROCESSING, got %s", saved.State)
	}
	msgs := ob.getMessages()
	found := false
	for _, m := range msgs {
		if m.EventType == "ProcessPayment" {
			found = true
		}
	}
	if !found {
		t.Error("expected ProcessPayment event in outbox")
	}
}

func TestHandlePaymentSucceeded_TransitionsToCompleted(t *testing.T) {
	repo := newMockRepository()
	ob := newMockOutbox()
	orch := NewOrchestrator(repo, ob)

	req := model.CreatePaymentRequest{UserID: "user-1", Amount: 25.0}
	payment, _ := orch.StartPayment(context.Background(), req, "idem-3")
	_ = orch.HandleFundsReserved(context.Background(), payment.PaymentID)
	err := orch.HandlePaymentSucceeded(context.Background(), payment.PaymentID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	saved, _ := repo.GetPayment(context.Background(), payment.PaymentID)
	if saved.State != model.StateCompleted {
		t.Errorf("expected state COMPLETED, got %s", saved.State)
	}
	msgs := ob.getMessages()
	found := false
	for _, m := range msgs {
		if m.EventType == "SendNotification" {
			var payload map[string]interface{}
			json.Unmarshal([]byte(m.Payload), &payload)
			if payload["status"] == "success" {
				found = true
			}
		}
	}
	if !found {
		t.Error("expected SendNotification with status=success")
	}
}

func TestHandlePaymentFailed_TransitionsToFailed(t *testing.T) {
	repo := newMockRepository()
	ob := newMockOutbox()
	orch := NewOrchestrator(repo, ob)

	req := model.CreatePaymentRequest{UserID: "user-1", Amount: 25.0}
	payment, _ := orch.StartPayment(context.Background(), req, "idem-4")
	err := orch.HandlePaymentFailed(context.Background(), payment.PaymentID, "gateway declined")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	saved, _ := repo.GetPayment(context.Background(), payment.PaymentID)
	if saved.State != model.StateFailed {
		t.Errorf("expected state FAILED, got %s", saved.State)
	}
}
