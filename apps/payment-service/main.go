package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/my-saas-platform/payment-service/internal/handlers"
	"github.com/my-saas-platform/payment-service/internal/repository"
	"github.com/my-saas-platform/payment-service/internal/saga"
	"github.com/my-saas-platform/payment-service/pkg/idempotency"
	"github.com/my-saas-platform/payment-service/pkg/outbox"
	"github.com/my-saas-platform/payment-service/pkg/queue"
)

type HealthResponse struct {
	Service   string    `json:"service"`
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
}

func main() {
	log.Println("Starting Payment Service...")

	// Environment variables
	queueURL := os.Getenv("QUEUE_URL")
	paymentsTable := os.Getenv("PAYMENTS_TABLE")
	idempotencyTable := os.Getenv("IDEMPOTENCY_TABLE")
	outboxTable := os.Getenv("OUTBOX_TABLE")
	awsRegion := os.Getenv("AWS_REGION")

	if queueURL == "" || paymentsTable == "" || idempotencyTable == "" || outboxTable == "" {
		log.Fatal("Missing required environment variables")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize AWS clients
	queueClient, err := queue.NewSQSClient(ctx, awsRegion, queueURL)
	if err != nil {
		log.Fatalf("Failed to create SQS client: %v", err)
	}

	repo, err := repository.NewDynamoDBRepository(ctx, awsRegion, paymentsTable, idempotencyTable)
	if err != nil {
		log.Fatalf("Failed to create repository: %v", err)
	}

	outboxRepo, err := outbox.NewDynamoDBOutbox(ctx, awsRegion, outboxTable)
	if err != nil {
		log.Fatalf("Failed to create outbox: %v", err)
	}

	// Initialize saga orchestrator
	sagaOrchestrator := saga.NewOrchestrator(repo, outboxRepo)

	// Initialize handlers
	paymentHandler := handlers.NewPaymentHandler(sagaOrchestrator, repo)
	idempotencyMW := idempotency.NewMiddleware(repo)

	// Create broadcast queue clients for outbox fan-out
	var broadcastQueues []queue.QueueClient
	broadcastURLs := os.Getenv("BROADCAST_QUEUE_URLS")
	if broadcastURLs != "" {
		for _, url := range strings.Split(broadcastURLs, ",") {
			url = strings.TrimSpace(url)
			if url == "" {
				continue
			}
			q, err := queue.NewSQSClient(ctx, awsRegion, url)
			if err != nil {
				log.Fatalf("Failed to create broadcast queue client for %s: %v", url, err)
			}
			broadcastQueues = append(broadcastQueues, q)
		}
	} else {
		// Fallback: broadcast to own queue only
		broadcastQueues = []queue.QueueClient{queueClient}
	}

	var wg sync.WaitGroup

	// Start outbox worker (background goroutine)
	outboxWorker := outbox.NewWorker(outboxRepo, broadcastQueues)
	wg.Add(1)
	go func() {
		defer wg.Done()
		outboxWorker.Start(ctx)
	}()

	// Start event consumer (background goroutine)
	eventConsumer := handlers.NewEventConsumer(queueClient, sagaOrchestrator)
	wg.Add(1)
	go func() {
		defer wg.Done()
		eventConsumer.Start(ctx)
	}()

	// Start timeout scanner (background goroutine)
	timeoutScanner := saga.NewTimeoutScanner(repo, outboxRepo)
	wg.Add(1)
	go func() {
		defer wg.Done()
		timeoutScanner.Start(ctx)
	}()

	// HTTP Router
	r := mux.NewRouter()

	// Health check
	r.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(HealthResponse{
			Service:   "Payment Service",
			Status:    "healthy",
			Timestamp: time.Now(),
		})
	}).Methods("GET")

	// Payment endpoints
	r.Handle("/payments", idempotencyMW.Wrap(http.HandlerFunc(paymentHandler.CreatePayment))).Methods("POST")
	r.HandleFunc("/payments/{id}", paymentHandler.GetPayment).Methods("GET")

	// HTTP Server
	srv := &http.Server{
		Addr:         ":8080",
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	go func() {
		log.Println("Payment Service listening on :8080")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")
	cancel()

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Println("Background workers stopped cleanly")
	case <-time.After(10 * time.Second):
		log.Println("Timeout waiting for background workers")
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited")
}
