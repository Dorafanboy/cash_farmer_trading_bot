package cache

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/goccy/go-json"
	"github.com/redis/go-redis/v9"

	"cash-farmer/internal/domain/valueobjects"
)

const (
	solPriceKey = "sol_price_usd"
)

// RedisCache implements cache operations using Redis
type RedisCache struct {
	client redis.UniversalClient
}

// SolPriceData represents SOL price data for Redis storage
type SolPriceData struct {
	PriceUSD  string    `json:"price_usd"`
	Timestamp time.Time `json:"timestamp"`
}

// NewRedisCache creates a new Redis cache instance
func NewRedisCache(client redis.UniversalClient) *RedisCache {
	return &RedisCache{
		client: client,
	}
}

// GetSolPrice retrieves SOL price from Redis cache
func (r *RedisCache) GetSolPrice(ctx context.Context) (*valueobjects.SolPrice, error) {
	result, err := r.client.Get(ctx, solPriceKey).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, fmt.Errorf("SOL price not found in cache")
		}
		return nil, fmt.Errorf("failed to get SOL price from cache: %w", err)
	}

	var data SolPriceData
	if err := json.Unmarshal([]byte(result), &data); err != nil {
		return nil, fmt.Errorf("failed to unmarshal SOL price data: %w", err)
	}

	solPrice, err := valueobjects.NewSolPriceFromString(data.PriceUSD)
	if err != nil {
		return nil, fmt.Errorf("failed to create SOL price from cached data: %w", err)
	}

	return solPrice, nil
}

// SetSolPrice stores SOL price in Redis cache with TTL
func (r *RedisCache) SetSolPrice(ctx context.Context, price *valueobjects.SolPrice, ttl time.Duration) error {
	data := SolPriceData{
		PriceUSD:  price.PriceUSD().String(),
		Timestamp: price.Timestamp(),
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal SOL price data: %w", err)
	}

	if err := r.client.Set(ctx, solPriceKey, jsonData, ttl).Err(); err != nil {
		return fmt.Errorf("failed to set SOL price in cache: %w", err)
	}

	return nil
}
