package models

import (
	"time"

	"gorm.io/gorm"
)

// Intent represents a user's trading intention
type Intent struct {
	ID string `gorm:"primaryKey" json:"id"`
	// Which network this intent settles on. Indexed because settlement and the
	// orchestrator both filter by it.
	Chain           string         `gorm:"index;default:arc" json:"chain"`
	UserWallet      string         `gorm:"index" json:"user_wallet"`
	TokenIn         string         `json:"token_in"`
	TokenOut        string         `json:"token_out"`
	AmountIn        string         `json:"amount_in"`    // stored as string to preserve precision
	MaxSlippage     float64        `json:"max_slippage"` // e.g., 0.01 = 1%
	Deadline        time.Time      `json:"deadline"`
	Status          string         `json:"status"` // pending, competition_open, winner_selected, executing, settled, failed
	SelectedAgentID string         `json:"selected_agent_id"`
	Metadata        []byte         `gorm:"type:jsonb" json:"metadata"` // arbitrary user data
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
	DeletedAt       gorm.DeletedAt `gorm:"index" json:"-"`

	// Relationships
	Proposals []Proposal `gorm:"foreignKey:IntentID;references:ID" json:"proposals,omitempty"`
	Execution *Execution `gorm:"foreignKey:IntentID;references:ID" json:"execution,omitempty"`
}

// Proposal represents an agent's bid to execute an intent
type Proposal struct {
	ID           string `gorm:"primaryKey" json:"id"`
	IntentID     string `gorm:"index" json:"intent_id"`
	AgentID      string `gorm:"index" json:"agent_id"`
	StrategyType string `json:"strategy_type"` // TWAP, Momentum, Shadow
	// The pair the agent intends to execute. Recorded on the proposal rather
	// than inferred from the intent, so validation can catch an agent that
	// proposes a different pair than the one the user asked for.
	TokenIn                   string         `json:"token_in"`
	TokenOut                  string         `json:"token_out"`
	ProjectedSlippage         float64        `json:"projected_slippage"`          // e.g., 0.005 = 0.5%
	ProjectedExecutionQuality float64        `json:"projected_execution_quality"` // 0-1
	ProposedAmount            string         `json:"proposed_amount"`
	ExecutionPath             string         `json:"execution_path"` // e.g., "uniswap_v3" or "curve"
	Score                     float64        `json:"score"`          // computed by scoring engine
	CreatedAt                 time.Time      `json:"created_at"`
	UpdatedAt                 time.Time      `json:"updated_at"`
	DeletedAt                 gorm.DeletedAt `gorm:"index" json:"-"`

	// Relationships
	Intent *Intent `gorm:"foreignKey:IntentID;references:ID" json:"-"`
	Agent  *Agent  `gorm:"foreignKey:AgentID;references:ID" json:"-"`
}

// Execution represents the settled outcome of an intent
type Execution struct {
	ID             string `gorm:"primaryKey" json:"id"`
	IntentID       string `gorm:"uniqueIndex" json:"intent_id"`
	WinningAgentID string `gorm:"index" json:"winning_agent_id"`
	ProposalID     string `json:"proposal_id"`
	ExecutedAmount string `json:"executed_amount"`
	// What the winning agent promised, copied from the proposal at settlement.
	// Kept on the execution rather than read back through the proposal so the
	// comparison survives the proposal being edited or soft-deleted.
	ProjectedSlippage float64 `json:"projected_slippage"`
	ActualSlippage    float64 `json:"actual_slippage"`
	// Actual minus projected. Positive means the agent delivered worse than it
	// promised. Stored rather than computed on read so it can be aggregated and
	// indexed without recomputing across every execution.
	SlippageDelta    float64        `gorm:"index" json:"slippage_delta"`
	TxHash           string         `json:"tx_hash"`
	SettlementStatus string         `json:"settlement_status"`  // pending, confirmed, failed
	ExecutionFeeUSDC string         `json:"execution_fee_usdc"` // agent fee
	ArcContractAddr  string         `json:"arc_contract_addr"`
	ExecutedAt       time.Time      `json:"executed_at"`
	SettledAt        *time.Time     `json:"settled_at"`
	FailureReason    string         `json:"failure_reason"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
	DeletedAt        gorm.DeletedAt `gorm:"index" json:"-"`

	// Relationships
	Intent *Intent `gorm:"foreignKey:IntentID;references:ID" json:"-"`
	Agent  *Agent  `gorm:"foreignKey:WinningAgentID;references:ID" json:"-"`
}

// Agent represents an autonomous execution agent
type Agent struct {
	ID               string         `gorm:"primaryKey" json:"id"`
	Name             string         `json:"name"`
	StrategyType     string         `json:"strategy_type"` // TWAP, Momentum, Shadow
	Description      string         `json:"description"`
	WalletAddress    string         `json:"wallet_address"`
	IsActive         bool           `json:"is_active"`
	TotalIntentsWon  int64          `json:"total_intents_won"`
	TotalCapitalUsdc string         `json:"total_capital_usdc"` // lifetime volume
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
	DeletedAt        gorm.DeletedAt `gorm:"index" json:"-"`

	// Relationships
	Reputation *AgentReputation `gorm:"foreignKey:AgentID;references:ID" json:"reputation,omitempty"`
	Proposals  []Proposal       `gorm:"foreignKey:AgentID;references:ID" json:"proposals,omitempty"`
}

// AgentReputation tracks agent performance metrics on-chain seed values
type AgentReputation struct {
	ID                   string  `gorm:"primaryKey" json:"id"`
	AgentID              string  `gorm:"uniqueIndex" json:"agent_id"`
	WinRate              float64 `json:"win_rate"` // wins / total_proposals
	AvgSlippageDelivered float64 `json:"avg_slippage_delivered"`
	// Mean of (actual - projected) across settled executions. Positive means
	// the agent habitually delivers worse than it promises, which win rate and
	// average slippage alone cannot reveal — an agent can win often, on
	// optimistic projections it never meets.
	AvgSlippageDelta       float64        `json:"avg_slippage_delta"`
	TotalExecutions        int64          `json:"total_executions"`
	ConsecutiveSuccesses   int64          `json:"consecutive_successes"`
	CompositeScore         float64        `json:"composite_score"`           // weighted avg of metrics
	OnChainReputationScore string         `json:"on_chain_reputation_score"` // synced to Arc L1
	LastSyncedAt           *time.Time     `json:"last_synced_at"`
	CreatedAt              time.Time      `json:"created_at"`
	UpdatedAt              time.Time      `json:"updated_at"`
	DeletedAt              gorm.DeletedAt `gorm:"index" json:"-"`

	// Relationships
	Agent *Agent `gorm:"foreignKey:AgentID;references:ID" json:"-"`
}
