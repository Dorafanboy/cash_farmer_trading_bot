package crypto

import (
	"fmt"

	"github.com/gagliardetto/solana-go"
)

// GenerateNewSolanaKeypair generates a new Solana keypair
func GenerateNewSolanaKeypair() (solana.PrivateKey, solana.PublicKey, error) {
	privateKey, err := solana.NewRandomPrivateKey()
	if err != nil {
		return solana.PrivateKey{}, solana.PublicKey{}, fmt.Errorf("failed to generate private key: %w", err)
	}

	publicKey := privateKey.PublicKey()
	return privateKey, publicKey, nil
}

// ValidateSolanaAddress validates a Solana address format
func ValidateSolanaAddress(address string) bool {
	_, err := solana.PublicKeyFromBase58(address)
	return err == nil
}

// PublicKeyFromPrivateKey derives public key from private key string
func PublicKeyFromPrivateKey(privateKeyString string) (solana.PublicKey, error) {
	privateKey, err := solana.PrivateKeyFromBase58(privateKeyString)
	if err != nil {
		return solana.PublicKey{}, fmt.Errorf("invalid private key format: %w", err)
	}

	return privateKey.PublicKey(), nil
}
