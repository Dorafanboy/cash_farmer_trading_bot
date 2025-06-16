package repositories

import (
	"context"
	"time"

	"cash-farmer/internal/domain/valueobjects"
)

type CacheRepository interface {
	GetSolPrice(ctx context.Context) (*valueobjects.SolPrice, error)
	SetSolPrice(ctx context.Context, price *valueobjects.SolPrice, ttl time.Duration) error
}
