package saga

import (
	"context"
	"fmt"
	"sync"

	"github.com/payflow/payment-service/internal/model"
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

func (m *mockRepository) SaveIdempotency(ctx context.Context, key string, response string, statusCode int) error {
	return nil
}
