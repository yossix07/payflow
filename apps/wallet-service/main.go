package main

import (
	"context"
	"encoding/json"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/payflow/wallet-service/internal/handlers"
	"github.com/payflow/wallet-service/internal/repository"
	"github.com/payflow/wallet-service/pkg/outbox"
	"github.com/payflow/wallet-service/pkg/queue"
)

type HealthResponse struct {
	Service   string    `json:"service"`
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
}

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	slog.Info("Starting Wallet Service...")

	// Environment variables
	queueURL := os.Getenv("QUEUE_URL")
	walletsTable := os.Getenv("WALLETS_TABLE")
	reservationsTable := os.Getenv("RESERVATIONS_TABLE")
	outboxTable := os.Getenv("OUTBOX_TABLE")
	awsRegion := os.Getenv("AWS_REGION")

	if queueURL == "" || walletsTable == "" || reservationsTable == "" || outboxTable == "" {
		log.Fatal("Missing required environment variables")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize AWS clients
	queueClient, err := queue.NewSQSClient(ctx, awsRegion, queueURL)
	if err != nil {
		log.Fatalf("Failed to create SQS client: %v", err)
	}

	repo, err := repository.NewDynamoDBRepository(ctx, awsRegion, walletsTable, reservationsTable)
	if err != nil {
		log.Fatalf("Failed to create repository: %v", err)
	}

	outboxRepo, err := outbox.NewDynamoDBOutbox(ctx, awsRegion, outboxTable)
	if err != nil {
		log.Fatalf("Failed to create outbox: %v", err)
	}

	// Initialize handlers
	walletHandler := handlers.NewWalletHandler(repo, outboxRepo)

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
		broadcastQueues = []queue.QueueClient{queueClient}
	}

	var wg sync.WaitGroup

	// Start outbox worker
	outboxWorker := outbox.NewWorker(outboxRepo, broadcastQueues)
	wg.Add(1)
	go func() {
		defer wg.Done()
		outboxWorker.Start(ctx)
	}()

	// Start event consumer
	eventConsumer := handlers.NewEventConsumer(queueClient, repo, outboxRepo)
	wg.Add(1)
	go func() {
		defer wg.Done()
		eventConsumer.Start(ctx)
	}()

	// HTTP Router
	r := mux.NewRouter()

	// Health check
	r.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(HealthResponse{
			Service:   "Wallet Service",
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

	// Wallet endpoints
	r.HandleFunc("/wallets/{user_id}", walletHandler.GetWallet).Methods("GET")
	r.HandleFunc("/wallets/{user_id}/credit", walletHandler.CreditWallet).Methods("POST")

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
		slog.Info("Wallet Service listening on :8080")
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
