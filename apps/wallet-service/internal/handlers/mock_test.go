package handlers

import (
	"context"
	"fmt"
	"sync"

	"github.com/my-saas-platform/wallet-service/internal/repository"
	"github.com/my-saas-platform/wallet-service/pkg/outbox"
	"github.com/my-saas-platform/wallet-service/pkg/queue"
)

// ---------- Mock Repository ----------

type mockRepository struct {
	mu           sync.Mutex
	wallets      map[string]*repository.Wallet
	reservations map[string]*repository.Reservation // keyed by paymentID
}

func newMockRepository() *mockRepository {
	return &mockRepository{
		wallets:      make(map[string]*repository.Wallet),
		reservations: make(map[string]*repository.Reservation),
	}
}

func (m *mockRepository) GetWallet(_ context.Context, userID string) (*repository.Wallet, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	w, ok := m.wallets[userID]
	if !ok {
		return nil, fmt.Errorf("wallet not found for user %s", userID)
	}
	copy := *w
	return &copy, nil
}

func (m *mockRepository) CreateWallet(_ context.Context, wallet *repository.Wallet) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.wallets[wallet.UserID] = wallet
	return nil
}

func (m *mockRepository) UpdateWallet(_ context.Context, wallet *repository.Wallet) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.wallets[wallet.UserID] = wallet
	return nil
}

func (m *mockRepository) CreateReservation(_ context.Context, reservation *repository.Reservation) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.reservations[reservation.PaymentID] = reservation
	return nil
}

func (m *mockRepository) GetReservationByPayment(_ context.Context, paymentID string) (*repository.Reservation, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.reservations[paymentID]
	if !ok {
		return nil, fmt.Errorf("reservation not found for payment %s", paymentID)
	}
	copy := *r
	return &copy, nil
}

func (m *mockRepository) UpdateReservation(_ context.Context, reservation *repository.Reservation) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.reservations[reservation.PaymentID] = reservation
	return nil
}

// ---------- Mock Outbox ----------

type outboxEntry struct {
	eventType string
	payload   string
}

type mockOutbox struct {
	mu       sync.Mutex
	messages []outboxEntry
}

func newMockOutbox() *mockOutbox {
	return &mockOutbox{}
}

func (m *mockOutbox) WriteMessage(_ context.Context, eventType string, payload string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = append(m.messages, outboxEntry{eventType: eventType, payload: payload})
	return nil
}

func (m *mockOutbox) GetUnpublishedMessages(_ context.Context) ([]outbox.OutboxMessage, error) {
	return nil, nil
}

func (m *mockOutbox) MarkAsPublished(_ context.Context, _ string) error {
	return nil
}

// ---------- Mock QueueClient ----------

type mockQueueClient struct{}

func (m *mockQueueClient) SendMessage(_ context.Context, _ string) error { return nil }
func (m *mockQueueClient) ReceiveMessages(_ context.Context) ([]queue.Message, error) {
	return nil, nil
}
func (m *mockQueueClient) DeleteMessage(_ context.Context, _ string) error { return nil }
