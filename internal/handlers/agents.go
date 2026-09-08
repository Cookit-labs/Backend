package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/Cookit-labs/Backend/internal/models"
)

// CreateAgent registers an agent so it can submit proposals.
func (h *Handler) CreateAgent(w http.ResponseWriter, r *http.Request) {
	var req CreateAgentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if msg := validateStruct(&req); msg != "" {
		respondError(w, http.StatusBadRequest, msg)
		return
	}

	// A wallet identifies an agent on chain, so two agents sharing one would
	// make settlement ambiguous.
	wallet := strings.TrimSpace(req.WalletAddress)
	var existing models.Agent
	err := h.db.First(&existing, "wallet_address = ?", wallet).Error
	if err == nil {
		respondError(w, http.StatusConflict, "an agent with that wallet address already exists")
		return
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		respondError(w, http.StatusInternalServerError, "failed to check existing agents")
		return
	}

	agent := &models.Agent{
		ID:            uuid.New().String(),
		Name:          strings.TrimSpace(req.Name),
		StrategyType:  strings.ToLower(strings.TrimSpace(req.StrategyType)),
		Description:   req.Description,
		WalletAddress: wallet,
		IsActive:      true,
	}

	if err := h.db.Create(agent).Error; err != nil {
		respondError(w, http.StatusInternalServerError, "failed to create agent")
		return
	}

	// Reputation is created alongside the agent rather than lazily, so the
	// leaderboard — which skips agents with no reputation row — shows a new
	// agent immediately instead of hiding it until its first settlement.
	reputation := &models.AgentReputation{
		// The primary key has no database-side default, so omitting it inserts
		// an empty string and the second agent collides with the first.
		ID:      uuid.New().String(),
		AgentID: agent.ID,
	}
	if err := h.db.Create(reputation).Error; err != nil {
		respondError(w, http.StatusInternalServerError, "failed to initialise agent reputation")
		return
	}
	agent.Reputation = reputation

	respondJSON(w, http.StatusCreated, agent)
}

// ListAgents returns every registered agent.
func (h *Handler) ListAgents(w http.ResponseWriter, r *http.Request) {
	query := h.db.Preload("Reputation")

	// ?active=true is how the orchestrator asks for agents it may actually
	// invite into a competition.
	if r.URL.Query().Get("active") == "true" {
		query = query.Where("is_active = ?", true)
	}

	var agents []models.Agent
	if err := query.Find(&agents).Error; err != nil {
		respondError(w, http.StatusInternalServerError, "failed to fetch agents")
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"agents": agents,
		"total":  len(agents),
	})
}

// UpdateAgentStatus activates or deactivates an agent.
//
// Deactivation rather than deletion: proposals and executions reference an
// agent, and removing the row would orphan settled history.
func (h *Handler) UpdateAgentStatus(w http.ResponseWriter, r *http.Request) {
	agentID := chi.URLParam(r, "agentID")

	var req UpdateAgentStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if msg := validateStruct(&req); msg != "" {
		respondError(w, http.StatusBadRequest, msg)
		return
	}

	var agent models.Agent
	if err := h.db.First(&agent, "id = ?", agentID).Error; err != nil {
		respondError(w, http.StatusNotFound, "agent not found")
		return
	}

	agent.IsActive = *req.IsActive
	if err := h.db.Save(&agent).Error; err != nil {
		respondError(w, http.StatusInternalServerError, "failed to update agent")
		return
	}

	respondJSON(w, http.StatusOK, agent)
}

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
			AvgSlippageDelta:     agent.Reputation.AvgSlippageDelta,
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
		"id":                 agent.ID,
		"name":               agent.Name,
		"strategy_type":      agent.StrategyType,
		"description":        agent.Description,
		"is_active":          agent.IsActive,
		"wallet_address":     agent.WalletAddress,
		"total_intents_won":  agent.TotalIntentsWon,
		"total_capital_usdc": agent.TotalCapitalUsdc,
		"total_executions":   totalExecutions,
		"win_rate":           rep.WinRate,
		"avg_slippage":       rep.AvgSlippageDelivered,
		// Positive means the agent habitually delivers worse than it promises.
		// Exposed alongside win rate because an agent can win often on
		// optimistic projections it never meets.
		"avg_slippage_delta":  rep.AvgSlippageDelta,
		"composite_score":     rep.CompositeScore,
		"on_chain_score":      rep.OnChainReputationScore,
		"consecutive_success": rep.ConsecutiveSuccesses,
		"created_at":          agent.CreatedAt,
	})
}
