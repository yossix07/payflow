package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/gorilla/mux"
	"github.com/my-saas-platform/wallet-service/internal/repository"
	"github.com/my-saas-platform/wallet-service/pkg/outbox"
)

type WalletHandler struct {
	repo   repository.Repository
	outbox outbox.Outbox
}

func NewWalletHandler(repo repository.Repository, outbox outbox.Outbox) *WalletHandler {
	return &WalletHandler{
		repo:   repo,
		outbox: outbox,
	}
}

type WalletResponse struct {
	UserID  string  `json:"user_id"`
	Balance float64 `json:"balance"`
}

func (h *WalletHandler) GetWallet(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userID := vars["user_id"]

	wallet, err := h.repo.GetWallet(r.Context(), userID)
	if err != nil {
		http.Error(w, "Failed to get wallet", http.StatusInternalServerError)
		return
	}

	response := WalletResponse{
		UserID:  wallet.UserID,
		Balance: wallet.Balance,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

type CreditRequest struct {
	Amount float64 `json:"amount"`
}

// updateWalletWithRetry wraps a read-modify-write cycle with optimistic locking retry.
func updateWalletWithRetry(ctx context.Context, repo repository.Repository, userID string, modifyFn func(*repository.Wallet) error) (*repository.Wallet, error) {
	const maxRetries = 3
	for attempt := 0; attempt <= maxRetries; attempt++ {
		wallet, err := repo.GetWallet(ctx, userID)
		if err != nil {
			return nil, err
		}

		if err := modifyFn(wallet); err != nil {
			return nil, err
		}

		wallet.UpdatedAt = time.Now()
		if err := repo.UpdateWallet(ctx, wallet); err != nil {
			var condErr *types.ConditionalCheckFailedException
			if errors.As(err, &condErr) && attempt < maxRetries {
				jitter := time.Duration(rand.Intn(50*(attempt+1))) * time.Millisecond
				time.Sleep(50*time.Millisecond + jitter)
				continue
			}
			return nil, err
		}

		return wallet, nil
	}
	return nil, fmt.Errorf("optimistic lock failed after %d retries for user %s", maxRetries, userID)
}

func (h *WalletHandler) CreditWallet(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userID := vars["user_id"]

	var req CreditRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Amount <= 0 {
		http.Error(w, "Amount must be positive", http.StatusBadRequest)
		return
	}

	wallet, err := updateWalletWithRetry(r.Context(), h.repo, userID, func(w *repository.Wallet) error {
		w.Balance += req.Amount
		return nil
	})
	if err != nil {
		http.Error(w, "Failed to update wallet", http.StatusInternalServerError)
		return
	}

	response := WalletResponse{
		UserID:  wallet.UserID,
		Balance: wallet.Balance,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
