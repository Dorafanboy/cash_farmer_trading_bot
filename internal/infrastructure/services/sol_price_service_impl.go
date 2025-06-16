package services

import (
	"context"
	"fmt"
	"time"

	"cash-farmer/internal/application/repositories"
	"cash-farmer/internal/domain/valueobjects"
	"cash-farmer/internal/infrastructure/api"
)

// SolPriceServiceImpl implements SOL price caching logic using DexScreener
type SolPriceServiceImpl struct {
	dexscreenerClient *api.DexscreenerClient
	cacheRepository   repositories.CacheRepository
	cacheTTL          time.Duration
}

// NewSolPriceServiceImpl creates a new SOL price service implementation using DexScreener
func NewSolPriceServiceImpl(
	dexscreenerClient *api.DexscreenerClient,
	cacheRepository repositories.CacheRepository,
	cacheTTL time.Duration,
) *SolPriceServiceImpl {
	return &SolPriceServiceImpl{
		dexscreenerClient: dexscreenerClient,
		cacheRepository:   cacheRepository,
		cacheTTL:          cacheTTL,
	}
}

// getSolPriceFromDexScreener получает цену SOL через DexScreener API
func (s *SolPriceServiceImpl) getSolPriceFromDexScreener(ctx context.Context) (*valueobjects.SolPrice, error) {
	// Адрес SOL токена в Solana
	solAddress, err := valueobjects.NewSolanaTokenAddress("So11111111111111111111111111111111111111112")
	if err != nil {
		return nil, fmt.Errorf("failed to create SOL address: %w", err)
	}

	// Получаем метрики токена через DexScreener
	metrics, err := s.dexscreenerClient.GetTokenMetrics(ctx, *solAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to get SOL metrics from DexScreener: %w", err)
	}

	// Создаем SolPrice из полученной цены
	solPrice := valueobjects.NewSolPrice(metrics.PriceUSD())

	return solPrice, nil
}

// GetCachedSolPrice retrieves SOL price from cache or fetches fresh data from DexScreener
func (s *SolPriceServiceImpl) GetCachedSolPrice(ctx context.Context) (*valueobjects.SolPrice, error) {
	cachedPrice, err := s.cacheRepository.GetSolPrice(ctx)
	if err == nil && !cachedPrice.IsExpired(s.cacheTTL) {
		return cachedPrice, nil
	}

	freshPrice, err := s.getSolPriceFromDexScreener(ctx)
	if err != nil {
		if cachedPrice != nil {
			fmt.Printf("⚠️ Failed to get fresh SOL price from DexScreener, using cached: %v\n", err)
			return cachedPrice, nil
		}
		return nil, fmt.Errorf("failed to fetch SOL price and no cache available: %w", err)
	}

	priceFloat, _ := freshPrice.PriceUSD().Float64()
	fmt.Printf("✅ Got fresh SOL price from DexScreener: $%.2f\n", priceFloat)

	if err := s.cacheRepository.SetSolPrice(ctx, freshPrice, s.cacheTTL); err != nil {
		fmt.Printf("⚠️ Failed to cache SOL price: %v\n", err)
		return freshPrice, nil
	}

	return freshPrice, nil
}

// RefreshSolPrice forcefully refreshes SOL price in cache using DexScreener
func (s *SolPriceServiceImpl) RefreshSolPrice(ctx context.Context) error {
	freshPrice, err := s.getSolPriceFromDexScreener(ctx)
	if err != nil {
		return fmt.Errorf("failed to fetch fresh SOL price from DexScreener: %w", err)
	}

	priceFloat, _ := freshPrice.PriceUSD().Float64()
	fmt.Printf("✅ Refreshed SOL price from DexScreener: $%.2f\n", priceFloat)

	if err := s.cacheRepository.SetSolPrice(ctx, freshPrice, s.cacheTTL); err != nil {
		return fmt.Errorf("failed to cache fresh SOL price: %w", err)
	}

	return nil
}
