package outbox

import (
	"context"
)

// Outbox defines the interface for the transactional outbox pattern
type Outbox interface {
	WriteMessage(ctx context.Context, eventType string, payload string) error
	GetUnpublishedMessages(ctx context.Context) ([]OutboxMessage, error)
	MarkAsPublished(ctx context.Context, messageID string) error
}

// OutboxMessage represents a message in the outbox
type OutboxMessage struct {
	MessageID string `dynamodbav:"message_id"`
	EventType string `dynamodbav:"event_type"`
	Payload   string `dynamodbav:"payload"`
	Published int    `dynamodbav:"published"`
	CreatedAt string `dynamodbav:"created_at"`
}
