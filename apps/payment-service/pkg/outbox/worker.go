package outbox

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/my-saas-platform/payment-service/pkg/queue"
)

const maxOutboxRetries = 5

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

		// Broadcast to all queues, tracking per-queue success
		allSent := true
		for _, q := range w.queues {
			if err := sendWithRetry(ctx, q, string(body)); err != nil {
				log.Printf("Failed to publish message %s to queue after retries: %v", msg.MessageID, err)
				allSent = false
			}
		}

		if allSent {
			if err := w.outbox.MarkAsPublished(ctx, msg.MessageID); err != nil {
				log.Printf("Failed to mark message as published: %v", err)
			} else {
				log.Printf("Published event: %s (message_id: %s)", msg.EventType, msg.MessageID)
			}
		} else {
			if err := w.outbox.IncrementRetryCount(ctx, msg.MessageID); err != nil {
				log.Printf("Failed to increment retry count for message %s: %v", msg.MessageID, err)
			}
			if msg.RetryCount+1 >= maxOutboxRetries {
				log.Printf("Marking message %s as failed after %d retries", msg.MessageID, msg.RetryCount+1)
				if err := w.outbox.MarkAsFailed(ctx, msg.MessageID); err != nil {
					log.Printf("Failed to mark message %s as failed: %v", msg.MessageID, err)
				}
			}
		}
	}

	return nil
}

func sendWithRetry(ctx context.Context, q queue.QueueClient, body string) error {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(attempt) * 500 * time.Millisecond
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}
		}
		if err := q.SendMessage(ctx, body); err != nil {
			lastErr = err
			continue
		}
		return nil
	}
	return fmt.Errorf("all 3 send attempts failed: %w", lastErr)
}
