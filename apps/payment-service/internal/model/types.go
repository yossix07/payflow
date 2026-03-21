package model

import (
	"time"
)

// PaymentState represents the current state in the saga
type PaymentState string

const (
	StatePending       PaymentState = "PENDING"
	StateFundsReserved PaymentState = "FUNDS_RESERVED"
	StateProcessing    PaymentState = "PROCESSING"
	StateCompleted     PaymentState = "COMPLETED"
	StateFailed        PaymentState = "FAILED"
	StateCompensating  PaymentState = "COMPENSATING"
	StateCancelled     PaymentState = "CANCELLED"
	StateTimedOut      PaymentState = "TIMED_OUT"
)

// Payment represents a payment saga instance
type Payment struct {
	PaymentID      string       `dynamodbav:"payment_id"`
	UserID         string       `dynamodbav:"user_id"`
	Amount         float64      `dynamodbav:"amount"`
	State          PaymentState `dynamodbav:"state"`
	IdempotencyKey string       `dynamodbav:"idempotency_key"`
	Steps          []SagaStep   `dynamodbav:"steps"`
	CreatedAt      time.Time    `dynamodbav:"created_at"`
	UpdatedAt      time.Time    `dynamodbav:"updated_at"`
}

// SagaStep represents a single step in the saga execution
type SagaStep struct {
	StepName    string    `dynamodbav:"step_name"`
	State       string    `dynamodbav:"state"`
	EventType   string    `dynamodbav:"event_type"`
	ExecutedAt  time.Time `dynamodbav:"executed_at"`
	ErrorReason string    `dynamodbav:"error_reason,omitempty"`
}

// CreatePaymentRequest represents the incoming payment request
type CreatePaymentRequest struct {
	UserID string  `json:"user_id"`
	Amount float64 `json:"amount"`
}

// PaymentResponse represents the API response
type PaymentResponse struct {
	PaymentID string       `json:"payment_id"`
	UserID    string       `json:"user_id"`
	Amount    float64      `json:"amount"`
	State     PaymentState `json:"state"`
	CreatedAt time.Time    `json:"created_at"`
}
