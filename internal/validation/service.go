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
			Message: fmt.Sprintf(
				"proposal token pair %s→%s does not match intent %s→%s",
				proposal.TokenIn, proposal.TokenOut, intent.TokenIn, intent.TokenOut,
			),
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

// tokenPairMatches reports whether the proposal executes the pair the user
// actually asked for.
//
// This previously ignored the proposal entirely and checked only that the
// intent's own fields were non-empty and different from each other, which no
// proposal could ever fail. A proposal swapping an unrelated pair validated
// clean and went to settlement.
//
// Comparison is case-insensitive and trims surrounding space: agents submit
// symbols as free text, and "usdc" naming the same asset as "USDC" is a
// formatting difference, not a mismatched pair.
func tokenPairMatches(intent *models.Intent, proposal *models.Proposal) bool {
	if intent.TokenIn == "" || intent.TokenOut == "" {
		return false
	}
	// An empty pair on the proposal is a mismatch, not a pass. Treating it as
	// acceptable would restore the original bug for any agent that simply
	// omits the fields.
	if proposal.TokenIn == "" || proposal.TokenOut == "" {
		return false
	}
	return symbolsEqual(intent.TokenIn, proposal.TokenIn) &&
		symbolsEqual(intent.TokenOut, proposal.TokenOut)
}

func symbolsEqual(a, b string) bool {
	return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b))
}

func venueApproved(venue string) bool {
	v := strings.ToLower(strings.TrimSpace(venue))
	return slices.Contains(ApprovedVenues, v)
}
