package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/my-saas-platform/ledger-service/internal/handlers"
	"github.com/my-saas-platform/ledger-service/internal/repository"
	"github.com/my-saas-platform/ledger-service/pkg/queue"
)

type HealthResponse struct {
	Service   string    `json:"service"`
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
}

func main() {
	log.Println("Starting Ledger Service...")

	queueURL := os.Getenv("QUEUE_URL")
	ledgerTable := os.Getenv("LEDGER_TABLE")
	awsRegion := os.Getenv("AWS_REGION")

	if queueURL == "" || ledgerTable == "" {
		log.Fatal("Missing required environment variables")
	}

	ctx := context.Background()

	queueClient, err := queue.NewSQSClient(ctx, awsRegion, queueURL)
	if err != nil {
		log.Fatalf("Failed to create SQS client: %v", err)
	}

	repo, err := repository.NewDynamoDBRepository(ctx, awsRegion, ledgerTable)
	if err != nil {
		log.Fatalf("Failed to create repository: %v", err)
	}

	ledgerHandler := handlers.NewLedgerHandler(repo)

	// Start event consumer
	eventConsumer := handlers.NewEventConsumer(queueClient, repo)
	go eventConsumer.Start(ctx)

	r := mux.NewRouter()

	r.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(HealthResponse{
			Service:   "Ledger Service",
			Status:    "healthy",
			Timestamp: time.Now(),
		})
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
		log.Println("Ledger Service listening on :8080")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited")
}
