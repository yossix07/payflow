package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/my-saas-platform/wallet-service/internal/repository"
)

func TestGetWallet_ReturnsBalance(t *testing.T) {
	repo := newMockRepository()
	ob := newMockOutbox()
	handler := NewWalletHandler(repo, ob)

	// Seed wallet
	repo.wallets["user-10"] = &repository.Wallet{
		UserID:  "user-10",
		Balance: 250.0,
	}

	req := httptest.NewRequest(http.MethodGet, "/wallets/user-10", nil)
	req = mux.SetURLVars(req, map[string]string{"user_id": "user-10"})
	rr := httptest.NewRecorder()

	handler.GetWallet(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	var resp WalletResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.UserID != "user-10" {
		t.Errorf("expected user_id user-10, got %s", resp.UserID)
	}
	if resp.Balance != 250.0 {
		t.Errorf("expected balance 250, got %.2f", resp.Balance)
	}
}

func TestCreditWallet_IncreasesBalance(t *testing.T) {
	repo := newMockRepository()
	ob := newMockOutbox()
	handler := NewWalletHandler(repo, ob)

	// Seed wallet with $100
	repo.wallets["user-20"] = &repository.Wallet{
		UserID:  "user-20",
		Balance: 100.0,
	}

	body, _ := json.Marshal(CreditRequest{Amount: 50.0})
	req := httptest.NewRequest(http.MethodPost, "/wallets/user-20/credit", bytes.NewReader(body))
	req = mux.SetURLVars(req, map[string]string{"user_id": "user-20"})
	rr := httptest.NewRecorder()

	handler.CreditWallet(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	var resp WalletResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Balance != 150.0 {
		t.Errorf("expected balance 150, got %.2f", resp.Balance)
	}

	// Verify the wallet was actually updated in the repository
	wallet := repo.wallets["user-20"]
	if wallet.Balance != 150.0 {
		t.Errorf("expected repo balance 150, got %.2f", wallet.Balance)
	}
}
