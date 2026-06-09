package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/Cookit-labs/Backend/internal/models"
)

func (h *Handler) SelectWinner(w http.ResponseWriter, r *http.Request) {
	intentID := chi.URLParam(r, "intentID")

	// Fetch all proposals for this intent
	var proposals []models.Proposal
	if err := h.db.Where("intent_id = ?", intentID).Find(&proposals).Error; err != nil {
		respondError(w, http.StatusInternalServerError, "failed to fetch proposals")
		return
	}

	if len(proposals) == 0 {
		respondError(w, http.StatusBadRequest, "no proposals to select from")
		return
	}

	// Simple scoring: select highest scored proposal (or first for now)
	// TODO: Replace with full scoring engine (Issue #6)
	winner := proposals[0]
	for _, p := range proposals {
		if p.Score > winner.Score {
			winner = p
		}
	}

	// Update intent with selected agent
	if err := h.db.Model(&models.Intent{}).Where("id = ?", intentID).Updates(map[string]any{
		"status":            "winner_selected",
		"selected_agent_id": winner.AgentID,
	}).Error; err != nil {
		respondError(w, http.StatusInternalServerError, "failed to update intent")
		return
	}

	// Broadcast winner selection
	h.hub.Broadcast(intentID, "winner_selected", map[string]any{
		"proposal_id": winner.ID,
		"agent_id":    winner.AgentID,
		"score":       winner.Score,
	})

	respondJSON(w, http.StatusOK, map[string]any{
		"winner_id": winner.ID,
		"agent_id":  winner.AgentID,
		"score":     winner.Score,
	})
}

func (h *Handler) ValidateExecution(w http.ResponseWriter, r *http.Request) {
	intentID := chi.URLParam(r, "intentID")

	var req ValidateExecutionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Fetch intent and proposal
	var intent models.Intent
	if err := h.db.First(&intent, "id = ?", intentID).Error; err != nil {
		respondError(w, http.StatusNotFound, "intent not found")
		return
	}

	var proposal models.Proposal
	if err := h.db.First(&proposal, "id = ?", req.ProposalID).Error; err != nil {
		respondError(w, http.StatusNotFound, "proposal not found")
		return
	}

	// Validation checks (simplified for now)
	// TODO: Full validation service (Issue #7)
	validationErrors := []string{}

	if time.Now().After(intent.Deadline) {
		validationErrors = append(validationErrors, "intent deadline has passed")
	}

	if proposal.ProjectedSlippage > intent.MaxSlippage {
		validationErrors = append(validationErrors, "projected slippage exceeds max tolerance")
	}

	if len(validationErrors) > 0 {
		respondJSON(w, http.StatusBadRequest, map[string]any{
			"valid":  false,
			"errors": validationErrors,
		})
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"valid":   true,
		"message": "execution validation passed",
	})
}

func (h *Handler) SettleExecution(w http.ResponseWriter, r *http.Request) {
	intentID := chi.URLParam(r, "intentID")

	var req SettleExecutionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Fetch intent and proposal
	var intent models.Intent
	if err := h.db.First(&intent, "id = ?", intentID).Error; err != nil {
		respondError(w, http.StatusNotFound, "intent not found")
		return
	}

	var proposal models.Proposal
	if err := h.db.First(&proposal, "id = ?", req.ProposalID).Error; err != nil {
		respondError(w, http.StatusNotFound, "proposal not found")
		return
	}

	// Create execution record
	now := time.Now()
	execution := &models.Execution{
		ID:               uuid.New().String(),
		IntentID:         intentID,
		WinningAgentID:   proposal.AgentID,
		ProposalID:       req.ProposalID,
		ActualSlippage:   req.ActualSlippage,
		TxHash:           req.TxHash,
		SettlementStatus: "confirmed",
		ExecutionFeeUSDC: req.ExecutionFeeUSDC,
		ExecutedAt:       now,
		SettledAt:        &now,
	}

	if err := h.db.Create(execution).Error; err != nil {
		respondError(w, http.StatusInternalServerError, "failed to create execution")
		return
	}

	// Update intent status
	if err := h.db.Model(&models.Intent{}).Where("id = ?", intentID).Update("status", "settled").Error; err != nil {
		respondError(w, http.StatusInternalServerError, "failed to update intent")
		return
	}

	// Update agent stats
	// TODO: Update reputation (Issue #6, #8)

	// Broadcast settlement
	h.hub.Broadcast(intentID, "execution_settled", map[string]any{
		"execution_id": execution.ID,
		"agent_id":     execution.WinningAgentID,
		"slippage":     execution.ActualSlippage,
		"tx_hash":      execution.TxHash,
	})

	respondJSON(w, http.StatusOK, map[string]any{
		"execution_id":    execution.ID,
		"status":          execution.SettlementStatus,
		"actual_slippage": execution.ActualSlippage,
		"tx_hash":         execution.TxHash,
	})
}
