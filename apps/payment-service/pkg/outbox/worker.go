package outbox

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/my-saas-platform/payment-service/pkg/queue"
)

// Worker polls the outbox and publishes messages to all queues
type Worker struct {
	outbox   Outbox
	queues   []queue.QueueClient
	interval time.Duration
}

func NewWorker(outbox Outbox, queues []queue.QueueClient, interval time.Duration) *Worker {
	return &Worker{
		outbox:   outbox,
		queues:   queues,
		interval: interval,
	}
}

func (w *Worker) Start(ctx context.Context) {
	log.Println("Starting outbox worker...")
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("Outbox worker stopped")
			return
		case <-ticker.C:
			if err := w.processOutbox(ctx); err != nil {
				log.Printf("Error processing outbox: %v", err)
			}
		}
	}
}

func (w *Worker) processOutbox(ctx context.Context) error {
	messages, err := w.outbox.GetUnpublishedMessages(ctx)
	if err != nil {
		return err
	}

	for _, msg := range messages {
		// Create message wrapper with event type
		wrapper := map[string]interface{}{
			"event_type": msg.EventType,
			"payload":    json.RawMessage(msg.Payload),
		}

		body, err := json.Marshal(wrapper)
		if err != nil {
			log.Printf("Failed to marshal message: %v", err)
			continue
		}

		// Broadcast to all queues
		for _, q := range w.queues {
			if err := q.SendMessage(ctx, string(body)); err != nil {
				log.Printf("Failed to publish message %s to queue: %v", msg.MessageID, err)
			}
		}

		// Mark as published
		if err := w.outbox.MarkAsPublished(ctx, msg.MessageID); err != nil {
			log.Printf("Failed to mark message as published: %v", err)
			// Note: The message might be published twice if this fails
		} else {
			log.Printf("Published event: %s (message_id: %s)", msg.EventType, msg.MessageID)
		}
	}

	return nil
}
