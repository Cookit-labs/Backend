package validation

import (
	"slices"
	"fmt"
	"strings"
	"time"

	"github.com/Cookit-labs/Backend/internal/models"
)

// ApprovedVenues is the list of permitted execution venues.
// The AI agent must have set a venue in this list on their proposal.
var ApprovedVenues = []string{
	"uniswap_v3",
	"uniswap_v2",
	"curve",
	"balancer",
	"1inch",
	"arc_dex",
}

// ValidationError represents a single failed constraint.
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// Result is the outcome of a full validation run.
type Result struct {
	Valid  bool              `json:"valid"`
	Errors []ValidationError `json:"errors,omitempty"`
}

// Service runs all pre-settlement constraint checks.
type Service struct{}

func New() *Service {
	return &Service{}
}

// Validate checks a proposal against an intent before any USDC moves.
// All checks run regardless — caller receives the full list of failures.
func (s *Service) Validate(intent *models.Intent, proposal *models.Proposal) Result {
	var errs []ValidationError

	// 1. Token pair matches
	if !tokenPairMatches(intent, proposal) {
		errs = append(errs, ValidationError{
			Field:   "token_pair",
			Message: fmt.Sprintf("proposal token pair does not match intent (want %s→%s)", intent.TokenIn, intent.TokenOut),
		})
	}

	// 2. Slippage within tolerance
	if proposal.ProjectedSlippage > intent.MaxSlippage {
		errs = append(errs, ValidationError{
			Field: "slippage",
			Message: fmt.Sprintf(
				"projected slippage %.4f%% exceeds max tolerance %.4f%%",
				proposal.ProjectedSlippage*100,
				intent.MaxSlippage*100,
			),
		})
	}

	// 3. Deadline not passed
	if time.Now().After(intent.Deadline) {
		errs = append(errs, ValidationError{
			Field:   "deadline",
			Message: fmt.Sprintf("intent deadline passed at %s", intent.Deadline.UTC().Format(time.RFC3339)),
		})
	}

	// 4. Execution venue is approved
	if !venueApproved(proposal.ExecutionPath) {
		errs = append(errs, ValidationError{
			Field:   "venue",
			Message: fmt.Sprintf("venue %q is not on the approved list: %s", proposal.ExecutionPath, strings.Join(ApprovedVenues, ", ")),
		})
	}

	// 5. Proposal belongs to this intent
	if proposal.IntentID != intent.ID {
		errs = append(errs, ValidationError{
			Field:   "intent_id",
			Message: "proposal does not belong to this intent",
		})
	}

	// 6. Agent on the proposal is active (non-empty agent ID)
	if proposal.AgentID == "" {
		errs = append(errs, ValidationError{
			Field:   "agent_id",
			Message: "proposal has no assigned agent",
		})
	}

	return Result{
		Valid:  len(errs) == 0,
		Errors: errs,
	}
}

func tokenPairMatches(intent *models.Intent, _ *models.Proposal) bool {
	return intent.TokenIn != "" && intent.TokenOut != "" && intent.TokenIn != intent.TokenOut
}

func venueApproved(venue string) bool {
	v := strings.ToLower(strings.TrimSpace(venue))
	return slices.Contains(ApprovedVenues, v)
}
