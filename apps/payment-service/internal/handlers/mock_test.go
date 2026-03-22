package handlers

import (
	"context"
	"fmt"
	"sync"

	"github.com/payflow/payment-service/internal/model"
	"github.com/payflow/payment-service/pkg/outbox"
)

type mockRepository struct {
	mu       sync.Mutex
	payments map[string]*model.Payment
}

func newMockRepository() *mockRepository {
	return &mockRepository{payments: make(map[string]*model.Payment)}
}

func (m *mockRepository) SavePayment(ctx context.Context, payment *model.Payment) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.payments[payment.PaymentID] = payment
	return nil
}

func (m *mockRepository) GetPayment(ctx context.Context, paymentID string) (*model.Payment, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, ok := m.payments[paymentID]
	if !ok {
		return nil, fmt.Errorf("payment not found: %s", paymentID)
	}
	return p, nil
}

func (m *mockRepository) CheckIdempotency(ctx context.Context, key string) (string, int, bool, error) {
	return "", 0, false, nil
}

func (m *mockRepository) SaveIdempotency(ctx context.Context, key, response string, statusCode int) error {
	return nil
}

type mockOutbox struct {
	mu       sync.Mutex
	messages []outbox.OutboxMessage
}

func newMockOutbox() *mockOutbox {
	return &mockOutbox{}
}

func (m *mockOutbox) WriteMessage(ctx context.Context, eventType, payload string) error {
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
