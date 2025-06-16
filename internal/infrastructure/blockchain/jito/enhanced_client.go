package jito

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/goccy/go-json"
	"github.com/valyala/fasthttp"
)

// Config holds configuration for Enhanced Jito Client
type Config struct {
	BaseURL string // Jito RPC URL
	UUID    string // UUID for authenticated endpoints (optional)
	Debug   bool   // Enable debug logging
}

// EnhancedJitoClient собственная реализация Jito клиента с fasthttp + go-json
// БЕЗ зависимости от jito-go-rpc
type EnhancedJitoClient struct {
	client *fasthttp.Client
	config *Config
}

// Backward compatibility types - matching existing api/jito_client.go
type JitoBundleResult struct {
	BundleID string `json:"bundleId"`
}

type JitoBundleStatus struct {
	BundleID     string   `json:"bundleId"`
	Status       string   `json:"status"` // "pending", "processing", "confirmed", "failed"
	LandedSlot   *int64   `json:"landedSlot,omitempty"`
	Transactions []string `json:"transactions"`
	Error        *string  `json:"error,omitempty"`
}

type JitoInflightBundle struct {
	BundleID     string   `json:"bundleId"`
	Transactions []string `json:"transactions"`
	Slot         int64    `json:"slot"`
}

// Internal RPC request/response types
type jitoRPCRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int         `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params"`
}

type jitoRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *jitoRPCError   `json:"error,omitempty"`
}

type jitoRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// NewEnhancedJitoClient creates a new enhanced Jito client
func NewEnhancedJitoClient(config *Config) *EnhancedJitoClient {
	return &EnhancedJitoClient{
		client: &fasthttp.Client{
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
		},
		config: config,
	}
}

// NewEnhancedJitoClientFromURL creates client with minimal config (backward compatibility)
func NewEnhancedJitoClientFromURL(rpcURL string) *EnhancedJitoClient {
	config := &Config{
		BaseURL: rpcURL,
		UUID:    "", // No authentication by default
		Debug:   false,
	}
	return NewEnhancedJitoClient(config)
}

