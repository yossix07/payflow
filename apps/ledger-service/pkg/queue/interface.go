package queue

import (
	"context"
)

type Message struct {
	MessageID     string
	ReceiptHandle string
	EventType     string
	Body          string
}

type QueueClient interface {
	SendMessage(ctx context.Context, body string) error
	ReceiveMessages(ctx context.Context) ([]Message, error)
	DeleteMessage(ctx context.Context, receiptHandle string) error
}
