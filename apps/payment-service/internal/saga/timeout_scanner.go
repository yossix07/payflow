package saga

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"time"

	ddbTypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/my-saas-platform/payment-service/internal/events"
	"github.com/my-saas-platform/payment-service/internal/model"
	"github.com/my-saas-platform/payment-service/internal/repository"
	"github.com/my-saas-platform/payment-service/pkg/outbox"
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
	log.Println("Starting saga timeout scanner...")
	timer := time.NewTimer(s.scanInterval)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("Saga timeout scanner stopped")
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
			log.Printf("Error scanning for stuck %s payments: %v", state, err)
			continue
		}

		for _, payment := range payments {
			s.compensatePayment(ctx, payment)
		}
	}
}

func (s *TimeoutScanner) compensatePayment(ctx context.Context, payment *model.Payment) {
	log.Printf("Timing out stuck payment %s (state: %s, updated: %s)",
		payment.PaymentID, payment.State, payment.UpdatedAt.Format(time.RFC3339))

	err := s.repo.ConditionalUpdateState(ctx, payment.PaymentID, payment.State, model.StateTimedOut)
	if err != nil {
		var condErr *ddbTypes.ConditionalCheckFailedException
		if errors.As(err, &condErr) {
			log.Printf("Payment %s state changed since scan, skipping", payment.PaymentID)
			return
		}
		log.Printf("Error updating payment %s to TIMED_OUT: %v", payment.PaymentID, err)
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
			log.Printf("ERROR: Failed to publish ReleaseFunds for timed-out payment %s: %v", payment.PaymentID, err)
		}
	}
}
