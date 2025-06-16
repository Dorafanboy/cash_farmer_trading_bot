package valueobjects

import (
	"fmt"
	"regexp"
	"strings"
)

type SolanaTokenAddress struct {
	value string
}

var (
	solanaAddressRegex = regexp.MustCompile(`^[1-9A-HJ-NP-Za-km-z]{32,44}$`)
)

func NewSolanaTokenAddress(address string) (*SolanaTokenAddress, error) {
	address = strings.TrimSpace(address)

	if address == "" {
		return nil, fmt.Errorf("token address cannot be empty")
	}

	if len(address) < 32 || len(address) > 44 {
		return nil, fmt.Errorf("invalid Solana address length: %d", len(address))
	}

	if !solanaAddressRegex.MatchString(address) {
		return nil, fmt.Errorf("invalid Solana address format: %s", address)
	}

	return &SolanaTokenAddress{
		value: address,
	}, nil
}

func (s *SolanaTokenAddress) String() string {
	return s.value
}

func (s *SolanaTokenAddress) Value() string {
	return s.value
}

func (s *SolanaTokenAddress) Equals(other *SolanaTokenAddress) bool {
	if other == nil {
		return false
	}
	return s.value == other.value
}

func (s *SolanaTokenAddress) ShortString() string {
	if len(s.value) < 8 {
		return s.value
	}
	return fmt.Sprintf("%s...%s", s.value[:4], s.value[len(s.value)-4:])
}
