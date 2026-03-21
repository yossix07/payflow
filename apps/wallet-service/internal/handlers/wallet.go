package handlers

import (
	"encoding/json"
	"net/http"

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

	wallet, err := h.repo.GetWallet(r.Context(), userID)
	if err != nil {
		http.Error(w, "Failed to get wallet", http.StatusInternalServerError)
		return
	}

	// Update balance
	wallet.Balance += req.Amount

	if err := h.repo.UpdateWallet(r.Context(), wallet); err != nil {
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
