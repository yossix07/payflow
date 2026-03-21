package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/my-saas-platform/payment-service/internal/saga"
)

func TestCreatePayment_Returns201(t *testing.T) {
	repo := newMockRepository()
	ob := newMockOutbox()
	orch := saga.NewOrchestrator(repo, ob)
	handler := NewPaymentHandler(orch, repo)

	body, _ := json.Marshal(map[string]interface{}{"user_id": "user-1", "amount": 100.0})
	req := httptest.NewRequest("POST", "/payments", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "key-1")
	rr := httptest.NewRecorder()
	handler.CreatePayment(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d", rr.Code)
	}
	var resp map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp["payment_id"] == "" {
		t.Error("expected payment_id in response")
	}
}

func TestGetPayment_Returns404_WhenNotFound(t *testing.T) {
	repo := newMockRepository()
	ob := newMockOutbox()
	orch := saga.NewOrchestrator(repo, ob)
	handler := NewPaymentHandler(orch, repo)

	req := httptest.NewRequest("GET", "/payments/nonexistent", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "nonexistent"})
	rr := httptest.NewRecorder()
	handler.GetPayment(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rr.Code)
	}
}
