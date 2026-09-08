package chain

import "testing"

func TestParseDefaultsToArcWhenOmitted(t *testing.T) {
	// Clients written before multi-chain send no chain at all; they must keep
	// working rather than start failing validation.
	got, err := Parse("")
	if err != nil || got != Arc {
		t.Fatalf("Parse(\"\") = %q, %v; want arc, nil", got, err)
	}
}

func TestParseNormalises(t *testing.T) {
	for _, in := range []string{"STELLAR", " stellar ", "Stellar"} {
		got, err := Parse(in)
		if err != nil || got != Stellar {
			t.Errorf("Parse(%q) = %q, %v; want stellar, nil", in, got, err)
		}
	}
}

func TestParseRejectsUnknownChain(t *testing.T) {
	if _, err := Parse("ethereum"); err == nil {
		t.Fatal("expected an unsupported chain to be rejected")
	}
}

func TestFamilySeparatesEVMFromStellar(t *testing.T) {
	if Arc.Family() != FamilyEVM {
		t.Errorf("arc should be an EVM chain")
	}
	if Stellar.Family() != FamilyStellar {
		t.Errorf("stellar should not be an EVM chain")
	}
}

func TestValidateAddressAcceptsCorrectFormats(t *testing.T) {
	if err := ValidateAddress(Arc, "0x742d35Cc6634C0532925a3b844Bc9e7595f0bEb0"); err != nil {
		t.Errorf("expected a valid EVM address to pass: %v", err)
	}
	// Circle's testnet USDC issuer — a real, well-formed Stellar key.
	if err := ValidateAddress(Stellar, "GBBD47IF6LWK7P7MDEVSCWR7DPUWV3NY3DTQEVFL4NAT4AQH3ZLLFLA5"); err != nil {
		t.Errorf("expected a valid Stellar address to pass: %v", err)
	}
}

// The mismatch this whole change exists to catch.
func TestValidateAddressRejectsWrongChainFormat(t *testing.T) {
	if err := ValidateAddress(Stellar, "0x742d35Cc6634C0532925a3b844Bc9e7595f0bEb0"); err == nil {
		t.Error("expected an EVM address on a Stellar intent to be rejected")
	}
	if err := ValidateAddress(Arc, "GBBD47IF6LWK7P7MDEVSCWR7DPUWV3NY3DTQEVFL4NAT4AQH3ZLLFLA5"); err == nil {
		t.Error("expected a Stellar address on an Arc intent to be rejected")
	}
}

func TestValidateAddressRejectsMalformed(t *testing.T) {
	cases := []struct {
		chain   Chain
		address string
		why     string
	}{
		{Arc, "", "empty"},
		{Arc, "0x123", "too short"},
		{Arc, "742d35Cc6634C0532925a3b844Bc9e7595f0bEb0", "missing 0x"},
		{Arc, "0xZZZZ35Cc6634C0532925a3b844Bc9e7595f0bEb0", "non-hex"},
		{Stellar, "G123", "too short"},
		{Stellar, "gbbd47if6lwk7p7mdevscwr7dpuwv3ny3dtqevfl4nat4aqh3zllfla5", "lowercase"},
	}
	for _, c := range cases {
		if err := ValidateAddress(c.chain, c.address); err == nil {
			t.Errorf("expected %s address to be rejected (%s)", c.chain, c.why)
		}
	}
}
