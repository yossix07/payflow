package handlers

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

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
	slog.Info("Starting event consumer")

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
	slog.Info("Received event", "event_type", msg.EventType)

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
		slog.Info("Ignoring event type", "event_type", msg.EventType)
	}

	return nil
}
