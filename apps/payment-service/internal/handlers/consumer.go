package handlers

import (
	"context"
	"encoding/json"
	"log"

	"github.com/my-saas-platform/payment-service/internal/events"
	"github.com/my-saas-platform/payment-service/internal/saga"
	"github.com/my-saas-platform/payment-service/pkg/queue"
)

type EventConsumer struct {
	queue        queue.QueueClient
	orchestrator *saga.Orchestrator
}

func NewEventConsumer(queue queue.QueueClient, orchestrator *saga.Orchestrator) *EventConsumer {
	return &EventConsumer{
		queue:        queue,
		orchestrator: orchestrator,
	}
}

func (ec *EventConsumer) Start(ctx context.Context) {
	log.Println("Starting event consumer...")

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
					// Delete message from queue
					ec.queue.DeleteMessage(ctx, msg.ReceiptHandle)
				}
			}
		}
	}
}

func (ec *EventConsumer) handleMessage(ctx context.Context, msg queue.Message) error {
	log.Printf("Received event: %s", msg.EventType)

	switch msg.EventType {
	case events.EventFundsReserved:
		var event events.FundsReservedEvent
		if err := json.Unmarshal([]byte(msg.Body), &event); err != nil {
			return err
		}
		return ec.orchestrator.HandleFundsReserved(ctx, event.PaymentID)

	case events.EventInsufficientFunds:
		var event events.InsufficientFundsEvent
		if err := json.Unmarshal([]byte(msg.Body), &event); err != nil {
			return err
		}
		return ec.orchestrator.HandlePaymentFailed(ctx, event.PaymentID, "insufficient funds")

	case events.EventPaymentSucceeded:
		var event events.PaymentSucceededEvent
		if err := json.Unmarshal([]byte(msg.Body), &event); err != nil {
			return err
		}
		return ec.orchestrator.HandlePaymentSucceeded(ctx, event.PaymentID)

	case events.EventPaymentFailed:
		var event events.PaymentFailedEvent
		if err := json.Unmarshal([]byte(msg.Body), &event); err != nil {
			return err
		}
		return ec.orchestrator.HandlePaymentFailed(ctx, event.PaymentID, event.Reason)

	default:
		log.Printf("Ignoring event type: %s", msg.EventType)
	}

	return nil
}
