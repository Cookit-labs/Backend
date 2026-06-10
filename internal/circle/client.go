package circle

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	baseURLSandbox    = "https://api-sandbox.circle.com/v1"
	baseURLProduction = "https://api.circle.com/v1"
)

// Client is the Circle API client.
type Client struct {
	apiKey  string
	baseURL string
	http    *http.Client
}

func New(apiKey string, sandbox bool) *Client {
	base := baseURLProduction
	if sandbox {
		base = baseURLSandbox
	}
	return &Client{
		apiKey:  apiKey,
		baseURL: base,
		http:    &http.Client{Timeout: 30 * time.Second},
	}
}

// --- Wallet ---

type CreateWalletRequest struct {
	IdempotencyKey string `json:"idempotencyKey"`
	Description    string `json:"description,omitempty"`
}

type Wallet struct {
	WalletID    string `json:"walletId"`
	EntityID    string `json:"entityId"`
	Type        string `json:"type"`
	Description string `json:"description"`
	Balances    []Balance `json:"balances"`
}

type Balance struct {
	Amount   string `json:"amount"`
	Currency string `json:"currency"`
}

// CreateWallet provisions a new programmable wallet for a user.
func (c *Client) CreateWallet(ctx context.Context, idempotencyKey, description string) (*Wallet, error) {
	body := CreateWalletRequest{
		IdempotencyKey: idempotencyKey,
		Description:    description,
	}

	var result struct {
		Data Wallet `json:"data"`
	}

	if err := c.post(ctx, "/wallets", body, &result); err != nil {
		return nil, fmt.Errorf("circle CreateWallet: %w", err)
	}

	return &result.Data, nil
}

// GetWallet retrieves a wallet by ID.
func (c *Client) GetWallet(ctx context.Context, walletID string) (*Wallet, error) {
	var result struct {
		Data Wallet `json:"data"`
	}

	if err := c.get(ctx, fmt.Sprintf("/wallets/%s", walletID), &result); err != nil {
		return nil, fmt.Errorf("circle GetWallet: %w", err)
	}

	return &result.Data, nil
}

// GetBalance returns the USDC balance for a given wallet.
func (c *Client) GetBalance(ctx context.Context, walletID string) (string, error) {
	wallet, err := c.GetWallet(ctx, walletID)
	if err != nil {
		return "0", err
	}

	for _, b := range wallet.Balances {
		if b.Currency == "USDC" {
			return b.Amount, nil
		}
	}

	return "0", nil
}

// --- Transfers (micropayment settlement) ---

type CreateTransferRequest struct {
	IdempotencyKey string          `json:"idempotencyKey"`
	Source         TransferAccount `json:"source"`
	Destination    TransferAccount `json:"destination"`
	Amount         TransferAmount  `json:"amount"`
}

type TransferAccount struct {
	Type     string `json:"type"`
	ID       string `json:"id"`
	Chain    string `json:"chain,omitempty"`
	Address  string `json:"address,omitempty"`
}

type TransferAmount struct {
	Amount   string `json:"amount"`
	Currency string `json:"currency"`
}

type Transfer struct {
	ID          string         `json:"id"`
	Source      TransferAccount `json:"source"`
	Destination TransferAccount `json:"destination"`
	Amount      TransferAmount  `json:"amount"`
	Status      string         `json:"status"`
	CreateDate  string         `json:"createDate"`
}

// SendUSDC transfers USDC from one wallet to another (agent execution fee settlement).
func (c *Client) SendUSDC(ctx context.Context, idempotencyKey, fromWalletID, toWalletID, amount string) (*Transfer, error) {
	body := CreateTransferRequest{
		IdempotencyKey: idempotencyKey,
		Source: TransferAccount{
			Type: "wallet",
			ID:   fromWalletID,
		},
		Destination: TransferAccount{
			Type: "wallet",
			ID:   toWalletID,
		},
		Amount: TransferAmount{
			Amount:   amount,
			Currency: "USDC",
		},
	}

	var result struct {
		Data Transfer `json:"data"`
	}

	if err := c.post(ctx, "/transfers", body, &result); err != nil {
		return nil, fmt.Errorf("circle SendUSDC: %w", err)
	}

	return &result.Data, nil
}

// --- CCTP (Cross-Chain Transfer Protocol simulation) ---

type CCTPBurnRequest struct {
	IdempotencyKey      string `json:"idempotencyKey"`
	Amount              string `json:"amount"`
	SourceWalletID      string `json:"sourceWalletId"`
	DestinationChain    string `json:"destinationChain"`
	DestinationAddress  string `json:"destinationAddress"`
}

type CCTPTransfer struct {
	ID                 string `json:"id"`
	Amount             string `json:"amount"`
	Status             string `json:"status"`
	SourceChain        string `json:"sourceChain"`
	DestinationChain   string `json:"destinationChain"`
	DestinationAddress string `json:"destinationAddress"`
}

// SimulateCCTPTransfer simulates a cross-chain USDC transfer via CCTP.
// In V0 this is a demo endpoint — it calls Circle's CCTP burn-and-mint flow.
func (c *Client) SimulateCCTPTransfer(ctx context.Context, req CCTPBurnRequest) (*CCTPTransfer, error) {
	var result struct {
		Data CCTPTransfer `json:"data"`
	}

	if err := c.post(ctx, "/transfers/cctp", req, &result); err != nil {
		return nil, fmt.Errorf("circle SimulateCCTPTransfer: %w", err)
	}

	return &result.Data, nil
}

// --- HTTP helpers ---

func (c *Client) post(ctx context.Context, path string, body, dest interface{}) error {
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(data))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	return c.do(req, dest)
}

func (c *Client) get(ctx context.Context, path string, dest interface{}) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	return c.do(req, dest)
}

func (c *Client) do(req *http.Request, dest interface{}) error {
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode >= 400 {
		return fmt.Errorf("circle API error %d: %s", resp.StatusCode, string(b))
	}

	if dest != nil {
		return json.Unmarshal(b, dest)
	}

	return nil
}
