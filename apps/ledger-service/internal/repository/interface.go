package repository

import "context"

type Repository interface {
	RecordEntry(ctx context.Context, entry *LedgerEntry) error
	GetEntries(ctx context.Context, limit int) ([]*LedgerEntry, error)
	GetEntriesByPayment(ctx context.Context, paymentID string) ([]*LedgerEntry, error)
}
