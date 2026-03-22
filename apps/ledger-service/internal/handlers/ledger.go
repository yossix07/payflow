package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/payflow/ledger-service/internal/repository"
)

type LedgerHandler struct {
	repo repository.Repository
}

func NewLedgerHandler(repo repository.Repository) *LedgerHandler {
	return &LedgerHandler{repo: repo}
}

func (h *LedgerHandler) GetEntries(w http.ResponseWriter, r *http.Request) {
	limitStr := r.URL.Query().Get("limit")
	limit := 50
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil {
			limit = l
		}
	}

	entries, err := h.repo.GetEntries(r.Context(), limit)
	if err != nil {
		http.Error(w, "Failed to get entries", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entries)
}

func (h *LedgerHandler) GetEntriesByPayment(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	paymentID := vars["payment_id"]

	entries, err := h.repo.GetEntriesByPayment(r.Context(), paymentID)
	if err != nil {
		http.Error(w, "Failed to get entries", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entries)
}
