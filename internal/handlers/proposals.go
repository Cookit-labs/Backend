package handlers

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/Cookit-labs/Backend/internal/models"
)

func (h *Handler) CreateProposal(w http.ResponseWriter, r *http.Request) {
	intentID := chi.URLParam(r, "intentID")

	// Verify intent exists
	var intent models.Intent
	if err := h.db.First(&intent, "id = ?", intentID).Error; err != nil {
		respondError(w, http.StatusNotFound, "intent not found")
		return
	}

	var req CreateProposalRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if msg := validateStruct(&req); msg != "" {
		respondError(w, http.StatusBadRequest, msg)
		return
	}

	// Verify agent exists
	var agent models.Agent
	if err := h.db.First(&agent, "id = ?", req.AgentID).Error; err != nil {
		respondError(w, http.StatusNotFound, "agent not found")
		return
	}

	proposal := &models.Proposal{
		ID:                        uuid.New().String(),
		IntentID:                  intentID,
		AgentID:                   req.AgentID,
		StrategyType:              req.StrategyType,
		TokenIn:                   req.TokenIn,
		TokenOut:                  req.TokenOut,
		ProjectedSlippage:         req.ProjectedSlippage,
		ProjectedExecutionQuality: req.ProjectedExecutionQuality,
		ProposedAmount:            req.ProposedAmount,
		ExecutionPath:             req.ExecutionPath,
	}

	if err := h.db.Create(proposal).Error; err != nil {
		respondError(w, http.StatusInternalServerError, "failed to create proposal")
		return
	}

	// Broadcast to WebSocket subscribers
	h.hub.Broadcast(intentID, "proposal_received", map[string]any{
		"proposal_id": proposal.ID,
		"agent_id":    proposal.AgentID,
		"strategy":    proposal.StrategyType,
		"slippage":    proposal.ProjectedSlippage,
		"quality":     proposal.ProjectedExecutionQuality,
	})

	// Publish to Redis pub/sub (multi-instance broadcast)
	ctx, cancel := context.WithTimeout(context.Background(), 5*1e9)
	defer cancel()
	_ = h.pubsub.Publish(ctx, intentID, "proposal_received", map[string]any{
		"proposal_id": proposal.ID,
		"agent_id":    proposal.AgentID,
		"strategy":    proposal.StrategyType,
		"slippage":    proposal.ProjectedSlippage,
		"quality":     proposal.ProjectedExecutionQuality,
	})

	respondJSON(w, http.StatusCreated, ProposalResponse{
		ID:                        proposal.ID,
		IntentID:                  proposal.IntentID,
		AgentID:                   proposal.AgentID,
		StrategyType:              proposal.StrategyType,
		ProjectedSlippage:         proposal.ProjectedSlippage,
		ProjectedExecutionQuality: proposal.ProjectedExecutionQuality,
		CreatedAt:                 proposal.CreatedAt,
	})
}

func (h *Handler) ListProposals(w http.ResponseWriter, r *http.Request) {
	intentID := chi.URLParam(r, "intentID")

	var proposals []models.Proposal
	if err := h.db.Where("intent_id = ?", intentID).Find(&proposals).Error; err != nil {
		respondError(w, http.StatusInternalServerError, "failed to fetch proposals")
		return
	}

	responses := make([]ProposalResponse, len(proposals))
	for i, p := range proposals {
		responses[i] = ProposalResponse{
			ID:                        p.ID,
			IntentID:                  p.IntentID,
			AgentID:                   p.AgentID,
			StrategyType:              p.StrategyType,
			ProjectedSlippage:         p.ProjectedSlippage,
			ProjectedExecutionQuality: p.ProjectedExecutionQuality,
			Score:                     p.Score,
			CreatedAt:                 p.CreatedAt,
		}
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"proposals": responses,
		"total":     len(proposals),
	})
}
