package repositories

import (
	"context"

	"cash-farmer/internal/domain/valueobjects"
)

// SolanaClient defines interface for Solana blockchain operations
type SolanaClient interface {
	GetBalance(ctx context.Context, publicKey string) (*valueobjects.TokenAmount, error)
	GenerateKeypair() (publicKey string, privateKey string, mnemonic string, error error)
	ImportFromPrivateKey(privateKey string) (publicKey string, error error)
	ImportFromMnemonic(mnemonic string) (publicKey string, privateKey string, error error)
	ValidatePrivateKey(privateKey string) error
	ValidateMnemonic(mnemonic string) error
}
