package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

// SolanaClient handles communication with Solana RPC
type SolanaClient struct {
	httpClient *http.Client
	rpcURL     string
}

// SolanaRPCRequest represents a JSON-RPC request to Solana
type SolanaRPCRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int         `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params"`
}

// SolanaRPCResponse represents a JSON-RPC response from Solana
type SolanaRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *SolanaRPCError `json:"error,omitempty"`
}

// SolanaRPCError represents an error in JSON-RPC response
type SolanaRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// SolanaBalanceResult represents balance query result
type SolanaBalanceResult struct {
	Context struct {
		Slot int64 `json:"slot"`
	} `json:"context"`
	Value int64 `json:"value"`
}

// SolanaTokenAccountResult represents token account information
type SolanaTokenAccountResult struct {
	Context struct {
		Slot int64 `json:"slot"`
	} `json:"context"`
	Value []SolanaTokenAccount `json:"value"`
}

// SolanaTokenAccount represents a token account
type SolanaTokenAccount struct {
	Account struct {
		Data       SolanaTokenAccountData `json:"data"`
		Executable bool                   `json:"executable"`
		Lamports   int64                  `json:"lamports"`
		Owner      string                 `json:"owner"`
		RentEpoch  int64                  `json:"rentEpoch"`
	} `json:"account"`
	Pubkey string `json:"pubkey"`
}

// SolanaTokenAccountData represents token account data
type SolanaTokenAccountData struct {
	Parsed struct {
		Info struct {
			IsNative    bool   `json:"isNative"`
			Mint        string `json:"mint"`
			Owner       string `json:"owner"`
			State       string `json:"state"`
			TokenAmount struct {
				Amount         string  `json:"amount"`
				Decimals       int     `json:"decimals"`
				UIAmount       float64 `json:"uiAmount"`
				UIAmountString string  `json:"uiAmountString"`
			} `json:"tokenAmount"`
		} `json:"info"`
		Type string `json:"type"`
	} `json:"parsed"`
	Program string `json:"program"`
	Space   int    `json:"space"`
}

// SolanaTransactionResult represents transaction submission result
type SolanaTransactionResult string

// SolanaTransactionStatusResult represents transaction status
type SolanaTransactionStatusResult struct {
	Context struct {
		Slot int64 `json:"slot"`
	} `json:"context"`
	Value struct {
		Slot               int64                  `json:"slot"`
		Confirmations      *int                   `json:"confirmations"`
		Err                interface{}            `json:"err"`
		ConfirmationStatus string                 `json:"confirmationStatus"`
		Meta               *SolanaTransactionMeta `json:"meta"`
	} `json:"value"`
}

// SolanaTransactionMeta represents transaction metadata
type SolanaTransactionMeta struct {
	Err               interface{}   `json:"err"`
	Fee               int64         `json:"fee"`
	InnerInstructions []interface{} `json:"innerInstructions"`
	LogMessages       []string      `json:"logMessages"`
	PostBalances      []int64       `json:"postBalances"`
	PostTokenBalances []interface{} `json:"postTokenBalances"`
	PreBalances       []int64       `json:"preBalances"`
	PreTokenBalances  []interface{} `json:"preTokenBalances"`
	Rewards           []interface{} `json:"rewards"`
	Status            interface{}   `json:"status"`
}

// NewSolanaClient creates a new Solana RPC client
func NewSolanaClient(rpcURL string) *SolanaClient {
	return &SolanaClient{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		rpcURL: rpcURL,
	}
}

// GetBalance retrieves SOL balance for an address
func (s *SolanaClient) GetBalance(ctx context.Context, address string) (int64, error) {
	req := SolanaRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "getBalance",
		Params:  []interface{}{address},
	}

	var result SolanaBalanceResult
	if err := s.makeRPCCall(ctx, req, &result); err != nil {
		return 0, fmt.Errorf("failed to get balance: %w", err)
	}

	return result.Value, nil
}

// GetTokenAccountsByOwner retrieves token accounts for an owner
func (s *SolanaClient) GetTokenAccountsByOwner(ctx context.Context, owner string, mint string) ([]SolanaTokenAccount, error) {
	params := []interface{}{
		owner,
		map[string]interface{}{
			"mint": mint,
		},
		map[string]interface{}{
			"encoding": "jsonParsed",
		},
	}

	req := SolanaRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "getTokenAccountsByOwner",
		Params:  params,
	}

	var result SolanaTokenAccountResult
	if err := s.makeRPCCall(ctx, req, &result); err != nil {
		return nil, fmt.Errorf("failed to get token accounts: %w", err)
	}

	return result.Value, nil
}

