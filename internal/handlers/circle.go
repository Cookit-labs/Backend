package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/Cookit-labs/Backend/internal/circle"
)

// circleClient returns a Circle client from the handler config.
// In production, this should be injected; for now it reads from the handler's cfg field.
func (h *Handler) circleClient() *circle.Client {
	return h.circle
}

// POST /api/v1/wallets
func (h *Handler) CreateWallet(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserWallet  string `json:"user_wallet"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Use the user's wallet address as idempotency key so we never double-create
	idempotencyKey := uuid.NewSHA1(uuid.NameSpaceURL, []byte("wallet:"+req.UserWallet)).String()

	wallet, err := h.circleClient().CreateWallet(r.Context(), idempotencyKey, req.Description)
	if err != nil {
		respondError(w, http.StatusBadGateway, "failed to create Circle wallet: "+err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, wallet)
}

// GET /api/v1/wallets/{walletID}/balance
func (h *Handler) GetWalletBalance(w http.ResponseWriter, r *http.Request) {
	walletID := chi.URLParam(r, "walletID")

	balance, err := h.circleClient().GetBalance(r.Context(), walletID)
	if err != nil {
		respondError(w, http.StatusBadGateway, "failed to fetch balance: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"wallet_id": walletID,
		"amount":    balance,
		"currency":  "USDC",
	})
}

// POST /api/v1/wallets/transfer
func (h *Handler) TransferUSDC(w http.ResponseWriter, r *http.Request) {
	var req struct {
		FromWalletID string `json:"from_wallet_id"`
		ToWalletID   string `json:"to_wallet_id"`
		Amount       string `json:"amount"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	idempotencyKey := uuid.New().String()

	transfer, err := h.circleClient().SendUSDC(r.Context(), idempotencyKey, req.FromWalletID, req.ToWalletID, req.Amount)
	if err != nil {
		respondError(w, http.StatusBadGateway, "USDC transfer failed: "+err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, transfer)
}

// POST /api/v1/wallets/cctp — cross-chain routing demo
func (h *Handler) CCTPSimulate(w http.ResponseWriter, r *http.Request) {
	var req circle.CCTPBurnRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.IdempotencyKey == "" {
		req.IdempotencyKey = uuid.New().String()
	}

	transfer, err := h.circleClient().SimulateCCTPTransfer(r.Context(), req)
	if err != nil {
		respondError(w, http.StatusBadGateway, "CCTP simulation failed: "+err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, transfer)
}
