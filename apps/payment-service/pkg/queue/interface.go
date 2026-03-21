package queue

import (
	"context"
)

// Message represents a message from the queue
type Message struct {
	MessageID     string
	ReceiptHandle string
	EventType     string
	Body          string
}

// QueueClient defines the interface for queue operations
type QueueClient interface {
	SendMessage(ctx context.Context, body string) error
	ReceiveMessages(ctx context.Context) ([]Message, error)
	DeleteMessage(ctx context.Context, receiptHandle string) error
}
