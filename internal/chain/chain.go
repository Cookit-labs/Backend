// Package chain describes the networks an intent can settle on.
//
// Intents used to carry no chain at all. The dApp now routes per chain and
// connects wallets on both Arc and Stellar, so without a discriminator an
// intent submitted from the Stellar side is indistinguishable from an Arc one
// once it reaches this service — and would settle against the wrong contracts.
package chain

import (
	"fmt"
	"regexp"
	"strings"
)

// Chain is the network an intent settles on.
type Chain string

const (
	Arc     Chain = "arc"
	Stellar Chain = "stellar"
)

// Family is the execution environment. Code that must branch between the two
// worlds branches on this rather than on the chain name, so adding another EVM
// chain does not mean revisiting every switch.
type Family string

const (
	FamilyEVM     Family = "evm"
	FamilyStellar Family = "stellar"
)

var families = map[Chain]Family{
	Arc:     FamilyEVM,
	Stellar: FamilyStellar,
}

// Supported lists every chain an intent may name, in display order.
var Supported = []Chain{Arc, Stellar}

// Default is used when a request omits the chain, so existing clients that
// predate multi-chain keep working rather than failing validation.
const Default = Arc

func (c Chain) Valid() bool {
	_, ok := families[c]
	return ok
}

func (c Chain) Family() Family {
	return families[c]
}

func (c Chain) String() string { return string(c) }

// Parse normalises and validates a chain name from a request.
func Parse(s string) (Chain, error) {
	trimmed := strings.ToLower(strings.TrimSpace(s))
	if trimmed == "" {
		return Default, nil
	}
	c := Chain(trimmed)
	if !c.Valid() {
		return "", fmt.Errorf("unsupported chain %q (want one of: %s)", s, JoinSupported())
	}
	return c, nil
}

func JoinSupported() string {
	names := make([]string, 0, len(Supported))
	for _, c := range Supported {
		names = append(names, string(c))
	}
	return strings.Join(names, ", ")
}

// EVM addresses are 20 hex bytes; Stellar public keys are 56 base32 characters
// beginning with G. The two are unmistakable, which is what makes it worth
// rejecting a mismatch at the API boundary rather than at settlement.
var (
	evmAddress     = regexp.MustCompile(`^0x[0-9a-fA-F]{40}$`)
	stellarAddress = regexp.MustCompile(`^G[A-Z2-7]{55}$`)
)

// ValidateAddress reports whether an address belongs to the given chain.
//
// A single address column that accepts both formats without checking them
// against the chain will happily store a mismatched pair, and the failure then
// surfaces at settlement — the most expensive place to find it.
func ValidateAddress(c Chain, address string) error {
	addr := strings.TrimSpace(address)
	if addr == "" {
		return fmt.Errorf("address is required")
	}

	switch c.Family() {
	case FamilyEVM:
		if !evmAddress.MatchString(addr) {
			return fmt.Errorf("%q is not a valid %s address (want 0x followed by 40 hex characters)", addr, c)
		}
	case FamilyStellar:
		// Stellar encodes in uppercase base32; a lowercase key is malformed
		// rather than merely differently spelled.
		if !stellarAddress.MatchString(addr) {
			return fmt.Errorf("%q is not a valid %s address (want G followed by 55 base32 characters)", addr, c)
		}
	default:
		return fmt.Errorf("unsupported chain %q", c)
	}
	return nil
}
