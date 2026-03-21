package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/google/uuid"
	"github.com/my-saas-platform/wallet-service/internal/events"
	"github.com/my-saas-platform/wallet-service/internal/repository"
	"github.com/my-saas-platform/wallet-service/pkg/outbox"
	"github.com/my-saas-platform/wallet-service/pkg/queue"
)

const walletUpdateMaxRetries = 3

type EventConsumer struct {
	queue  queue.QueueClient
	repo   repository.Repository
	outbox outbox.Outbox
}

func NewEventConsumer(queue queue.QueueClient, repo repository.Repository, outbox outbox.Outbox) *EventConsumer {
	return &EventConsumer{
		queue:  queue,
		repo:   repo,
		outbox: outbox,
	}
}

func (ec *EventConsumer) Start(ctx context.Context) {
	log.Println("Starting event consumer...")

	for {
		select {
		case <-ctx.Done():
			log.Println("Event consumer stopped")
			return
		default:
		}

		messages, err := ec.queue.ReceiveMessages(ctx)
		if err != nil {
			log.Printf("Error receiving messages: %v", err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(1 * time.Second):
			}
			continue
		}

		for _, msg := range messages {
			if err := ec.handleMessage(ctx, msg); err != nil {
				log.Printf("Error handling message: %v", err)
			} else {
				ec.queue.DeleteMessage(ctx, msg.ReceiptHandle)
			}
		}

		// Backoff if no messages received (SQS long-poll already waits up to 20s)
		if len(messages) == 0 {
			select {
			case <-ctx.Done():
				return
			case <-time.After(1 * time.Second):
			}
		}
	}
}

func (ec *EventConsumer) handleMessage(ctx context.Context, msg queue.Message) error {
	log.Printf("Received event: %s", msg.EventType)

	switch msg.EventType {
	case events.EventReserveFunds:
		var event events.ReserveFundsEvent
		if err := json.Unmarshal([]byte(msg.Body), &event); err != nil {
			return err
		}
		return ec.handleReserveFunds(ctx, event)

	case events.EventReleaseFunds:
		var event events.ReleaseFundsEvent
		if err := json.Unmarshal([]byte(msg.Body), &event); err != nil {
			return err
		}
		return ec.handleReleaseFunds(ctx, event)

	default:
		log.Printf("Ignoring event type: %s", msg.EventType)
	}

	return nil
}

func (ec *EventConsumer) handleReserveFunds(ctx context.Context, event events.ReserveFundsEvent) error {
	log.Printf("Reserving funds for payment %s: user %s, amount %.2f", event.PaymentID, event.UserID, event.Amount)

	// Create reservation once, idempotently (conditional put ignores duplicates)
	reservation := &repository.Reservation{
		ReservationID: uuid.NewSHA1(uuid.NameSpaceDNS, []byte(event.PaymentID)).String(),
		PaymentID:     event.PaymentID,
		UserID:        event.UserID,
		Amount:        event.Amount,
		Status:        "ACTIVE",
		CreatedAt:     time.Now(),
		ExpiresAt:     time.Now().Add(24 * time.Hour).Unix(),
	}
	if err := ec.repo.CreateReservation(ctx, reservation); err != nil {
		return err
	}

	// Retry loop only for the balance deduction (optimistic locking)
	for attempt := 0; attempt <= walletUpdateMaxRetries; attempt++ {
		wallet, err := ec.repo.GetWallet(ctx, event.UserID)
		if err != nil {
			return err
		}

		if wallet.Balance < event.Amount {
			log.Printf("Insufficient funds for user %s: balance %.2f, requested %.2f", event.UserID, wallet.Balance, event.Amount)
			return ec.publishInsufficientFunds(ctx, event.PaymentID, event.UserID)
		}

		// Deduct balance with optimistic lock
		wallet.Balance -= event.Amount
		wallet.UpdatedAt = time.Now()

		if err := ec.repo.UpdateWallet(ctx, wallet); err != nil {
			var condErr *types.ConditionalCheckFailedException
			if errors.As(err, &condErr) && attempt < walletUpdateMaxRetries {
				jitter := time.Duration(rand.Intn(50*(attempt+1))) * time.Millisecond
				time.Sleep(50*time.Millisecond + jitter)
				continue
			}
			return err
		}

		log.Printf("Funds reserved: reservation %s", reservation.ReservationID)
		return ec.publishFundsReserved(ctx, event.PaymentID)
	}

	return fmt.Errorf("optimistic lock failed after retries for payment %s", event.PaymentID)
}

func (ec *EventConsumer) handleReleaseFunds(ctx context.Context, event events.ReleaseFundsEvent) error {
	log.Printf("Releasing funds for payment %s", event.PaymentID)

	// Get reservation
	reservation, err := ec.repo.GetReservationByPayment(ctx, event.PaymentID)
	if err != nil {
		log.Printf("Reservation not found: %v", err)
		return nil // Already released or doesn't exist
	}

	if reservation.Status != "ACTIVE" {
		log.Printf("Reservation %s already released", reservation.ReservationID)
		return nil
	}

	for attempt := 0; attempt <= walletUpdateMaxRetries; attempt++ {
		wallet, err := ec.repo.GetWallet(ctx, reservation.UserID)
		if err != nil {
			return err
		}

		// Return balance with optimistic lock
		wallet.Balance += reservation.Amount
		wallet.UpdatedAt = time.Now()

		if err := ec.repo.UpdateWallet(ctx, wallet); err != nil {
			var condErr *types.ConditionalCheckFailedException
			if errors.As(err, &condErr) && attempt < walletUpdateMaxRetries {
				jitter := time.Duration(rand.Intn(50*(attempt+1))) * time.Millisecond
				time.Sleep(50*time.Millisecond + jitter)
				continue
			}
			return err
		}

		// Update reservation
		reservation.Status = "RELEASED"
		if err := ec.repo.UpdateReservation(ctx, reservation); err != nil {
			return err
		}

		log.Printf("Funds released: reservation %s", reservation.ReservationID)

		// Publish FundsReleased event
		return ec.publishFundsReleased(ctx, event.PaymentID)
	}

	return fmt.Errorf("optimistic lock failed after retries for payment %s", event.PaymentID)
}

func (ec *EventConsumer) publishFundsReserved(ctx context.Context, paymentID string) error {
	event := events.FundsReservedEvent{
		PaymentID: paymentID,
		Timestamp: time.Now().Format(time.RFC3339),
	}

	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}

	return ec.outbox.WriteMessage(ctx, events.EventFundsReserved, string(payload))
}

func (ec *EventConsumer) publishInsufficientFunds(ctx context.Context, paymentID, userID string) error {
	event := events.InsufficientFundsEvent{
		PaymentID: paymentID,
		UserID:    userID,
		Timestamp: time.Now().Format(time.RFC3339),
	}

	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}

	return ec.outbox.WriteMessage(ctx, events.EventInsufficientFunds, string(payload))
}

func (ec *EventConsumer) publishFundsReleased(ctx context.Context, paymentID string) error {
	event := events.FundsReleasedEvent{
		PaymentID: paymentID,
		Timestamp: time.Now().Format(time.RFC3339),
	}

	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}

	return ec.outbox.WriteMessage(ctx, events.EventFundsReleased, string(payload))
}
