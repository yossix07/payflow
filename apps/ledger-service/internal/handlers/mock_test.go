package handlers

import (
	"context"
	"sync"

	"github.com/my-saas-platform/ledger-service/internal/repository"
)

type mockRepository struct {
	mu      sync.Mutex
	entries []*repository.LedgerEntry
}

func newMockRepository() *mockRepository {
	return &mockRepository{}
}

func (m *mockRepository) RecordEntry(_ context.Context, entry *repository.LedgerEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries = append(m.entries, entry)
	return nil
}

func (m *mockRepository) GetEntries(_ context.Context, limit int) ([]*repository.LedgerEntry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if limit > len(m.entries) {
		limit = len(m.entries)
	}
	return m.entries[:limit], nil
}

func (m *mockRepository) GetEntriesByPayment(_ context.Context, paymentID string) ([]*repository.LedgerEntry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []*repository.LedgerEntry
	for _, e := range m.entries {
		if e.PaymentID == paymentID {
			result = append(result, e)
		}
	}
	return result, nil
}
