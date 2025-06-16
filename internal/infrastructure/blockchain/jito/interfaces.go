package jito

import (
	"context"
	"time"
)

// JitoClientInterface defines the interface for Jito clients
// This allows easy switching between implementations and testing
type JitoClientInterface interface {
	// Core bundle operations
	SendBundle(ctx context.Context, transactions []string) (*JitoBundleResult, error)
	GetBundleStatuses(ctx context.Context, bundleIDs []string) ([]JitoBundleStatus, error)
	GetBundleStatus(ctx context.Context, bundleID string) (*JitoBundleStatus, error)
	GetInflightBundleStatuses(ctx context.Context) ([]JitoInflightBundle, error)

	// Tip account operations
	GetTipAccounts(ctx context.Context) ([]string, error)
	GetRandomTipAccount(ctx context.Context) (*TipAccount, error)
	GetRandomTipAccountAddress(ctx context.Context) (string, error)

	// Transaction operations
	SendTransaction(ctx context.Context, txData string, bundleOnly bool) (string, error)
	SendBundleOnly(ctx context.Context, transactions []string) (*JitoBundleResult, error)

	// Utility methods
	WaitForBundleConfirmation(ctx context.Context, bundleID string, timeout time.Duration) (*JitoBundleStatus, error)
	ValidateBundle(transactions []string) error

	// Configuration
	SetUUID(uuid string)
	SetDebug(enabled bool)
	GetConfig() *Config
}

// Ensure EnhancedJitoClient implements the interface
var _ JitoClientInterface = (*EnhancedJitoClient)(nil)

// LegacyJitoClientInterface defines backward compatibility interface
// This matches the existing api/jito_client.go interface
type LegacyJitoClientInterface interface {
	SendBundle(ctx context.Context, transactions []string) (*JitoBundleResult, error)
	GetBundleStatuses(ctx context.Context, bundleIDs []string) ([]JitoBundleStatus, error)
	GetBundleStatus(ctx context.Context, bundleID string) (*JitoBundleStatus, error)
	GetInflightBundleStatuses(ctx context.Context) ([]JitoInflightBundle, error)
	GetTipAccounts(ctx context.Context) ([]string, error)
	WaitForBundleConfirmation(ctx context.Context, bundleID string, timeout time.Duration) (*JitoBundleStatus, error)
	ValidateBundle(transactions []string) error
}

// Ensure EnhancedJitoClient implements legacy interface for backward compatibility
var _ LegacyJitoClientInterface = (*EnhancedJitoClient)(nil)

// ClientMode represents the mode of operation for the Jito client
type ClientMode int

const (
	// ModeStandard uses regular Jito RPC endpoints
	ModeStandard ClientMode = iota
	// ModeAuthenticated uses authenticated endpoints with UUID
	ModeAuthenticated
	// ModeBundleOnly uses bundle-only mode for maximum MEV protection
	ModeBundleOnly
)

// ClientCapabilities describes the capabilities of a Jito client
type ClientCapabilities struct {
	SupportsUUIDAuth   bool       // Client supports UUID authentication
	SupportsBundleOnly bool       // Client supports bundle-only mode
	SupportsRandomTips bool       // Client supports automatic tip account selection
	MaxBundleSize      int        // Maximum number of transactions per bundle
	DefaultMode        ClientMode // Default operation mode
}

// GetCapabilities returns the capabilities of the enhanced client
func (e *EnhancedJitoClient) GetCapabilities() ClientCapabilities {
	return ClientCapabilities{
		SupportsUUIDAuth:   true,
		SupportsBundleOnly: true,
		SupportsRandomTips: true,
		MaxBundleSize:      5,
		DefaultMode:        ModeStandard,
	}
}

// Factory function type for creating Jito clients
type JitoClientFactory func(config *Config) JitoClientInterface

// DefaultFactory creates enhanced Jito clients
func DefaultFactory(config *Config) JitoClientInterface {
	return NewEnhancedJitoClient(config)
}
