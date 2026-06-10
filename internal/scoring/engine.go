package scoring

import (
	"github.com/Cookit-labs/Backend/internal/models"
)

// Weights for the four scoring criteria. Must sum to 1.0.
const (
	weightExecutionEfficiency = 0.35 // projected execution quality
	weightWinRate             = 0.25 // agent historical win rate
	weightAvgSlippage         = 0.25 // agent average slippage delivered (lower = better)
	weightReputationScore     = 0.15 // composite on-chain reputation score
)

// Score holds a proposal's breakdown and final computed score.
type Score struct {
	ProposalID              string
	AgentID                 string
	ExecutionEfficiencyScore float64
	WinRateScore            float64
	SlippageScore           float64
	ReputationScore         float64
	Total                   float64
}

// Result is the output of the scoring engine for a given intent.
type Result struct {
	Winner    *Score
	AllScores []Score
}

// Engine scores proposals and picks a deterministic winner.
type Engine struct{}

func New() *Engine {
	return &Engine{}
}

// Score evaluates all proposals for an intent and returns the winner.
// Inputs: the proposals and their agents' current reputation records.
// Output is deterministic — same inputs always produce the same winner.
func (e *Engine) Score(proposals []models.Proposal, reputations map[string]*models.AgentReputation) Result {
	scores := make([]Score, 0, len(proposals))

	for _, p := range proposals {
		rep := reputations[p.AgentID]

		s := Score{
			ProposalID: p.ID,
			AgentID:    p.AgentID,
		}

		// 1. Execution efficiency (0–1, higher = better, directly from proposal)
		s.ExecutionEfficiencyScore = clamp(p.ProjectedExecutionQuality, 0, 1)

		// 2. Win rate (0–1, higher = better)
		if rep != nil {
			s.WinRateScore = clamp(rep.WinRate, 0, 1)
		}

		// 3. Slippage score (lower delivered slippage = higher score)
		// Normalise: assume max acceptable slippage is 5% (0.05)
		// score = 1 - (avgSlippage / 0.05), clamped to [0,1]
		if rep != nil {
			s.SlippageScore = clamp(1-(rep.AvgSlippageDelivered/0.05), 0, 1)
		} else {
			// No history: neutral score for new agents
			s.SlippageScore = 0.5
		}

		// 4. Reputation score (0–1, higher = better)
		if rep != nil {
			s.ReputationScore = clamp(rep.CompositeScore, 0, 1)
		} else {
			// New agent: seeded neutral value
			s.ReputationScore = 0.5
		}

		// Weighted total
		s.Total = (s.ExecutionEfficiencyScore * weightExecutionEfficiency) +
			(s.WinRateScore * weightWinRate) +
			(s.SlippageScore * weightAvgSlippage) +
			(s.ReputationScore * weightReputationScore)

		scores = append(scores, s)
	}

	winner := pickWinner(scores)

	return Result{
		Winner:    winner,
		AllScores: scores,
	}
}

// pickWinner returns the highest-scoring proposal.
// Tie-break: earlier proposal wins (stable, deterministic).
func pickWinner(scores []Score) *Score {
	if len(scores) == 0 {
		return nil
	}
	best := &scores[0]
	for i := 1; i < len(scores); i++ {
		if scores[i].Total > best.Total {
			best = &scores[i]
		}
	}
	return best
}

func clamp(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
