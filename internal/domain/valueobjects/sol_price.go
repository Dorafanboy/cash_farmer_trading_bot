package valueobjects

import (
	"fmt"
	"time"

	"github.com/shopspring/decimal"
)

type SolPrice struct {
	priceUSD  decimal.Decimal
	timestamp time.Time
}

func NewSolPrice(priceUSD decimal.Decimal) *SolPrice {
	return &SolPrice{
		priceUSD:  priceUSD,
		timestamp: time.Now(),
	}
}

func NewSolPriceFromFloat(price float64) (*SolPrice, error) {
	if price < 0 {
		return nil, fmt.Errorf("price cannot be negative: %f", price)
	}

	priceDecimal := decimal.NewFromFloat(price)
	return NewSolPrice(priceDecimal), nil
}

func NewSolPriceFromString(price string) (*SolPrice, error) {
	priceDecimal, err := decimal.NewFromString(price)
	if err != nil {
		return nil, fmt.Errorf("invalid price format: %s", price)
	}

	if priceDecimal.IsNegative() {
		return nil, fmt.Errorf("price cannot be negative: %s", price)
	}

	return NewSolPrice(priceDecimal), nil
}

func (s *SolPrice) PriceUSD() decimal.Decimal {
	return s.priceUSD
}

func (s *SolPrice) Timestamp() time.Time {
	return s.timestamp
}

func (s *SolPrice) IsExpired(ttl time.Duration) bool {
	return time.Since(s.timestamp) > ttl
}

func (s *SolPrice) String() string {
	return fmt.Sprintf("$%s", s.priceUSD.StringFixed(6))
}

func (s *SolPrice) Equals(other *SolPrice) bool {
	if other == nil {
		return false
	}
	return s.priceUSD.Equal(other.priceUSD)
}
