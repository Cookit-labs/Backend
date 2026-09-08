package validation

import (
	"testing"
	"time"

	"github.com/Cookit-labs/Backend/internal/models"
)

func baseIntent() *models.Intent {
	return &models.Intent{
		ID:          "intent_1",
		TokenIn:     "USDC",
		TokenOut:    "WETH",
		MaxSlippage: 0.01,
		Deadline:    time.Now().Add(time.Hour),
	}
}

func baseProposal() *models.Proposal {
	return &models.Proposal{
		IntentID:          "intent_1",
		AgentID:           "agent_1",
		TokenIn:           "USDC",
		TokenOut:          "WETH",
		ProjectedSlippage: 0.005,
		ExecutionPath:     "uniswap_v3",
	}
}

func hasError(r Result, field string) bool {
	for _, e := range r.Errors {
		if e.Field == field {
			return true
		}
	}
	return false
}

func TestValidateAcceptsMatchingProposal(t *testing.T) {
	got := New().Validate(baseIntent(), baseProposal())
	if !got.Valid {
		t.Fatalf("expected valid, got errors: %v", got.Errors)
	}
}

// The regression this whole change exists for: a proposal executing a
// completely different pair used to validate clean and go to settlement.
func TestValidateRejectsMismatchedTokenPair(t *testing.T) {
	p := baseProposal()
	p.TokenIn = "USDT"
	p.TokenOut = "WBTC"

	got := New().Validate(baseIntent(), p)
	if got.Valid {
		t.Fatal("expected a mismatched pair to be rejected")
	}
	if !hasError(got, "token_pair") {
		t.Fatalf("expected a token_pair error, got: %v", got.Errors)
	}
}

func TestValidateRejectsReversedPair(t *testing.T) {
	// Selling WETH for USDC is not the same intent as buying WETH with USDC.
	p := baseProposal()
	p.TokenIn, p.TokenOut = p.TokenOut, p.TokenIn

	if New().Validate(baseIntent(), p).Valid {
		t.Fatal("expected a reversed pair to be rejected")
	}
}

func TestValidateRejectsEmptyPairOnProposal(t *testing.T) {
	// Omitting the fields must not be a way back to the old always-pass path.
	p := baseProposal()
	p.TokenIn = ""
	p.TokenOut = ""

	if New().Validate(baseIntent(), p).Valid {
		t.Fatal("expected an empty proposal pair to be rejected")
	}
}

func TestValidateIgnoresCaseAndSurroundingSpace(t *testing.T) {
	p := baseProposal()
	p.TokenIn = " usdc "
	p.TokenOut = "weth"

	if got := New().Validate(baseIntent(), p); !got.Valid {
		t.Fatalf("expected formatting differences to be tolerated, got: %v", got.Errors)
	}
}

func TestValidateRejectsExcessiveSlippage(t *testing.T) {
	p := baseProposal()
	p.ProjectedSlippage = 0.05 // 5% against a 1% tolerance

	got := New().Validate(baseIntent(), p)
	if got.Valid || !hasError(got, "slippage") {
		t.Fatalf("expected a slippage error, got: %+v", got)
	}
}

func TestValidateRejectsPassedDeadline(t *testing.T) {
	i := baseIntent()
	i.Deadline = time.Now().Add(-time.Minute)

	got := New().Validate(i, baseProposal())
	if got.Valid || !hasError(got, "deadline") {
		t.Fatalf("expected a deadline error, got: %+v", got)
	}
}

func TestValidateRejectsUnapprovedVenue(t *testing.T) {
	p := baseProposal()
	p.ExecutionPath = "some_unlisted_dex"

	got := New().Validate(baseIntent(), p)
	if got.Valid || !hasError(got, "venue") {
		t.Fatalf("expected a venue error, got: %+v", got)
	}
}

func TestValidateRejectsProposalForAnotherIntent(t *testing.T) {
	p := baseProposal()
	p.IntentID = "intent_999"

	got := New().Validate(baseIntent(), p)
	if got.Valid || !hasError(got, "intent_id") {
		t.Fatalf("expected an intent_id error, got: %+v", got)
	}
}

func TestValidateReportsEveryFailureAtOnce(t *testing.T) {
	// Callers fix malformed proposals faster when they see the whole list.
	p := baseProposal()
	p.TokenIn = "USDT"
	p.ProjectedSlippage = 0.9
	p.ExecutionPath = "nope"
	p.AgentID = ""

	got := New().Validate(baseIntent(), p)
	if len(got.Errors) < 4 {
		t.Fatalf("expected at least 4 errors, got %d: %v", len(got.Errors), got.Errors)
	}
}
