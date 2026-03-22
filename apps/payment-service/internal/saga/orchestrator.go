package saga

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/payflow/payment-service/internal/events"
	"github.com/payflow/payment-service/internal/model"
	"github.com/payflow/payment-service/internal/repository"
	"github.com/payflow/payment-service/pkg/outbox"
)

// Orchestrator manages the payment saga
type Orchestrator struct {
	repo   repository.Repository
	outbox outbox.Outbox
}

// NewOrchestrator creates a new saga orchestrator
func NewOrchestrator(repo repository.Repository, outbox outbox.Outbox) *Orchestrator {
	return &Orchestrator{
		repo:   repo,
		outbox: outbox,
	}
}

// StartPayment initiates a new payment saga
func (o *Orchestrator) StartPayment(ctx context.Context, req model.CreatePaymentRequest, idempotencyKey string) (*model.Payment, error) {
	paymentID := uuid.New().String()
	now := time.Now()

	payment := &model.Payment{
		PaymentID:      paymentID,
		UserID:         req.UserID,
		Amount:         req.Amount,
		State:          model.StatePending,
		IdempotencyKey: idempotencyKey,
		Steps:          []model.SagaStep{},
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	// Save payment to repository
	if err := o.repo.SavePayment(ctx, payment); err != nil {
		return nil, fmt.Errorf("failed to save payment: %w", err)
	}

	// Add step
	payment.Steps = append(payment.Steps, model.SagaStep{
		StepName:   "PaymentStarted",
		State:      "COMPLETED",
		EventType:  events.EventPaymentStarted,
		ExecutedAt: now,
	})

	// Publish PaymentStarted event (via outbox)
	event := events.PaymentStartedEvent{
		PaymentID: paymentID,
		UserID:    req.UserID,
		Amount:    req.Amount,
		Timestamp: now.Format(time.RFC3339),
	}

	if err := o.publishEvent(ctx, events.EventPaymentStarted, event); err != nil {
		return nil, fmt.Errorf("failed to publish event: %w", err)
	}

	// Transition to next step: reserve funds
	if err := o.ReserveFunds(ctx, payment); err != nil {
		return nil, fmt.Errorf("failed to reserve funds: %w", err)
	}

	return payment, nil
}

// ReserveFunds publishes the ReserveFunds event
func (o *Orchestrator) ReserveFunds(ctx context.Context, payment *model.Payment) error {
	event := events.ReserveFundsEvent{
		PaymentID: payment.PaymentID,
		UserID:    payment.UserID,
		Amount:    payment.Amount,
		Timestamp: time.Now().Format(time.RFC3339),
	}

	payment.Steps = append(payment.Steps, model.SagaStep{
		StepName:   "ReserveFunds",
		State:      "PENDING",
		EventType:  events.EventReserveFunds,
		ExecutedAt: time.Now(),
	})

	if err := o.repo.SavePayment(ctx, payment); err != nil {
		return err
	}

	return o.publishEvent(ctx, events.EventReserveFunds, event)
}

// HandleFundsReserved processes the FundsReserved event
func (o *Orchestrator) HandleFundsReserved(ctx context.Context, paymentID string) error {
	payment, err := o.repo.GetPayment(ctx, paymentID)
	if err != nil {
		return err
	}

	payment.State = model.StateFundsReserved
	payment.UpdatedAt = time.Now()
	payment.Steps = append(payment.Steps, model.SagaStep{
		StepName:   "FundsReserved",
		State:      "COMPLETED",
		EventType:  events.EventFundsReserved,
		ExecutedAt: time.Now(),
	})

	if err := o.repo.SavePayment(ctx, payment); err != nil {
		return err
	}

	// Next step: process payment via gateway
	return o.ProcessPayment(ctx, payment)
}

// ProcessPayment publishes the ProcessPayment event
func (o *Orchestrator) ProcessPayment(ctx context.Context, payment *model.Payment) error {
	event := events.ProcessPaymentEvent{
		PaymentID: payment.PaymentID,
		UserID:    payment.UserID,
		Amount:    payment.Amount,
		Timestamp: time.Now().Format(time.RFC3339),
	}

	payment.State = model.StateProcessing
	payment.Steps = append(payment.Steps, model.SagaStep{
		StepName:   "ProcessPayment",
		State:      "PENDING",
		EventType:  events.EventProcessPayment,
		ExecutedAt: time.Now(),
	})

	if err := o.repo.SavePayment(ctx, payment); err != nil {
		return err
	}

	return o.publishEvent(ctx, events.EventProcessPayment, event)
}

// HandlePaymentSucceeded processes the PaymentSucceeded event
func (o *Orchestrator) HandlePaymentSucceeded(ctx context.Context, paymentID string) error {
	payment, err := o.repo.GetPayment(ctx, paymentID)
	if err != nil {
		return err
	}

	if payment.State == model.StateTimedOut {
		slog.Info("Ignoring late PaymentSucceeded for timed-out payment", "payment_id", paymentID)
		return nil
	}

	payment.State = model.StateCompleted
	payment.UpdatedAt = time.Now()
	payment.Steps = append(payment.Steps, model.SagaStep{
		StepName:   "PaymentSucceeded",
		State:      "COMPLETED",
		EventType:  events.EventPaymentSucceeded,
		ExecutedAt: time.Now(),
	})

	if err := o.repo.SavePayment(ctx, payment); err != nil {
		return err
	}

	// Send notification
	return o.SendNotification(ctx, payment, "success")
}

// HandlePaymentFailed processes the PaymentFailed event
func (o *Orchestrator) HandlePaymentFailed(ctx context.Context, paymentID, reason string) error {
	payment, err := o.repo.GetPayment(ctx, paymentID)
	if err != nil {
		return err
	}

	if payment.State == model.StateTimedOut {
		slog.Info("Ignoring late PaymentFailed for timed-out payment", "payment_id", paymentID)
		return nil
	}

	payment.State = model.StateFailed
	payment.UpdatedAt = time.Now()
	payment.Steps = append(payment.Steps, model.SagaStep{
		StepName:    "PaymentFailed",
		State:       "COMPLETED",
		EventType:   events.EventPaymentFailed,
		ExecutedAt:  time.Now(),
		ErrorReason: reason,
	})

	if err := o.repo.SavePayment(ctx, payment); err != nil {
		return err
	}

	// Send notification
	return o.SendNotification(ctx, payment, "failed")
}

// SendNotification publishes notification event
func (o *Orchestrator) SendNotification(ctx context.Context, payment *model.Payment, status string) error {
	event := events.SendNotificationEvent{
		PaymentID: payment.PaymentID,
		UserID:    payment.UserID,
		Status:    status,
		Amount:    payment.Amount,
		Timestamp: time.Now().Format(time.RFC3339),
	}

	return o.publishEvent(ctx, events.EventSendNotification, event)
}

// publishEvent writes event to outbox
func (o *Orchestrator) publishEvent(ctx context.Context, eventType string, event interface{}) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	slog.Info("Publishing event", "event_type", eventType)
	return o.outbox.WriteMessage(ctx, eventType, string(payload))
}
