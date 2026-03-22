package main

import (
	"context"
	"encoding/json"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/payflow/ledger-service/internal/handlers"
	"github.com/payflow/ledger-service/internal/repository"
	"github.com/payflow/ledger-service/pkg/queue"
)

type HealthResponse struct {
	Service   string    `json:"service"`
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
}

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	slog.Info("Starting Ledger Service...")

	queueURL := os.Getenv("QUEUE_URL")
	ledgerTable := os.Getenv("LEDGER_TABLE")
	awsRegion := os.Getenv("AWS_REGION")

	if queueURL == "" || ledgerTable == "" {
		log.Fatal("Missing required environment variables")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	queueClient, err := queue.NewSQSClient(ctx, awsRegion, queueURL)
	if err != nil {
		log.Fatalf("Failed to create SQS client: %v", err)
	}

	repo, err := repository.NewDynamoDBRepository(ctx, awsRegion, ledgerTable)
	if err != nil {
		log.Fatalf("Failed to create repository: %v", err)
	}

	ledgerHandler := handlers.NewLedgerHandler(repo)

	var wg sync.WaitGroup

	// Start event consumer
	eventConsumer := handlers.NewEventConsumer(queueClient, repo)
	wg.Add(1)
	go func() {
		defer wg.Done()
		eventConsumer.Start(ctx)
	}()

	r := mux.NewRouter()

	r.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(HealthResponse{
			Service:   "Ledger Service",
			Status:    "healthy",
			Timestamp: time.Now(),
		})
	}).Methods("GET")

	// Readiness check
	r.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-ctx.Done():
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(map[string]string{"status": "draining"})
			return
		default:
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ready"})
	}).Methods("GET")

	r.HandleFunc("/ledger", ledgerHandler.GetEntries).Methods("GET")
	r.HandleFunc("/ledger/payment/{payment_id}", ledgerHandler.GetEntriesByPayment).Methods("GET")

	srv := &http.Server{
		Addr:         ":8080",
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		slog.Info("Ledger Service listening on :8080")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("Shutting down server...")
	cancel()

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		slog.Info("Background workers stopped cleanly")
	case <-time.After(10 * time.Second):
		slog.Info("Timeout waiting for background workers")
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	slog.Info("Server exited")
}
