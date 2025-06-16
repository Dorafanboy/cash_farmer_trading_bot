package services

import (
	"context"
	"fmt"

	"cash-farmer/internal/domain/valueobjects"
)

// PriceServiceAdapter adapts TokenDataService and SolPriceService to PriceService interface
type PriceServiceAdapter struct {
	tokenDataService TokenDataService
	solPriceService  SolPriceService
}

// NewPriceServiceAdapter creates a new price service adapter
func NewPriceServiceAdapter(
	tokenDataService TokenDataService,
	solPriceService SolPriceService,
) PriceService {
	return &PriceServiceAdapter{
		tokenDataService: tokenDataService,
		solPriceService:  solPriceService,
	}
}

// GetTokenPrice gets price for a single token
func (p *PriceServiceAdapter) GetTokenPrice(ctx context.Context, tokenAddress string) (float64, error) {
	// Check if it's SOL address
	if tokenAddress == "So11111111111111111111111111111111111111112" {
		solPrice, err := p.solPriceService.GetCachedSolPrice(ctx)
		if err != nil {
			return 0, fmt.Errorf("failed to get SOL price: %w", err)
		}
		priceFloat, _ := solPrice.PriceUSD().Float64()
		return priceFloat, nil
	}

	// For other tokens, use TokenDataService
	address, err := valueobjects.NewSolanaTokenAddress(tokenAddress)
	if err != nil {
		return 0, fmt.Errorf("invalid token address: %w", err)
	}

	metrics, err := p.tokenDataService.GetTokenMetrics(ctx, *address)
	if err != nil {
		return 0, fmt.Errorf("failed to get token metrics: %w", err)
	}

	priceFloat, _ := metrics.PriceUSD().Float64()
	return priceFloat, nil
}

// GetMultipleTokenPrices gets prices for multiple tokens
func (p *PriceServiceAdapter) GetMultipleTokenPrices(
	ctx context.Context,
	tokenAddresses []string,
) (map[string]float64, error) {
	prices := make(map[string]float64)

	for _, address := range tokenAddresses {
		price, err := p.GetTokenPrice(ctx, address)
		if err != nil {
			// Log error but continue with other tokens
			continue
		}
		prices[address] = price
	}

	return prices, nil
}
