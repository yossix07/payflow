package repository

import (
	"context"

	"github.com/my-saas-platform/payment-service/internal/model"
)

// Repository defines the interface for payment data operations
type Repository interface {
	// Payment operations
	SavePayment(ctx context.Context, payment *model.Payment) error
	GetPayment(ctx context.Context, paymentID string) (*model.Payment, error)

	// Idempotency operations
	CheckIdempotency(ctx context.Context, key string) (string, int, bool, error)
	SaveIdempotency(ctx context.Context, key string, response string, statusCode int) error
}
