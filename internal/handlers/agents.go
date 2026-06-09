package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/Cookit-labs/Backend/internal/models"
)

func (h *Handler) GetLeaderboard(w http.ResponseWriter, r *http.Request) {
	var agents []models.Agent
	if err := h.db.Preload("Reputation").Find(&agents).Error; err != nil {
		respondError(w, http.StatusInternalServerError, "failed to fetch agents")
		return
	}

	entries := make([]LeaderboardEntry, 0)
	for _, agent := range agents {
		if agent.Reputation == nil {
			continue
		}

		entry := LeaderboardEntry{
			AgentID:              agent.ID,
			Name:                 agent.Name,
			StrategyType:         agent.StrategyType,
			TotalIntentsWon:      agent.TotalIntentsWon,
			WinRate:              agent.Reputation.WinRate,
			AvgSlippageDelivered: agent.Reputation.AvgSlippageDelivered,
			CompositeScore:       agent.Reputation.CompositeScore,
		}
		entries = append(entries, entry)
	}

	respondJSON(w, http.StatusOK, LeaderboardResponse{
		Agents: entries,
		Total:  int64(len(entries)),
	})
}

func (h *Handler) GetAgent(w http.ResponseWriter, r *http.Request) {
	agentID := chi.URLParam(r, "agentID")

	var agent models.Agent
	if err := h.db.Preload("Reputation").Preload("Proposals").First(&agent, "id = ?", agentID).Error; err != nil {
		respondError(w, http.StatusNotFound, "agent not found")
		return
	}

	// Count total executions for this agent
	var totalExecutions int64
	h.db.Model(&models.Execution{}).Where("winning_agent_id = ?", agentID).Count(&totalExecutions)

	rep := agent.Reputation
	if rep == nil {
		rep = &models.AgentReputation{}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"id":                  agent.ID,
		"name":                agent.Name,
		"strategy_type":       agent.StrategyType,
		"description":         agent.Description,
		"is_active":           agent.IsActive,
		"wallet_address":      agent.WalletAddress,
		"total_intents_won":   agent.TotalIntentsWon,
		"total_capital_usdc":  agent.TotalCapitalUsdc,
		"total_executions":    totalExecutions,
		"win_rate":            rep.WinRate,
		"avg_slippage":        rep.AvgSlippageDelivered,
		"composite_score":     rep.CompositeScore,
		"on_chain_score":      rep.OnChainReputationScore,
		"consecutive_success": rep.ConsecutiveSuccesses,
		"created_at":          agent.CreatedAt,
	})
}
