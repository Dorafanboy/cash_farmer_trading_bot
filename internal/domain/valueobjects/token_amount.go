package valueobjects

import (
	"errors"
	"fmt"
	"math"
)

// TokenAmount represents token amount in lamports (minimal SOL unit)
type TokenAmount struct {
	lamports uint64
}

const (
	// LamportsPerSOL amount of lamports in one SOL
	LamportsPerSOL = 1_000_000_000 // 1e9
)

// NewTokenAmountFromLamports creates TokenAmount from lamports
func NewTokenAmountFromLamports(lamports uint64) TokenAmount {
	return TokenAmount{lamports: lamports}
}

// NewTokenAmountFromSOL creates TokenAmount from SOL
func NewTokenAmountFromSOL(sol float64) (TokenAmount, error) {
	if sol < 0 {
		return TokenAmount{}, errors.New("amount cannot be negative")
	}

	if sol > float64(math.MaxUint64)/LamportsPerSOL {
		return TokenAmount{}, errors.New("amount too large")
	}

	lamports := uint64(sol * LamportsPerSOL)
	return TokenAmount{lamports: lamports}, nil
}

// Lamports returns amount in lamports
func (ta TokenAmount) Lamports() uint64 {
	return ta.lamports
}

// SOL returns amount in SOL as float64
func (ta TokenAmount) SOL() float64 {
	return float64(ta.lamports) / LamportsPerSOL
}

// String returns string representation in SOL
func (ta TokenAmount) String() string {
	return fmt.Sprintf("%.9f SOL", ta.SOL())
}

// FormatSOL formats SOL amount with specified precision
func (ta TokenAmount) FormatSOL(precision int) string {
	format := fmt.Sprintf("%%.%df SOL", precision)
	return fmt.Sprintf(format, ta.SOL())
}

// Add adds two token amounts
func (ta TokenAmount) Add(other TokenAmount) (TokenAmount, error) {
	if ta.lamports > math.MaxUint64-other.lamports {
		return TokenAmount{}, errors.New("addition overflow")
	}

	return TokenAmount{lamports: ta.lamports + other.lamports}, nil
}

// Subtract subtracts token amount
func (ta TokenAmount) Subtract(other TokenAmount) (TokenAmount, error) {
	if ta.lamports < other.lamports {
		return TokenAmount{}, errors.New("insufficient funds")
	}

	return TokenAmount{lamports: ta.lamports - other.lamports}, nil
}

// IsZero checks if amount equals zero
func (ta TokenAmount) IsZero() bool {
	return ta.lamports == 0
}

// IsGreaterThan compares with another amount
func (ta TokenAmount) IsGreaterThan(other TokenAmount) bool {
	return ta.lamports > other.lamports
}

// IsLessThan compares with another amount
func (ta TokenAmount) IsLessThan(other TokenAmount) bool {
	return ta.lamports < other.lamports
}

// Equals compares two amounts for equality
func (ta TokenAmount) Equals(other TokenAmount) bool {
	return ta.lamports == other.lamports
}
