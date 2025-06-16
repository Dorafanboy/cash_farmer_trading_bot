package entities

import (
	"time"
)

// UserBalance represents cached SOL balance for a user's wallet
type UserBalance struct {
	ID              int64     `json:"id"`
	WalletID        int64     `json:"wallet_id"`
	SOLBalance      float64   `json:"sol_balance"`
	LastUpdated     time.Time `json:"last_updated"`
	CacheValidUntil time.Time `json:"cache_valid_until"`
}

// NewUserBalance creates a new UserBalance instance
func NewUserBalance(walletID int64, solBalance float64) *UserBalance {
	now := time.Now()
	return &UserBalance{
		WalletID:        walletID,
		SOLBalance:      solBalance,
		LastUpdated:     now,
		CacheValidUntil: now.Add(10 * time.Minute), // Default 10 minute cache
	}
}

// IsValid checks if the cached balance is still valid
func (ub *UserBalance) IsValid() bool {
	return time.Now().Before(ub.CacheValidUntil)
}

// IsExpired checks if the cached balance has expired
func (ub *UserBalance) IsExpired() bool {
	return !ub.IsValid()
}

// Update updates the balance and refreshes the cache validity
func (ub *UserBalance) Update(newBalance float64) {
	ub.SOLBalance = newBalance
	ub.LastUpdated = time.Now()
	ub.CacheValidUntil = time.Now().Add(10 * time.Minute)
}

// ExtendCache extends the cache validity by another 10 minutes
func (ub *UserBalance) ExtendCache() {
	ub.CacheValidUntil = time.Now().Add(10 * time.Minute)
}

// GetAgeInMinutes returns how many minutes ago the balance was last updated
func (ub *UserBalance) GetAgeInMinutes() float64 {
	return time.Since(ub.LastUpdated).Minutes()
}

// GetRemainingCacheTime returns how much time is left before cache expires
func (ub *UserBalance) GetRemainingCacheTime() time.Duration {
	if ub.IsExpired() {
		return 0
	}
	return time.Until(ub.CacheValidUntil)
}

// Validate performs basic validation on the UserBalance
func (ub *UserBalance) Validate() error {
	if ub.WalletID <= 0 {
		return NewValidationError("wallet_id", "must be positive")
	}
	if ub.SOLBalance < 0 {
		return NewValidationError("sol_balance", "cannot be negative")
	}
	return nil
}