// GetTokenBalance retrieves token balance for a specific mint
func (s *SolanaClient) GetTokenBalance(ctx context.Context, owner string, mint string) (int64, int, error) {
	accounts, err := s.GetTokenAccountsByOwner(ctx, owner, mint)
	if err != nil {
		return 0, 0, err
	}

	if len(accounts) == 0 {
		return 0, 0, nil // No token accounts found
	}

	// Use the first account (user might have multiple accounts for same token)
	account := accounts[0]
	amountStr := account.Account.Data.Parsed.Info.TokenAmount.Amount
	decimals := account.Account.Data.Parsed.Info.TokenAmount.Decimals

	amount, err := strconv.ParseInt(amountStr, 10, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to parse token amount: %w", err)
	}

	return amount, decimals, nil
}

// SendTransaction sends a transaction to the Solana network
func (s *SolanaClient) SendTransaction(ctx context.Context, transaction string, options map[string]interface{}) (string, error) {
	params := []interface{}{transaction}

	if options != nil {
		params = append(params, options)
	} else {
		// Default options
		params = append(params, map[string]interface{}{
			"encoding":            "base64",
			"skipPreflight":       false,
			"preflightCommitment": "confirmed",
			"maxRetries":          3,
		})
	}

	req := SolanaRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "sendTransaction",
		Params:  params,
	}

	var result SolanaTransactionResult
	if err := s.makeRPCCall(ctx, req, &result); err != nil {
		return "", fmt.Errorf("failed to send transaction: %w", err)
	}

	return string(result), nil
}

// GetTransaction retrieves transaction details
func (s *SolanaClient) GetTransaction(ctx context.Context, signature string) (*SolanaTransactionStatusResult, error) {
	params := []interface{}{
		signature,
		map[string]interface{}{
			"encoding":   "json",
			"commitment": "confirmed",
		},
	}

	req := SolanaRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "getTransaction",
		Params:  params,
	}

	var result SolanaTransactionStatusResult
	if err := s.makeRPCCall(ctx, req, &result); err != nil {
		return nil, fmt.Errorf("failed to get transaction: %w", err)
	}

	return &result, nil
}

// GetTransactionStatus checks transaction confirmation status
func (s *SolanaClient) GetTransactionStatus(ctx context.Context, signature string) (string, error) {
	tx, err := s.GetTransaction(ctx, signature)
	if err != nil {
		return "unknown", err
	}

	if tx.Value.Err != nil {
		return "failed", nil
	}

	return tx.Value.ConfirmationStatus, nil
}

// WaitForTransactionConfirmation waits for transaction confirmation with timeout
func (s *SolanaClient) WaitForTransactionConfirmation(ctx context.Context, signature string, timeout time.Duration) (string, error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeoutCtx.Done():
			return "timeout", fmt.Errorf("transaction confirmation timeout after %v", timeout)
		case <-ticker.C:
			status, err := s.GetTransactionStatus(timeoutCtx, signature)
			if err != nil {
				// Transaction might not be found yet, continue waiting
				continue
			}

			switch status {
			case "confirmed", "finalized":
				return status, nil
			case "failed":
				return status, fmt.Errorf("transaction failed")
			case "processed":
				// Continue waiting for confirmation
				continue
			default:
				// Unknown status, continue waiting
				continue
			}
		}
	}
}

// GetLatestBlockhash retrieves the latest blockhash
func (s *SolanaClient) GetLatestBlockhash(ctx context.Context) (string, int64, error) {
	req := SolanaRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "getLatestBlockhash",
		Params:  []interface{}{},
	}

	var result struct {
		Context struct {
			Slot int64 `json:"slot"`
		} `json:"context"`
		Value struct {
			Blockhash            string `json:"blockhash"`
			LastValidBlockHeight int64  `json:"lastValidBlockHeight"`
		} `json:"value"`
	}

	if err := s.makeRPCCall(ctx, req, &result); err != nil {
		return "", 0, fmt.Errorf("failed to get latest blockhash: %w", err)
	}

	return result.Value.Blockhash, result.Value.LastValidBlockHeight, nil
}

// makeRPCCall makes a JSON-RPC call to Solana and decodes the result
func (s *SolanaClient) makeRPCCall(ctx context.Context, req SolanaRPCRequest, result interface{}) error {
	reqBody, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, s.rpcURL, bytes.NewBuffer(reqBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("User-Agent", "cash-farmer-bot/1.0")

	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Solana RPC request failed with status %d", resp.StatusCode)
	}

	var rpcResp SolanaRPCResponse
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if rpcResp.Error != nil {
		return fmt.Errorf("Solana RPC error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}

	if result != nil && rpcResp.Result != nil {
		if err := json.Unmarshal(rpcResp.Result, result); err != nil {
			return fmt.Errorf("failed to decode result: %w", err)
		}
	}

	return nil
}
