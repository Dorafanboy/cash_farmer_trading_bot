package services

import (
	"context"

	"cash-farmer/internal/domain/valueobjects"
)

// TokenDataService handles token data operations
type TokenDataService interface {
	GetTokenMetrics(ctx context.Context, address valueobjects.SolanaTokenAddress) (*valueobjects.TokenMetrics, error)
}
