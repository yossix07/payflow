package repository

import (
	"context"
	"time"
)

// Wallet represents a user's wallet
type Wallet struct {
	UserID    string    `dynamodbav:"user_id"`
	Balance   float64   `dynamodbav:"balance"`
	CreatedAt time.Time `dynamodbav:"created_at"`
	UpdatedAt time.Time `dynamodbav:"updated_at"`
}

// Reservation represents a temporary hold on funds
type Reservation struct {
	ReservationID string    `dynamodbav:"reservation_id"`
	PaymentID     string    `dynamodbav:"payment_id"`
	UserID        string    `dynamodbav:"user_id"`
	Amount        float64   `dynamodbav:"amount"`
	Status        string    `dynamodbav:"status"` // ACTIVE, RELEASED
	CreatedAt     time.Time `dynamodbav:"created_at"`
	ExpiresAt     int64     `dynamodbav:"expires_at"` // TTL
}

// Repository defines the interface for wallet operations
type Repository interface {
	// Wallet operations
	GetWallet(ctx context.Context, userID string) (*Wallet, error)
	CreateWallet(ctx context.Context, wallet *Wallet) error
	UpdateWallet(ctx context.Context, wallet *Wallet) error

	// Reservation operations
	CreateReservation(ctx context.Context, reservation *Reservation) error
	GetReservationByPayment(ctx context.Context, paymentID string) (*Reservation, error)
	UpdateReservation(ctx context.Context, reservation *Reservation) error
}
