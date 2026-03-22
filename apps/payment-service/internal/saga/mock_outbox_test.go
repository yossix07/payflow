package saga

import (
	"context"
	"sync"

	"github.com/payflow/payment-service/pkg/outbox"
)

type mockOutbox struct {
	mu       sync.Mutex
	messages []outbox.OutboxMessage
}

func newMockOutbox() *mockOutbox {
	return &mockOutbox{}
}

func (m *mockOutbox) WriteMessage(ctx context.Context, eventType string, payload string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = append(m.messages, outbox.OutboxMessage{EventType: eventType, Payload: payload})
	return nil
}

func (m *mockOutbox) GetUnpublishedMessages(ctx context.Context) ([]outbox.OutboxMessage, error) {
	return nil, nil
}

func (m *mockOutbox) MarkAsPublished(ctx context.Context, messageID string) error {
	return nil
}

func (m *mockOutbox) getMessages() []outbox.OutboxMessage {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([]outbox.OutboxMessage, len(m.messages))
	copy(cp, m.messages)
	return cp
}
