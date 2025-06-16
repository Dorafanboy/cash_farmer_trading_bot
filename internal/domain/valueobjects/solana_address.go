package valueobjects

import (
	"errors"
	"regexp"
)

// SolanaAddress represents Solana wallet address
type SolanaAddress struct {
	value string
}

// NewSolanaAddress creates new SolanaAddress with validation
func NewSolanaAddress(address string) (*SolanaAddress, error) {
	if !isValidSolanaAddress(address) {
		return nil, errors.New("invalid solana address format")
	}

	return &SolanaAddress{value: address}, nil
}

// String returns string representation of address
func (sa SolanaAddress) String() string {
	return sa.value
}

// Value returns address value
func (sa SolanaAddress) Value() string {
	return sa.value
}

// IsValid checks if address is valid
func (sa SolanaAddress) IsValid() bool {
	return isValidSolanaAddress(sa.value)
}

// Equals compares two addresses
func (sa SolanaAddress) Equals(other SolanaAddress) bool {
	return sa.value == other.value
}

// isValidSolanaAddress validates Solana address format
// Solana address: base58, length 32-44 characters
func isValidSolanaAddress(address string) bool {
	if len(address) < 32 || len(address) > 44 {
		return false
	}

	// Check if it's base58 (only specific characters)
	base58Regex := regexp.MustCompile(`^[123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz]+$`)
	return base58Regex.MatchString(address)
}