// makeRPCCall выполняет JSON-RPC вызов к Jito API
func (e *EnhancedJitoClient) makeRPCCall(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
	req := jitoRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  method,
		Params:  params,
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	if e.config.Debug {
		log.Printf("🔍 JITO RPC REQUEST: %s to %s", method, e.config.BaseURL)
	}

	httpReq := fasthttp.AcquireRequest()
	httpResp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(httpReq)
	defer fasthttp.ReleaseResponse(httpResp)

	httpReq.SetRequestURI(e.config.BaseURL)
	httpReq.Header.SetMethod(fasthttp.MethodPost)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("User-Agent", "cash-farmer-bot/1.0")

	// Add UUID header if configured
	if e.config.UUID != "" {
		httpReq.Header.Set("X-UUID", e.config.UUID)
	}

	httpReq.SetBody(reqBody)

	err = e.client.DoTimeout(httpReq, httpResp, 30*time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}

	if httpResp.StatusCode() != fasthttp.StatusOK {
		return nil, fmt.Errorf("jito RPC request failed with status %d", httpResp.StatusCode())
	}

	var rpcResp jitoRPCResponse
	if err := json.Unmarshal(httpResp.Body(), &rpcResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if rpcResp.Error != nil {
		return nil, fmt.Errorf("jito RPC error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}

	if e.config.Debug {
		log.Printf("✅ JITO RPC SUCCESS: %s", method)
	}

	return rpcResp.Result, nil
}

// BACKWARD COMPATIBILITY METHODS - Matching existing JitoClient interface

// SendBundle submits a bundle of transactions to Jito
func (e *EnhancedJitoClient) SendBundle(ctx context.Context, transactions []string) (*JitoBundleResult, error) {
	if err := e.ValidateBundle(transactions); err != nil {
		return nil, fmt.Errorf("bundle validation failed: %w", err)
	}

	log.Printf("Sending bundle with %d transactions via enhanced client", len(transactions))

	params := map[string]interface{}{
		"encodedTransactions": transactions,
	}

	result, err := e.makeRPCCall(ctx, "sendBundle", params)
	if err != nil {
		return nil, fmt.Errorf("failed to send bundle: %w", err)
	}

	// Parse the bundle ID from the result
	var bundleID string
	if err := json.Unmarshal(result, &bundleID); err != nil {
		return nil, fmt.Errorf("failed to parse bundle ID: %w", err)
	}

	return &JitoBundleResult{
		BundleID: bundleID,
	}, nil
}

// GetBundleStatuses retrieves status for multiple bundles
func (e *EnhancedJitoClient) GetBundleStatuses(ctx context.Context, bundleIDs []string) ([]JitoBundleStatus, error) {
	if len(bundleIDs) == 0 {
		return []JitoBundleStatus{}, nil
	}

	params := map[string]interface{}{
		"value": bundleIDs,
	}

	result, err := e.makeRPCCall(ctx, "getBundleStatuses", params)
	if err != nil {
		return nil, fmt.Errorf("failed to get bundle statuses: %w", err)
	}

	// Parse bundle statuses response
	var response struct {
		Context struct {
			Slot int64 `json:"slot"`
		} `json:"context"`
		Value []struct {
			BundleID           string      `json:"bundle_id"`
			ConfirmationStatus string      `json:"confirmation_status"`
			Slot               int64       `json:"slot"`
			Transactions       []string    `json:"transactions"`
			Err                interface{} `json:"err"`
		} `json:"value"`
	}

	if err := json.Unmarshal(result, &response); err != nil {
		return nil, fmt.Errorf("failed to parse bundle statuses: %w", err)
	}

	// Convert to our format
	var statuses []JitoBundleStatus
	for _, value := range response.Value {
		status := JitoBundleStatus{
			BundleID:     value.BundleID,
			Status:       value.ConfirmationStatus,
			LandedSlot:   &value.Slot,
			Transactions: value.Transactions,
		}

		// Handle error field if present
		if value.Err != nil {
			if errStr, ok := value.Err.(string); ok && errStr != "" {
				status.Error = &errStr
			}
		}

		statuses = append(statuses, status)
	}

	return statuses, nil
}

// GetBundleStatus retrieves status for a single bundle
func (e *EnhancedJitoClient) GetBundleStatus(ctx context.Context, bundleID string) (*JitoBundleStatus, error) {
	statuses, err := e.GetBundleStatuses(ctx, []string{bundleID})
	if err != nil {
		return nil, err
	}

	if len(statuses) == 0 {
		return nil, fmt.Errorf("bundle %s not found", bundleID)
	}

	return &statuses[0], nil
}

// GetInflightBundleStatuses retrieves all inflight bundles
func (e *EnhancedJitoClient) GetInflightBundleStatuses(ctx context.Context) ([]JitoInflightBundle, error) {
	result, err := e.makeRPCCall(ctx, "getInflightBundleStatuses", []interface{}{})
	if err != nil {
		return nil, fmt.Errorf("failed to get inflight bundle statuses: %w", err)
	}

	// Parse the result
	var bundles []JitoInflightBundle
	if err := json.Unmarshal(result, &bundles); err != nil {
		// Try alternative parsing if needed
		log.Printf("Failed to parse inflight bundles as expected format: %v", err)
		return []JitoInflightBundle{}, nil
	}

	return bundles, nil
}

// GetTipAccounts retrieves Jito tip accounts for priority fees
func (e *EnhancedJitoClient) GetTipAccounts(ctx context.Context) ([]string, error) {
	result, err := e.makeRPCCall(ctx, "getTipAccounts", []interface{}{})
	if err != nil {
		return nil, fmt.Errorf("failed to get tip accounts: %w", err)
	}

	var tipAddresses []string
	if err := json.Unmarshal(result, &tipAddresses); err != nil {
		return nil, fmt.Errorf("failed to parse tip accounts: %w", err)
	}

	return tipAddresses, nil
}

// WaitForBundleConfirmation waits for bundle to be confirmed with timeout
func (e *EnhancedJitoClient) WaitForBundleConfirmation(ctx context.Context, bundleID string, timeout time.Duration) (*JitoBundleStatus, error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeoutCtx.Done():
			return nil, fmt.Errorf("bundle confirmation timeout after %v", timeout)
		case <-ticker.C:
			status, err := e.GetBundleStatus(timeoutCtx, bundleID)
			if err != nil {
				return nil, fmt.Errorf("failed to check bundle status: %w", err)
			}

			switch status.Status {
			case "confirmed", "finalized":
				return status, nil
			case "failed":
				errorMsg := "unknown error"
				if status.Error != nil {
					errorMsg = *status.Error
				}
				return status, fmt.Errorf("bundle failed: %s", errorMsg)
			case "pending", "processing", "processed":
				// Continue waiting
				continue
			default:
				return status, fmt.Errorf("unknown bundle status: %s", status.Status)
			}
		}
	}
}

// ValidateBundle performs basic validation on bundle before submission
func (e *EnhancedJitoClient) ValidateBundle(transactions []string) error {
	if len(transactions) == 0 {
		return fmt.Errorf("bundle cannot be empty")
	}

	if len(transactions) > 5 {
		return fmt.Errorf("bundle cannot contain more than 5 transactions, got %d", len(transactions))
	}

	for i, tx := range transactions {
		if tx == "" {
			return fmt.Errorf("transaction %d is empty", i)
		}
		// Basic base64 validation - should be valid base64 string
		if len(tx) < 100 { // Solana transactions are typically much longer
			return fmt.Errorf("transaction %d appears to be too short: %d characters", i, len(tx))
		}
	}

	return nil
}

// ENHANCED METHODS - New functionality

// Static list of known Jito tip accounts (fallback for rate-limited endpoints)
var staticTipAccounts = []string{
	"96gYZGLnJYVFmbjzopPSU6QiEV5fGqZNyN9nmNhvrZU5",
	"HFqU5x63VTqvQss8hp11i4wVV8bD44PvwucfZ2bU7gRe",
	"Cw8CFyM9FkoMi7K7Crf6HNQqf4uEMzpKw6QNghXLvLkY",
	"ADaUMid9yfUytqMBgopwjb2DTLSokTSzL1zt6iGPaS49",
	"DfXygSm4jCyNCybVYYK6DwvWqjKee8pbDmJGcLWNDXjh",
	"ADuUkR4vqLUMWXxW9gh6D6L8pMSawimctcNZ5pGwDcEt",
	"DttWaMuVvTiduZRnguLF7jNxTgiMBZ1hyAumKUiL2KRL",
	"3AVi9Tg9Uo68tJfuvoKvqKNWKkC5wPdSSdeBnizKZ6jT",
}

// GetRandomTipAccount returns a random tip account for MEV protection
func (e *EnhancedJitoClient) GetRandomTipAccount(ctx context.Context) (*TipAccount, error) {
	// Try to get tip accounts from API first
	tipAccounts, err := e.GetTipAccounts(ctx)
	if err == nil && len(tipAccounts) > 0 {
		// Use first account from API
		selectedAddress := tipAccounts[0]
		return &TipAccount{
			Account:   selectedAddress,
			PublicKey: selectedAddress,
		}, nil
	}

	// Fallback to static tip accounts
	log.Printf("🔧 TIP ACCOUNTS: Using static tip account (API unavailable)")
	staticIndex := len(staticTipAccounts) - 1 // Use last account as default
	if len(staticTipAccounts) > 1 {
		// Simple rotation based on time
		currentTime := time.Now().Unix()
		staticIndex = int(currentTime) % len(staticTipAccounts)
	}

	selectedAddress := staticTipAccounts[staticIndex]
	return &TipAccount{
		Account:   selectedAddress,
		PublicKey: selectedAddress, // Both fields for compatibility
	}, nil
}

// SendTransaction sends a single transaction with optional bundle-only mode
func (e *EnhancedJitoClient) SendTransaction(ctx context.Context, txData string, bundleOnly bool) (string, error) {
	params := map[string]interface{}{
		"encodedTransaction": txData,
		"bundleOnly":         bundleOnly,
	}

	result, err := e.makeRPCCall(ctx, "sendTransaction", params)
	if err != nil {
		return "", fmt.Errorf("failed to send transaction: %w", err)
	}

	// Parse transaction signature or bundle ID
	var signature string
	if err := json.Unmarshal(result, &signature); err != nil {
		return "", fmt.Errorf("failed to parse transaction result: %w", err)
	}

	return signature, nil
}

// SendBundleOnly sends a bundle in bundle-only mode for maximum MEV protection
func (e *EnhancedJitoClient) SendBundleOnly(ctx context.Context, transactions []string) (*JitoBundleResult, error) {
	if err := e.ValidateBundle(transactions); err != nil {
		return nil, fmt.Errorf("bundle validation failed: %w", err)
	}

	log.Printf("Sending bundle-only with %d transactions for MEV protection", len(transactions))

	// Use bundle submission directly for maximum MEV protection
	return e.SendBundle(ctx, transactions)
}

// GetRandomTipAccountAddress is a convenience method for getting just the address
func (e *EnhancedJitoClient) GetRandomTipAccountAddress(ctx context.Context) (string, error) {
	tipAccount, err := e.GetRandomTipAccount(ctx)
	if err != nil {
		return "", err
	}
	return tipAccount.Account, nil
}

// SetUUID updates the UUID for authenticated endpoints
func (e *EnhancedJitoClient) SetUUID(uuid string) {
	e.config.UUID = uuid
}

// SetDebug enables or disables debug logging
func (e *EnhancedJitoClient) SetDebug(enabled bool) {
	e.config.Debug = enabled
}

// GetConfig returns the current client configuration
func (e *EnhancedJitoClient) GetConfig() *Config {
	return e.config
}
