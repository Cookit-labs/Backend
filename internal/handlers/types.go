package handlers

import "time"

// Intent requests/responses

type CreateIntentRequest struct {
	// Optional: omitting it defaults to Arc, so clients written before
	// multi-chain keep working. The wallet is validated against whichever
	// chain applies.
	Chain       string    `json:"chain,omitempty" validate:"omitempty,oneof=arc stellar"`
	UserWallet  string    `json:"user_wallet" validate:"required"`
	TokenIn     string    `json:"token_in" validate:"required"`
	TokenOut    string    `json:"token_out" validate:"required"`
	AmountIn    string    `json:"amount_in" validate:"required"`
	MaxSlippage float64   `json:"max_slippage" validate:"required,gt=0"`
	Deadline    time.Time `json:"deadline" validate:"required"`
	Metadata    []byte    `json:"metadata,omitempty"`
}

type IntentResponse struct {
	ID              string    `json:"id"`
	Chain           string    `json:"chain"`
	UserWallet      string    `json:"user_wallet"`
	TokenIn         string    `json:"token_in"`
	TokenOut        string    `json:"token_out"`
	AmountIn        string    `json:"amount_in"`
	MaxSlippage     float64   `json:"max_slippage"`
	Deadline        time.Time `json:"deadline"`
	Status          string    `json:"status"`
	SelectedAgentID string    `json:"selected_agent_id,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// Proposal requests/responses

type CreateProposalRequest struct {
	AgentID      string `json:"agent_id" validate:"required"`
	StrategyType string `json:"strategy_type" validate:"required"`
	// The pair the agent will execute. Validated against the intent before
	// settlement; omitting it is a validation failure, not a default.
	TokenIn  string `json:"token_in" validate:"required"`
	TokenOut string `json:"token_out" validate:"required"`
	// gte=0 rather than required: required rejects the zero value, and zero
	// projected slippage is a legitimate claim.
	ProjectedSlippage         float64 `json:"projected_slippage" validate:"gte=0,lte=1"`
	ProjectedExecutionQuality float64 `json:"projected_execution_quality" validate:"gte=0,lte=1"`
	ProposedAmount            string  `json:"proposed_amount" validate:"required"`
	ExecutionPath             string  `json:"execution_path" validate:"required"`
}

type ProposalResponse struct {
	ID                        string    `json:"id"`
	IntentID                  string    `json:"intent_id"`
	AgentID                   string    `json:"agent_id"`
	StrategyType              string    `json:"strategy_type"`
	ProjectedSlippage         float64   `json:"projected_slippage"`
	ProjectedExecutionQuality float64   `json:"projected_execution_quality"`
	Score                     float64   `json:"score"`
	CreatedAt                 time.Time `json:"created_at"`
}

// Execution/Settlement

type ValidateExecutionRequest struct {
	ProposalID string `json:"proposal_id" validate:"required"`
}

type SettleExecutionRequest struct {
	ProposalID       string  `json:"proposal_id" validate:"required"`
	ActualSlippage   float64 `json:"actual_slippage" validate:"required,gte=0"`
	TxHash           string  `json:"tx_hash" validate:"required"`
	ExecutionFeeUSDC string  `json:"execution_fee_usdc" validate:"required"`
}

// Leaderboard

type LeaderboardEntry struct {
	AgentID              string  `json:"agent_id"`
	Name                 string  `json:"name"`
	StrategyType         string  `json:"strategy_type"`
	TotalIntentsWon      int64   `json:"total_intents_won"`
	WinRate              float64 `json:"win_rate"`
	AvgSlippageDelivered float64 `json:"avg_slippage_delivered"`
	AvgSlippageDelta     float64 `json:"avg_slippage_delta"`
	CompositeScore       float64 `json:"composite_score"`
}

type LeaderboardResponse struct {
	Agents []LeaderboardEntry `json:"agents"`
	Total  int64              `json:"total"`
}

// Error response

type ErrorResponse struct {
	Error  string `json:"error"`
	Status int    `json:"status"`
}

// Agent requests

// CreateAgentRequest registers an agent so it can submit proposals.
//
// Agents were previously read-only: proposals require an existing agent row,
// but nothing could create one, so a fresh database could not accept a single
// proposal.
type CreateAgentRequest struct {
	Name string `json:"name" validate:"required"`
	// Constrained to the strategies the scoring engine and the dApp both know
	// about. A free-text value would be accepted here and then silently fail to
	// match anything downstream.
	StrategyType  string `json:"strategy_type" validate:"required,oneof=twap momentum shadow arbitrage custom"`
	Description   string `json:"description"`
	WalletAddress string `json:"wallet_address" validate:"required"`
}

// UpdateAgentStatusRequest activates or deactivates an agent.
//
// Uses a pointer so that `false` is distinguishable from an omitted field —
// with a plain bool, "deactivate this agent" and "do not change the status"
// are the same JSON.
type UpdateAgentStatusRequest struct {
	IsActive *bool `json:"is_active" validate:"required"`
}
