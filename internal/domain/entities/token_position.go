package entities

import (
	"time"
)

// TokenPosition represents a token position in user's portfolio
type TokenPosition struct {
	ID           int64     `json:"id"`
	WalletID     int64     `json:"wallet_id"`
	TokenAddress string    `json:"token_address"`
	TokenSymbol  string    `json:"token_symbol"`
	Amount       float64   `json:"amount"`
	EntryPrice   float64   `json:"entry_price"`
	CurrentPrice *float64  `json:"current_price,omitempty"`
	PnLUSD       *float64  `json:"pnl_usd,omitempty"`
	OpenedAt     time.Time `json:"opened_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	IsActive     bool      `json:"is_active"`
}

// NewTokenPosition creates a new token position
func NewTokenPosition(walletID int64, tokenAddress, tokenSymbol string, amount, entryPrice float64) *TokenPosition {
	now := time.Now()
	return &TokenPosition{
		WalletID:     walletID,
		TokenAddress: tokenAddress,
		TokenSymbol:  tokenSymbol,
		Amount:       amount,
		EntryPrice:   entryPrice,
		OpenedAt:     now,
		UpdatedAt:    now,
		IsActive:     true,
	}
}

// UpdatePrice updates the current price and calculates PnL
func (tp *TokenPosition) UpdatePrice(newPrice float64) {
	tp.CurrentPrice = &newPrice
	tp.UpdatedAt = time.Now()

	// Calculate PnL: (current_price - entry_price) * amount
	pnl := (newPrice - tp.EntryPrice) * tp.Amount
	tp.PnLUSD = &pnl
}

// UpdateAmount updates the position amount (for partial sells/buys)
func (tp *TokenPosition) UpdateAmount(newAmount float64) {
	tp.Amount = newAmount
	tp.UpdatedAt = time.Now()

	// Recalculate PnL with new amount
	if tp.CurrentPrice != nil {
		pnl := (*tp.CurrentPrice - tp.EntryPrice) * tp.Amount
		tp.PnLUSD = &pnl
	}
}

// Close closes the position
func (tp *TokenPosition) Close() {
	tp.IsActive = false
	tp.UpdatedAt = time.Now()
}

// GetPnLPercentage returns PnL as percentage of entry price
func (tp *TokenPosition) GetPnLPercentage() *float64 {
	if tp.CurrentPrice == nil {
		return nil
	}

	percentage := ((*tp.CurrentPrice - tp.EntryPrice) / tp.EntryPrice) * 100
	return &percentage
}

// GetCurrentValue returns current USD value of the position
func (tp *TokenPosition) GetCurrentValue() *float64 {
	if tp.CurrentPrice == nil {
		return nil
	}

	value := *tp.CurrentPrice * tp.Amount
	return &value
}

// GetEntryValue returns entry USD value of the position
func (tp *TokenPosition) GetEntryValue() float64 {
	return tp.EntryPrice * tp.Amount
}

// IsProfit checks if position is currently profitable
func (tp *TokenPosition) IsProfit() bool {
	return tp.PnLUSD != nil && *tp.PnLUSD > 0
}

// IsLoss checks if position is currently losing
func (tp *TokenPosition) IsLoss() bool {
	return tp.PnLUSD != nil && *tp.PnLUSD < 0
}

// GetAge returns how long the position has been open
func (tp *TokenPosition) GetAge() time.Duration {
	return time.Since(tp.OpenedAt)
}

// GetTimeSinceUpdate returns how long since last price update
func (tp *TokenPosition) GetTimeSinceUpdate() time.Duration {
	return time.Since(tp.UpdatedAt)
}

// IsStale checks if price data is older than 5 minutes
func (tp *TokenPosition) IsStale() bool {
	return tp.GetTimeSinceUpdate() > 5*time.Minute
}

// Validate performs basic validation on the TokenPosition
func (tp *TokenPosition) Validate() error {
	if tp.WalletID <= 0 {
		return NewValidationError("wallet_id", "must be positive")
	}
	if tp.TokenAddress == "" {
		return NewValidationError("token_address", "cannot be empty")
	}
	if len(tp.TokenAddress) < 32 || len(tp.TokenAddress) > 44 {
		return NewValidationError("token_address", "must be between 32 and 44 characters")
	}
	if tp.Amount <= 0 {
		return NewValidationError("amount", "must be positive")
	}
	if tp.EntryPrice <= 0 {
		return NewValidationError("entry_price", "must be positive")
	}
	if tp.CurrentPrice != nil && *tp.CurrentPrice <= 0 {
		return NewValidationError("current_price", "must be positive when set")
	}
	return nil
}
