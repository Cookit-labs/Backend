package handlers

import (
	"strings"
	"testing"
)

func TestValidateStructAcceptsAWellFormedProposal(t *testing.T) {
	req := CreateProposalRequest{
		AgentID:                   "agent_1",
		StrategyType:              "twap",
		TokenIn:                   "USDC",
		TokenOut:                  "WETH",
		ProjectedSlippage:         0.005,
		ProjectedExecutionQuality: 0.9,
		ProposedAmount:            "1000",
		ExecutionPath:             "uniswap_v3",
	}
	if msg := validateStruct(&req); msg != "" {
		t.Fatalf("expected valid, got: %s", msg)
	}
}

// Before this change the tags were inert: an empty payload was persisted.
func TestValidateStructRejectsAnEmptyProposal(t *testing.T) {
	msg := validateStruct(&CreateProposalRequest{})
	if msg == "" {
		t.Fatal("expected an empty proposal to be rejected")
	}
	for _, field := range []string{"agent_id", "strategy_type", "token_in", "token_out"} {
		if !strings.Contains(msg, field) {
			t.Errorf("expected %q to be reported, got: %s", field, msg)
		}
	}
}

func TestValidateStructReportsFieldsByTheirJSONNames(t *testing.T) {
	// An API consumer sent "agent_id"; telling them "AgentID" is unhelpful.
	msg := validateStruct(&CreateProposalRequest{})
	if strings.Contains(msg, "AgentID") {
		t.Errorf("expected snake_case field names, got: %s", msg)
	}
}

func TestValidateStructAllowsZeroSlippage(t *testing.T) {
	// `required` on a float rejects 0, but zero projected slippage is a
	// legitimate claim — hence gte=0 rather than required.
	req := CreateProposalRequest{
		AgentID:           "agent_1",
		StrategyType:      "twap",
		TokenIn:           "USDC",
		TokenOut:          "WETH",
		ProjectedSlippage: 0,
		ProposedAmount:    "1000",
		ExecutionPath:     "uniswap_v3",
	}
	if msg := validateStruct(&req); msg != "" {
		t.Fatalf("expected zero slippage to be accepted, got: %s", msg)
	}
}

func TestValidateStructRejectsOutOfRangeQuality(t *testing.T) {
	req := CreateProposalRequest{
		AgentID:                   "agent_1",
		StrategyType:              "twap",
		TokenIn:                   "USDC",
		TokenOut:                  "WETH",
		ProjectedExecutionQuality: 5, // capped at 1
		ProposedAmount:            "1000",
		ExecutionPath:             "uniswap_v3",
	}
	if msg := validateStruct(&req); msg == "" {
		t.Fatal("expected an out-of-range quality to be rejected")
	}
}

func TestValidateStructReportsEveryFailureTogether(t *testing.T) {
	msg := validateStruct(&CreateProposalRequest{})
	if strings.Count(msg, ";") < 3 {
		t.Fatalf("expected several failures in one message, got: %s", msg)
	}
}

func TestToSnakeKeepsAcronymsIntact(t *testing.T) {
	// A naive implementation renders AgentID as agent_i_d.
	cases := map[string]string{
		"AgentID":                   "agent_id",
		"TokenIn":                   "token_in",
		"ProjectedSlippage":         "projected_slippage",
		"ProjectedExecutionQuality": "projected_execution_quality",
		"IDToken":                   "id_token",
		"UserWallet":                "user_wallet",
	}
	for in, want := range cases {
		if got := toSnake(in); got != want {
			t.Errorf("toSnake(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCreateAgentRequestRequiresIdentity(t *testing.T) {
	msg := validateStruct(&CreateAgentRequest{})
	if msg == "" {
		t.Fatal("expected an empty agent to be rejected")
	}
	for _, field := range []string{"name", "strategy_type", "wallet_address"} {
		if !strings.Contains(msg, field) {
			t.Errorf("expected %q to be reported, got: %s", field, msg)
		}
	}
}

func TestCreateAgentRequestRejectsUnknownStrategy(t *testing.T) {
	// A free-text strategy would be stored and then match nothing downstream.
	req := CreateAgentRequest{
		Name:          "Rogue",
		StrategyType:  "not_a_strategy",
		WalletAddress: "0xabc",
	}
	if msg := validateStruct(&req); msg == "" {
		t.Fatal("expected an unknown strategy to be rejected")
	}
}

func TestCreateAgentRequestAcceptsKnownStrategies(t *testing.T) {
	for _, s := range []string{"twap", "momentum", "shadow", "arbitrage", "custom"} {
		req := CreateAgentRequest{Name: "A", StrategyType: s, WalletAddress: "0xabc"}
		if msg := validateStruct(&req); msg != "" {
			t.Errorf("expected %q to be accepted, got: %s", s, msg)
		}
	}
}

func TestUpdateAgentStatusDistinguishesFalseFromOmitted(t *testing.T) {
	// The reason IsActive is a pointer: with a plain bool, "deactivate" and
	// "leave unchanged" arrive as the same JSON.
	deactivate := false
	if msg := validateStruct(&UpdateAgentStatusRequest{IsActive: &deactivate}); msg != "" {
		t.Fatalf("expected an explicit false to be accepted, got: %s", msg)
	}
	if msg := validateStruct(&UpdateAgentStatusRequest{}); msg == "" {
		t.Fatal("expected an omitted is_active to be rejected")
	}
}
