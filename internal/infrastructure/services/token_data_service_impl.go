package services

import (
	"context"
	"fmt"

	"cash-farmer/internal/domain/valueobjects"
	"cash-farmer/internal/infrastructure/api"
)

// TokenDataServiceImpl implements token data operations using Dexscreener
type TokenDataServiceImpl struct {
	dexscreenerClient *api.DexscreenerClient
}

// NewTokenDataServiceImpl creates a new token data service implementation
func NewTokenDataServiceImpl(dexscreenerClient *api.DexscreenerClient) *TokenDataServiceImpl {
	return &TokenDataServiceImpl{
		dexscreenerClient: dexscreenerClient,
	}
}

// GetTokenMetrics fetches token metrics using Dexscreener API
func (t *TokenDataServiceImpl) GetTokenMetrics(
	ctx context.Context,
	address valueobjects.SolanaTokenAddress,
) (*valueobjects.TokenMetrics, error) {
	metrics, err := t.dexscreenerClient.GetTokenMetrics(ctx, address)
	if err != nil {
		return nil, fmt.Errorf("failed to get token metrics for %s: %w", address.ShortString(), err)
	}

	return metrics, nil
}
