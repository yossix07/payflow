package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/payflow/ledger-service/internal/repository"
)

func TestGetEntries_ReturnsLedgerEntries(t *testing.T) {
	mock := newMockRepository()
	mock.entries = []*repository.LedgerEntry{
		{EntryID: "e1", PaymentID: "p1", EventType: "PaymentStarted", Amount: 100, UserID: "u1"},
		{EntryID: "e2", PaymentID: "p2", EventType: "FundsReserved", Amount: 200, UserID: "u2"},
	}

	handler := NewLedgerHandler(mock)

	req := httptest.NewRequest(http.MethodGet, "/ledger", nil)
	rec := httptest.NewRecorder()

	handler.GetEntries(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var entries []*repository.LedgerEntry
	if err := json.NewDecoder(rec.Body).Decode(&entries); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	if entries[0].EntryID != "e1" || entries[1].EntryID != "e2" {
		t.Errorf("unexpected entry IDs: %s, %s", entries[0].EntryID, entries[1].EntryID)
	}
}

func TestGetEntriesByPayment_FiltersCorrectly(t *testing.T) {
	mock := newMockRepository()
	mock.entries = []*repository.LedgerEntry{
		{EntryID: "e1", PaymentID: "p1", EventType: "PaymentStarted", Amount: 100, UserID: "u1"},
		{EntryID: "e2", PaymentID: "p2", EventType: "FundsReserved", Amount: 200, UserID: "u2"},
		{EntryID: "e3", PaymentID: "p1", EventType: "PaymentSucceeded", Amount: 100, UserID: "u1"},
	}

	handler := NewLedgerHandler(mock)

	req := httptest.NewRequest(http.MethodGet, "/ledger/payment/p1", nil)
	// Set mux vars so handler can extract payment_id
	req = mux.SetURLVars(req, map[string]string{"payment_id": "p1"})
	rec := httptest.NewRecorder()

	handler.GetEntriesByPayment(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var entries []*repository.LedgerEntry
	if err := json.NewDecoder(rec.Body).Decode(&entries); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(entries) != 2 {
		t.Fatalf("expected 2 entries for payment p1, got %d", len(entries))
	}

	for _, e := range entries {
		if e.PaymentID != "p1" {
			t.Errorf("expected payment_id p1, got %s", e.PaymentID)
		}
	}
}
