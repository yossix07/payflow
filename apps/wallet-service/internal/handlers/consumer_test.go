package handlers

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/payflow/wallet-service/internal/events"
	"github.com/payflow/wallet-service/internal/repository"
)

func TestReserveFunds_Success(t *testing.T) {
	repo := newMockRepository()
	ob := newMockOutbox()
	q := &mockQueueClient{}
	consumer := NewEventConsumer(q, repo, ob)

	// Seed wallet with $500
	repo.wallets["user-1"] = &repository.Wallet{
		UserID:  "user-1",
		Balance: 500.0,
	}

	event := events.ReserveFundsEvent{
		PaymentID: "pay-1",
		UserID:    "user-1",
		Amount:    100.0,
		Timestamp: time.Now().Format(time.RFC3339),
	}

	err := consumer.handleReserveFunds(context.Background(), event)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Verify balance deducted to $400
	wallet := repo.wallets["user-1"]
	if wallet.Balance != 400.0 {
		t.Errorf("expected balance 400, got %.2f", wallet.Balance)
	}

	// Verify reservation created
	res, ok := repo.reservations["pay-1"]
	if !ok {
		t.Fatal("expected reservation to be created")
	}
	if res.Status != "ACTIVE" {
		t.Errorf("expected reservation status ACTIVE, got %s", res.Status)
	}
	if res.Amount != 100.0 {
		t.Errorf("expected reservation amount 100, got %.2f", res.Amount)
	}

	// Verify FundsReserved event published
	if len(ob.messages) != 1 {
		t.Fatalf("expected 1 outbox message, got %d", len(ob.messages))
	}
	if ob.messages[0].eventType != events.EventFundsReserved {
		t.Errorf("expected event type %s, got %s", events.EventFundsReserved, ob.messages[0].eventType)
	}

	// Verify payload contains payment ID
	var published events.FundsReservedEvent
	if err := json.Unmarshal([]byte(ob.messages[0].payload), &published); err != nil {
		t.Fatalf("failed to unmarshal outbox payload: %v", err)
	}
	if published.PaymentID != "pay-1" {
		t.Errorf("expected payment_id pay-1 in event, got %s", published.PaymentID)
	}
}

func TestReserveFunds_InsufficientBalance(t *testing.T) {
	repo := newMockRepository()
	ob := newMockOutbox()
	q := &mockQueueClient{}
	consumer := NewEventConsumer(q, repo, ob)

	// Seed wallet with $50
	repo.wallets["user-2"] = &repository.Wallet{
		UserID:  "user-2",
		Balance: 50.0,
	}

	event := events.ReserveFundsEvent{
		PaymentID: "pay-2",
		UserID:    "user-2",
		Amount:    100.0,
		Timestamp: time.Now().Format(time.RFC3339),
	}

	err := consumer.handleReserveFunds(context.Background(), event)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Verify balance unchanged
	wallet := repo.wallets["user-2"]
	if wallet.Balance != 50.0 {
		t.Errorf("expected balance 50 (unchanged), got %.2f", wallet.Balance)
	}

	// Verify no reservation created
	if _, ok := repo.reservations["pay-2"]; ok {
		t.Error("expected no reservation to be created for insufficient funds")
	}

	// Verify InsufficientFunds event published
	if len(ob.messages) != 1 {
		t.Fatalf("expected 1 outbox message, got %d", len(ob.messages))
	}
	if ob.messages[0].eventType != events.EventInsufficientFunds {
		t.Errorf("expected event type %s, got %s", events.EventInsufficientFunds, ob.messages[0].eventType)
	}
}

func TestReleaseFunds_RestoresBalance(t *testing.T) {
	repo := newMockRepository()
	ob := newMockOutbox()
	q := &mockQueueClient{}
	consumer := NewEventConsumer(q, repo, ob)

	// Seed wallet with $400 (after a $100 reservation)
	repo.wallets["user-3"] = &repository.Wallet{
		UserID:  "user-3",
		Balance: 400.0,
	}

	// Seed active reservation
	repo.reservations["pay-3"] = &repository.Reservation{
		ReservationID: "res-3",
		PaymentID:     "pay-3",
		UserID:        "user-3",
		Amount:        100.0,
		Status:        "ACTIVE",
		CreatedAt:     time.Now(),
		ExpiresAt:     time.Now().Add(24 * time.Hour).Unix(),
	}

	event := events.ReleaseFundsEvent{
		PaymentID: "pay-3",
		Timestamp: time.Now().Format(time.RFC3339),
	}

	err := consumer.handleReleaseFunds(context.Background(), event)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Verify balance restored to $500
	wallet := repo.wallets["user-3"]
	if wallet.Balance != 500.0 {
		t.Errorf("expected balance 500, got %.2f", wallet.Balance)
	}

	// Verify reservation status is RELEASED
	res := repo.reservations["pay-3"]
	if res.Status != "RELEASED" {
		t.Errorf("expected reservation status RELEASED, got %s", res.Status)
	}

	// Verify FundsReleased event published
	if len(ob.messages) != 1 {
		t.Fatalf("expected 1 outbox message, got %d", len(ob.messages))
	}
	if ob.messages[0].eventType != events.EventFundsReleased {
		t.Errorf("expected event type %s, got %s", events.EventFundsReleased, ob.messages[0].eventType)
	}
}
