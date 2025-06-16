package api

import "cash-farmer/internal/infrastructure/blockchain/jito"

// JitoClient is an alias for backward compatibility
// This allows existing code to continue using api.JitoClient
// while the actual implementation is now in the jito package
type JitoClient = jito.LegacyJitoClient

// NewJitoClient creates a new Jito client with enhanced functionality
// This function maintains backward compatibility with existing code
func NewJitoClient(rpcURL string) *JitoClient {
	return jito.NewJitoClient(rpcURL)
}

// NewJitoClientFromEnhanced creates a legacy client from enhanced client
func NewJitoClientFromEnhanced(enhanced jito.JitoClientInterface, rpcURL string) *JitoClient {
	return jito.NewJitoClientFromEnhanced(enhanced, rpcURL)
}
