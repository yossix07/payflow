package handlers

import (
	"context"
	"encoding/json"
	"log"

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
	log.Println("Starting ledger event consumer...")

	for {
		select {
		case <-ctx.Done():
			log.Println("Event consumer stopped")
			return
		default:
			messages, err := ec.queue.ReceiveMessages(ctx)
			if err != nil {
				log.Printf("Error receiving messages: %v", err)
				continue
			}

			for _, msg := range messages {
				if err := ec.handleMessage(ctx, msg); err != nil {
					log.Printf("Error handling message: %v", err)
				} else {
					ec.queue.DeleteMessage(ctx, msg.ReceiptHandle)
				}
			}
		}
	}
}

func (ec *EventConsumer) handleMessage(ctx context.Context, msg queue.Message) error {
	log.Printf("Recording ledger entry for event: %s", msg.EventType)

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

	log.Printf("Ledger entry recorded: %s for payment %s", msg.EventType, paymentID)
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
