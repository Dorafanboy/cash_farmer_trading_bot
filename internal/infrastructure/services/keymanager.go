package services

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
)

// KeyManager handles encryption/decryption operations for sensitive data
type KeyManager struct {
	masterKey []byte
}

// EncryptedData represents encrypted data with its nonce
type EncryptedData struct {
	Data  []byte
	Nonce []byte
}

// NewKeyManager creates a new KeyManager instance
func NewKeyManager() (*KeyManager, error) {
	masterKeyHex := os.Getenv("CASH_FARMER_MASTER_KEY")
	if masterKeyHex == "" {
		return nil, fmt.Errorf("CASH_FARMER_MASTER_KEY environment variable is required")
	}

	masterKey, err := hex.DecodeString(masterKeyHex)
	if err != nil {
		return nil, fmt.Errorf("invalid master key format: %w", err)
	}

	if len(masterKey) != 32 {
		return nil, fmt.Errorf("master key must be 32 bytes (64 hex characters), got %d bytes", len(masterKey))
	}

	return &KeyManager{
		masterKey: masterKey,
	}, nil
}

// deriveUserKey derives a user-specific encryption key using HMAC-SHA256
func (km *KeyManager) deriveUserKey(userID int64, version int) []byte {
	h := hmac.New(sha256.New, km.masterKey)
	keyData := fmt.Sprintf("user_%d_v%d", userID, version)
	h.Write([]byte(keyData))
	return h.Sum(nil)
}

// Encrypt encrypts data using AES-256-GCM with a user-specific derived key
func (km *KeyManager) Encrypt(userID int64, plaintext []byte) (*EncryptedData, error) {
	// Derive user-specific key (version 1)
	derivedKey := km.deriveUserKey(userID, 1)

	// Create AES cipher
	block, err := aes.NewCipher(derivedKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	// Generate random nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Encrypt data
	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)

	return &EncryptedData{
		Data:  ciphertext,
		Nonce: nonce,
	}, nil
}

// Decrypt decrypts data using AES-256-GCM with a user-specific derived key
func (km *KeyManager) Decrypt(userID int64, encryptedData *EncryptedData) ([]byte, error) {
	return km.DecryptWithVersion(userID, encryptedData, 1)
}

// DecryptWithVersion decrypts data using a specific key version (for key rotation support)
func (km *KeyManager) DecryptWithVersion(userID int64, encryptedData *EncryptedData, version int) ([]byte, error) {
	// Derive user-specific key with version
	derivedKey := km.deriveUserKey(userID, version)

	// Create AES cipher
	block, err := aes.NewCipher(derivedKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	// Decrypt data
	plaintext, err := gcm.Open(nil, encryptedData.Nonce, encryptedData.Data, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt data: %w", err)
	}

	return plaintext, nil
}

// EncryptPrivateKey encrypts a private key for storage
func (km *KeyManager) EncryptPrivateKey(userID int64, privateKey string) (*EncryptedData, error) {
	return km.Encrypt(userID, []byte(privateKey))
}

// DecryptPrivateKey decrypts a private key from storage
func (km *KeyManager) DecryptPrivateKey(userID int64, encryptedData *EncryptedData) (string, error) {
	plaintext, err := km.Decrypt(userID, encryptedData)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

// EncryptMnemonic encrypts a mnemonic phrase for storage
func (km *KeyManager) EncryptMnemonic(userID int64, mnemonic string) (*EncryptedData, error) {
	return km.Encrypt(userID, []byte(mnemonic))
}

// DecryptMnemonic decrypts a mnemonic phrase from storage
func (km *KeyManager) DecryptMnemonic(userID int64, encryptedData *EncryptedData) (string, error) {
	plaintext, err := km.Decrypt(userID, encryptedData)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

// ValidateMasterKey validates that the master key is properly configured
func (km *KeyManager) ValidateMasterKey() error {
	if len(km.masterKey) != 32 {
		return fmt.Errorf("invalid master key length: expected 32 bytes, got %d", len(km.masterKey))
	}
	return nil
}

// GenerateMasterKey generates a new 32-byte master key (for setup purposes)
func GenerateMasterKey() (string, error) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return "", fmt.Errorf("failed to generate random key: %w", err)
	}
	return hex.EncodeToString(key), nil
}
