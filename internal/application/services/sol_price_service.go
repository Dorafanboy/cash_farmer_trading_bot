package services

import (
	"context"

	"cash-farmer/internal/domain/valueobjects"
)

// SolPriceService handles SOL price caching operations
type SolPriceService interface {
	GetCachedSolPrice(ctx context.Context) (*valueobjects.SolPrice, error)
	RefreshSolPrice(ctx context.Context) error
}
