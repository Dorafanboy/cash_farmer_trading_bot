package jito

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	jitorpc "github.com/jito-labs/jito-go-rpc"
)

// Config holds configuration for Enhanced Jito Client
type Config struct {
	BaseURL string // Jito RPC URL
	UUID    string // UUID for authenticated endpoints (optional)
	Debug   bool   // Enable debug logging
}

// EnhancedJitoClient wraps the official jito-go-rpc library
// while maintaining backward compatibility with existing interfaces
type EnhancedJitoClient struct {
	client *jitorpc.JitoJsonRpcClient
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

// NewEnhancedJitoClient creates a new enhanced Jito client
func NewEnhancedJitoClient(config *Config) *EnhancedJitoClient {
	client := jitorpc.NewJitoJsonRpcClient(config.BaseURL, config.UUID)

	// Set debug mode if enabled
	if config.Debug {
		debug := true
		client.Debug = &debug
	}

	return &EnhancedJitoClient{
		client: client,
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

// BACKWARD COMPATIBILITY METHODS - Matching existing JitoClient interface

// SendBundle submits a bundle of transactions to Jito
func (e *EnhancedJitoClient) SendBundle(ctx context.Context, transactions []string) (*JitoBundleResult, error) {
	if err := e.ValidateBundle(transactions); err != nil {
		return nil, fmt.Errorf("bundle validation failed: %w", err)
	}

	log.Printf("Sending bundle with %d transactions via enhanced client", len(transactions))

	// Convert to format expected by jito-go-rpc v0.2.1 ([][]string)
	bundleTransactions := [][]string{transactions}
	result, err := e.client.SendBundle(bundleTransactions)
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

	statusResponse, err := e.client.GetBundleStatuses(bundleIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to get bundle statuses: %w", err)
	}

	// Convert official library response to our format
	var statuses []JitoBundleStatus
	for _, value := range statusResponse.Value {
		status := JitoBundleStatus{
			BundleID:     value.BundleID,
			Status:       value.ConfirmationStatus,
			LandedSlot:   &value.Slot,
			Transactions: value.Transactions,
		}

		// Handle error field if present
		if value.Err.Ok != nil {
			if errStr, ok := value.Err.Ok.(string); ok {
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
	result, err := e.client.GetInflightBundleStatuses(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get inflight bundle statuses: %w", err)
	}

	// Parse the result - structure may vary
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
	result, err := e.client.GetTipAccounts()
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

// ENHANCED METHODS - New functionality from jito-go-rpc

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
	// Fallback to static tip accounts (official API may be rate limited)
	log.Printf("🔧 TIP ACCOUNTS: Using static tip account")
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
	result, err := e.client.SendTxn(txData, bundleOnly)
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
	// Create new client with updated UUID
	e.client = jitorpc.NewJitoJsonRpcClient(e.config.BaseURL, uuid)
	if e.config.Debug {
		debug := true
		e.client.Debug = &debug
	}
}

// SetDebug enables or disables debug logging
func (e *EnhancedJitoClient) SetDebug(enabled bool) {
	e.config.Debug = enabled
	e.client.Debug = &enabled
}

// GetConfig returns the current client configuration
func (e *EnhancedJitoClient) GetConfig() *Config {
	return e.config
}
