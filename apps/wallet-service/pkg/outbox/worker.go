package outbox

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"time"

	"github.com/my-saas-platform/wallet-service/pkg/queue"
)

const maxOutboxRetries = 5

func getEnvDuration(key string, defaultMs int) time.Duration {
	if v := os.Getenv(key); v != "" {
		if ms, err := strconv.Atoi(v); err == nil {
			return time.Duration(ms)
		}
	}
	return time.Duration(defaultMs)
}

// Worker polls the outbox and publishes messages to all queues
type Worker struct {
	outbox Outbox
	queues []queue.QueueClient
}

func NewWorker(outbox Outbox, queues []queue.QueueClient) *Worker {
	return &Worker{
		outbox: outbox,
		queues: queues,
	}
}

func (w *Worker) Start(ctx context.Context) {
	slog.Info("Starting outbox worker")

	baseInterval := getEnvDuration("OUTBOX_POLL_INTERVAL_MS", 100) * time.Millisecond
	maxInterval := getEnvDuration("OUTBOX_MAX_INTERVAL_MS", 5000) * time.Millisecond
	currentInterval := baseInterval

	timer := time.NewTimer(currentInterval)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("Outbox worker stopped")
			return
		case <-timer.C:
			count, err := w.processOutbox(ctx)
			if err != nil {
				slog.Error("Error processing outbox", "error", err)
				currentInterval = baseInterval
			} else if count >= 10 {
				currentInterval = 0
			} else if count > 0 {
				currentInterval = baseInterval
			} else {
				currentInterval *= 2
				if currentInterval > maxInterval {
					currentInterval = maxInterval
				}
			}

			if currentInterval == 0 {
				timer.Reset(time.Millisecond)
			} else {
				timer.Reset(currentInterval)
			}
		}
	}
}

func (w *Worker) processOutbox(ctx context.Context) (int, error) {
	messages, err := w.outbox.GetUnpublishedMessages(ctx)
	if err != nil {
		return 0, err
	}

	for _, msg := range messages {
		wrapper := map[string]interface{}{
			"event_type": msg.EventType,
			"payload":    json.RawMessage(msg.Payload),
		}

		body, err := json.Marshal(wrapper)
		if err != nil {
			slog.Error("Failed to marshal message", "error", err)
			continue
		}

		// Broadcast to all queues, tracking per-queue success
		allSent := true
		for _, q := range w.queues {
			if err := sendWithRetry(ctx, q, string(body)); err != nil {
				slog.Error("Failed to publish message to queue after retries", "message_id", msg.MessageID, "error", err)
				allSent = false
			}
		}

		if allSent {
			if err := w.outbox.MarkAsPublished(ctx, msg.MessageID); err != nil {
				slog.Error("Failed to mark message as published", "message_id", msg.MessageID, "error", err)
			} else {
				slog.Info("Published event", "event_type", msg.EventType, "message_id", msg.MessageID)
			}
		} else {
			if err := w.outbox.IncrementRetryCount(ctx, msg.MessageID); err != nil {
				slog.Error("Failed to increment retry count for message", "message_id", msg.MessageID, "error", err)
			}
			if msg.RetryCount+1 >= maxOutboxRetries {
				slog.Error("Marking message as failed after max retries", "message_id", msg.MessageID, "retries", msg.RetryCount+1)
				if err := w.outbox.MarkAsFailed(ctx, msg.MessageID); err != nil {
					slog.Error("Failed to mark message as failed", "message_id", msg.MessageID, "error", err)
				}
			}
		}
	}

	return len(messages), nil
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
