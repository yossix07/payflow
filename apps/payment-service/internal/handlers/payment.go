package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/my-saas-platform/payment-service/internal/model"
	"github.com/my-saas-platform/payment-service/internal/repository"
	"github.com/my-saas-platform/payment-service/internal/saga"
)

type PaymentHandler struct {
	orchestrator *saga.Orchestrator
	repo         repository.Repository
}

func NewPaymentHandler(orchestrator *saga.Orchestrator, repo repository.Repository) *PaymentHandler {
	return &PaymentHandler{
		orchestrator: orchestrator,
		repo:         repo,
	}
}

func (h *PaymentHandler) CreatePayment(w http.ResponseWriter, r *http.Request) {
	var req model.CreatePaymentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validation
	if req.UserID == "" || req.Amount <= 0 {
		http.Error(w, "Invalid user_id or amount", http.StatusBadRequest)
		return
	}

	// The idempotency key is set by the middleware and stored in context
	idempotencyKey := r.Header.Get("Idempotency-Key")

	// Start payment saga
	payment, err := h.orchestrator.StartPayment(r.Context(), req, idempotencyKey)
	if err != nil {
		slog.Error("Error creating payment", "error", err)
		http.Error(w, "Failed to create payment", http.StatusInternalServerError)
		return
	}

	response := model.PaymentResponse{
		PaymentID: payment.PaymentID,
		UserID:    payment.UserID,
		Amount:    payment.Amount,
		State:     payment.State,
		CreatedAt: payment.CreatedAt,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

func (h *PaymentHandler) GetPayment(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	paymentID := vars["id"]

	payment, err := h.repo.GetPayment(r.Context(), paymentID)
	if err != nil {
		http.Error(w, "Payment not found", http.StatusNotFound)
		return
	}

	response := model.PaymentResponse{
		PaymentID: payment.PaymentID,
		UserID:    payment.UserID,
		Amount:    payment.Amount,
		State:     payment.State,
		CreatedAt: payment.CreatedAt,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
