package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"gorm.io/gorm"

	"github.com/Cookit-labs/Backend/internal/models"
	"github.com/Cookit-labs/Backend/internal/scoring"
	"github.com/Cookit-labs/Backend/internal/validation"
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

	// Fetch reputation records for all agents in these proposals
	agentIDs := make([]string, len(proposals))
	for i, p := range proposals {
		agentIDs[i] = p.AgentID
	}

	var reputations []models.AgentReputation
	h.db.Where("agent_id IN ?", agentIDs).Find(&reputations)

	repMap := make(map[string]*models.AgentReputation, len(reputations))
	for i := range reputations {
		repMap[reputations[i].AgentID] = &reputations[i]
	}

	// Run deterministic scoring engine
	engine := scoring.New()
	result := engine.Score(proposals, repMap)

	if result.Winner == nil {
		respondError(w, http.StatusInternalServerError, "scoring engine returned no winner")
		return
	}

	// Persist each proposal's computed score
	for _, s := range result.AllScores {
		h.db.Model(&models.Proposal{}).Where("id = ?", s.ProposalID).Update("score", s.Total)
	}

	// Update intent with selected agent
	if err := h.db.Model(&models.Intent{}).Where("id = ?", intentID).Updates(map[string]any{
		"status":            "winner_selected",
		"selected_agent_id": result.Winner.AgentID,
	}).Error; err != nil {
		respondError(w, http.StatusInternalServerError, "failed to update intent")
		return
	}

	// Broadcast winner selection
	h.hub.Broadcast(intentID, "winner_selected", map[string]any{
		"proposal_id":              result.Winner.ProposalID,
		"agent_id":                 result.Winner.AgentID,
		"score":                    result.Winner.Total,
		"execution_efficiency":     result.Winner.ExecutionEfficiencyScore,
		"win_rate_score":           result.Winner.WinRateScore,
		"slippage_score":           result.Winner.SlippageScore,
		"reputation_score":         result.Winner.ReputationScore,
	})

	// Publish to Redis pub/sub
	ctx, cancel := context.WithTimeout(context.Background(), 5*1e9)
	defer cancel()
	_ = h.pubsub.Publish(ctx, intentID, "winner_selected", map[string]any{
		"proposal_id": result.Winner.ProposalID,
		"agent_id":    result.Winner.AgentID,
		"score":       result.Winner.Total,
	})

	respondJSON(w, http.StatusOK, map[string]any{
		"winner_proposal_id":       result.Winner.ProposalID,
		"agent_id":                 result.Winner.AgentID,
		"total_score":              result.Winner.Total,
		"execution_efficiency":     result.Winner.ExecutionEfficiencyScore,
		"win_rate_score":           result.Winner.WinRateScore,
		"slippage_score":           result.Winner.SlippageScore,
		"reputation_score":         result.Winner.ReputationScore,
		"all_scores":               result.AllScores,
	})
}

func (h *Handler) ValidateExecution(w http.ResponseWriter, r *http.Request) {
	intentID := chi.URLParam(r, "intentID")

	var req ValidateExecutionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if msg := validateStruct(&req); msg != "" {
		respondError(w, http.StatusBadRequest, msg)
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

	// Run full validation service
	svc := validation.New()
	result := svc.Validate(&intent, &proposal)

	if !result.Valid {
		// Record failed validation attempt against the agent
		h.db.Model(&models.Proposal{}).Where("id = ?", req.ProposalID).
			Update("score", 0)

		respondJSON(w, http.StatusUnprocessableEntity, map[string]any{
			"valid":  false,
			"errors": result.Errors,
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

	if msg := validateStruct(&req); msg != "" {
		respondError(w, http.StatusBadRequest, msg)
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

	// Re-run validation gate before any settlement
	svc := validation.New()
	if vr := svc.Validate(&intent, &proposal); !vr.Valid {
		respondJSON(w, http.StatusUnprocessableEntity, map[string]any{
			"error":  "settlement blocked: validation failed",
			"errors": vr.Errors,
		})
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

	// Pay agent execution fee via Circle if wallets are configured
	if proposal.AgentID != "" && req.ExecutionFeeUSDC != "" {
		// Fetch agent's Circle wallet address
		var agent models.Agent
		if err := h.db.First(&agent, "id = ?", proposal.AgentID).Error; err == nil && agent.WalletAddress != "" {
			idempotencyKey := "settle-" + execution.ID
			feeCtx, feeCancel := context.WithTimeout(context.Background(), 15*1e9)
			defer feeCancel()
			// Transfer fee from escrow (intent's wallet) to agent wallet
			// In V0 we use the agent's WalletAddress as the Circle wallet ID
			_, _ = h.circle.SendUSDC(feeCtx, idempotencyKey, intent.UserWallet, agent.WalletAddress, req.ExecutionFeeUSDC)
		}
	}

	// Update agent win stats and reputation
	h.db.Model(&models.Agent{}).Where("id = ?", proposal.AgentID).
		UpdateColumn("total_intents_won", gorm.Expr("total_intents_won + 1"))

	h.db.Model(&models.AgentReputation{}).Where("agent_id = ?", proposal.AgentID).Updates(map[string]any{
		"total_executions":      gorm.Expr("total_executions + 1"),
		"consecutive_successes": gorm.Expr("consecutive_successes + 1"),
	})

	// Broadcast settlement
	h.hub.Broadcast(intentID, "execution_settled", map[string]any{
		"execution_id": execution.ID,
		"agent_id":     execution.WinningAgentID,
		"slippage":     execution.ActualSlippage,
		"tx_hash":      execution.TxHash,
	})

	// Publish to Redis pub/sub
	ctx, cancel := context.WithTimeout(context.Background(), 5*1e9)
	defer cancel()
	_ = h.pubsub.Publish(ctx, intentID, "execution_settled", map[string]any{
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
