package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/Cookit-labs/Backend/internal/chain"
	"github.com/Cookit-labs/Backend/internal/models"
)

func (h *Handler) CreateIntent(w http.ResponseWriter, r *http.Request) {
	var req CreateIntentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if msg := validateStruct(&req); msg != "" {
		respondError(w, http.StatusBadRequest, msg)
		return
	}

	target, err := chain.Parse(req.Chain)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Checked here rather than at settlement: an Arc address on a Stellar
	// intent is unrecoverable once funds are in flight, and the two formats are
	// unmistakable, so there is no reason to accept the mismatch.
	if err := chain.ValidateAddress(target, req.UserWallet); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	intent := &models.Intent{
		ID:          uuid.New().String(),
		Chain:       target.String(),
		UserWallet:  req.UserWallet,
		TokenIn:     req.TokenIn,
		TokenOut:    req.TokenOut,
		AmountIn:    req.AmountIn,
		MaxSlippage: req.MaxSlippage,
		Deadline:    req.Deadline,
		Status:      "pending",
		Metadata:    req.Metadata,
	}

	if err := h.db.Create(intent).Error; err != nil {
		respondError(w, http.StatusInternalServerError, "failed to create intent")
		return
	}

	// Broadcast to WebSocket subscribers
	h.hub.Broadcast(intent.ID, "intent_created", map[string]string{
		"intent_id": intent.ID,
		"status":    intent.Status,
	})

	respondJSON(w, http.StatusCreated, IntentResponse{
		ID:          intent.ID,
		Chain:       intent.Chain,
		UserWallet:  intent.UserWallet,
		TokenIn:     intent.TokenIn,
		TokenOut:    intent.TokenOut,
		AmountIn:    intent.AmountIn,
		MaxSlippage: intent.MaxSlippage,
		Deadline:    intent.Deadline,
		Status:      intent.Status,
		CreatedAt:   intent.CreatedAt,
		UpdatedAt:   intent.UpdatedAt,
	})
}

func (h *Handler) GetIntent(w http.ResponseWriter, r *http.Request) {
	intentID := chi.URLParam(r, "intentID")

	var intent models.Intent
	if err := h.db.Preload("Proposals").Preload("Execution").First(&intent, "id = ?", intentID).Error; err != nil {
		respondError(w, http.StatusNotFound, "intent not found")
		return
	}

	respondJSON(w, http.StatusOK, IntentResponse{
		ID:              intent.ID,
		Chain:           intent.Chain,
		UserWallet:      intent.UserWallet,
		TokenIn:         intent.TokenIn,
		TokenOut:        intent.TokenOut,
		AmountIn:        intent.AmountIn,
		MaxSlippage:     intent.MaxSlippage,
		Deadline:        intent.Deadline,
		Status:          intent.Status,
		SelectedAgentID: intent.SelectedAgentID,
		CreatedAt:       intent.CreatedAt,
		UpdatedAt:       intent.UpdatedAt,
	})
}
