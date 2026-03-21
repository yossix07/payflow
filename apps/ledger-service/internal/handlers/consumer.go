package handlers

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/my-saas-platform/ledger-service/internal/repository"
	"github.com/my-saas-platform/ledger-service/pkg/queue"
)

type EventConsumer struct {
	queue queue.QueueClient
	repo  repository.Repository
}

func NewEventConsumer(queue queue.QueueClient, repo repository.Repository) *EventConsumer {
	return &EventConsumer{
		queue: queue,
		repo:  repo,
	}
}

func (ec *EventConsumer) Start(ctx context.Context) {
	slog.Info("Starting ledger event consumer")

	for {
		select {
		case <-ctx.Done():
			slog.Info("Event consumer stopped")
			return
		default:
		}

		messages, err := ec.queue.ReceiveMessages(ctx)
		if err != nil {
			slog.Error("Error receiving messages", "error", err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(1 * time.Second):
			}
			continue
		}

		for _, msg := range messages {
			if err := ec.handleMessage(ctx, msg); err != nil {
				slog.Error("Error handling message", "error", err)
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

func (ec *EventConsumer) handleMessage(ctx context.Context, msg queue.Message) error {
	slog.Info("Recording ledger entry for event", "event_type", msg.EventType)

	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(msg.Body), &payload); err != nil {
		return err
	}

	paymentID, _ := payload["payment_id"].(string)
	userID, _ := payload["user_id"].(string)
	amount, _ := payload["amount"].(float64)

	entry := &repository.LedgerEntry{
		PaymentID:   paymentID,
		EventType:   msg.EventType,
		Amount:      amount,
		UserID:      userID,
		Description: ec.getDescription(msg.EventType, payload),
	}

	if err := ec.repo.RecordEntry(ctx, entry); err != nil {
		return err
	}

	slog.Info("Ledger entry recorded", "event_type", msg.EventType, "payment_id", paymentID)
	return nil
}

func (ec *EventConsumer) getDescription(eventType string, payload map[string]interface{}) string {
	switch eventType {
	case "PaymentStarted":
		return "Payment initiated"
	case "FundsReserved":
		return "Funds reserved from wallet"
	case "PaymentSucceeded":
		txnID, _ := payload["transaction_id"].(string)
		return "Payment processed successfully (txn: " + txnID + ")"
	case "PaymentFailed":
		reason, _ := payload["reason"].(string)
		return "Payment failed: " + reason
	case "InsufficientFunds":
		return "Payment failed due to insufficient funds"
	default:
		return eventType
	}
}
