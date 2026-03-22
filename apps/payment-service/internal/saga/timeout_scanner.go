package saga

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	ddbTypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/payflow/payment-service/internal/events"
	"github.com/payflow/payment-service/internal/model"
	"github.com/payflow/payment-service/internal/repository"
	"github.com/payflow/payment-service/pkg/outbox"
)

type TimeoutScanner struct {
	repo         *repository.DynamoDBRepository
	outbox       outbox.Outbox
	scanInterval time.Duration
	timeoutAge   time.Duration
}

func NewTimeoutScanner(repo *repository.DynamoDBRepository, outbox outbox.Outbox) *TimeoutScanner {
	return &TimeoutScanner{
		repo:         repo,
		outbox:       outbox,
		scanInterval: 30 * time.Second,
		timeoutAge:   60 * time.Second,
	}
}

func (s *TimeoutScanner) Start(ctx context.Context) {
	slog.Info("Starting saga timeout scanner")
	timer := time.NewTimer(s.scanInterval)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("Saga timeout scanner stopped")
			return
		case <-timer.C:
			s.scanForStuckPayments(ctx)
			timer.Reset(s.scanInterval)
		}
	}
}

func (s *TimeoutScanner) scanForStuckPayments(ctx context.Context) {
	threshold := time.Now().Add(-s.timeoutAge)
	stuckStates := []model.PaymentState{
		model.StatePending,
		model.StateProcessing,
		model.StateFundsReserved,
	}

	for _, state := range stuckStates {
		payments, err := s.repo.GetStuckPayments(ctx, state, threshold)
		if err != nil {
			slog.Error("Error scanning for stuck payments", "state", state, "error", err)
			continue
		}

		for _, payment := range payments {
			s.compensatePayment(ctx, payment)
		}
	}
}

func (s *TimeoutScanner) compensatePayment(ctx context.Context, payment *model.Payment) {
	slog.Info("Timing out stuck payment", "payment_id", payment.PaymentID, "state", payment.State, "updated_at", payment.UpdatedAt.Format(time.RFC3339))

	err := s.repo.ConditionalUpdateState(ctx, payment.PaymentID, payment.State, model.StateTimedOut)
	if err != nil {
		var condErr *ddbTypes.ConditionalCheckFailedException
		if errors.As(err, &condErr) {
			slog.Info("Payment state changed since scan, skipping", "payment_id", payment.PaymentID)
			return
		}
		slog.Error("Error updating payment to TIMED_OUT", "payment_id", payment.PaymentID, "error", err)
		return
	}

	// Publish ReleaseFunds compensation if funds were reserved
	if payment.State == model.StateFundsReserved || payment.State == model.StateProcessing {
		event := events.ReleaseFundsEvent{
			PaymentID: payment.PaymentID,
			Timestamp: time.Now().Format(time.RFC3339),
		}
		payload, _ := json.Marshal(event)
		if err := s.outbox.WriteMessage(ctx, events.EventReleaseFunds, string(payload)); err != nil {
			slog.Error("Failed to publish ReleaseFunds for timed-out payment", "payment_id", payment.PaymentID, "error", err)
		}
	}
}
