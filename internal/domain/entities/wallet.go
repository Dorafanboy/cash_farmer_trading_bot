package entities

import (
	"errors"
	"strings"
	"time"

	"cash-farmer/internal/domain/valueobjects"
)

// Wallet represents user wallet
type Wallet struct {
	ID         int64                       `json:"id"`
	UserID     int64                       `json:"user_id"`
	Name       string                      `json:"name"`
	PublicKey  *valueobjects.SolanaAddress `json:"public_key"`
	PrivateKey string                      `json:"private_key"`
	Mnemonic   string                      `json:"mnemonic"`
	CreatedAt  time.Time                   `json:"created_at"`
	IsDefault  bool                        `json:"is_default"`
}

// NewWallet creates a new wallet
func NewWallet(userID int64, name string, publicKey *valueobjects.SolanaAddress, privateKey string, mnemonic string) (*Wallet, error) {
	if err := validateWalletName(name); err != nil {
		return nil, err
	}

	if publicKey == nil {
		return nil, errors.New("public key cannot be nil")
	}

	if strings.TrimSpace(privateKey) == "" {
		return nil, errors.New("private key cannot be empty")
	}

	return &Wallet{
		UserID:     userID,
		Name:       strings.TrimSpace(name),
		PublicKey:  publicKey,
		PrivateKey: privateKey,
		Mnemonic:   mnemonic,
		CreatedAt:  time.Now(),
		IsDefault:  false,
	}, nil
}

// IsValidForTrading checks if wallet can be used for trading
func (w *Wallet) IsValidForTrading() bool {
	return w.PublicKey != nil &&
		w.PublicKey.IsValid() &&
		strings.TrimSpace(w.PrivateKey) != ""
}

// UpdateName updates wallet name
func (w *Wallet) UpdateName(newName string) error {
	if err := validateWalletName(newName); err != nil {
		return err
	}

	w.Name = strings.TrimSpace(newName)
	return nil
}

// SetAsDefault sets wallet as default
func (w *Wallet) SetAsDefault() {
	w.IsDefault = true
}

// RemoveDefault removes default status
func (w *Wallet) RemoveDefault() {
	w.IsDefault = false
}

// HasMnemonic checks if wallet has mnemonic phrase
func (w *Wallet) HasMnemonic() bool {
	return strings.TrimSpace(w.Mnemonic) != ""
}

// GetPublicKeyString returns public key as string
func (w *Wallet) GetPublicKeyString() string {
	if w.PublicKey == nil {
		return ""
	}
	return w.PublicKey.String()
}

// Clone creates a copy of wallet without sensitive data
func (w *Wallet) Clone() *Wallet {
	return &Wallet{
		ID:        w.ID,
		UserID:    w.UserID,
		Name:      w.Name,
		PublicKey: w.PublicKey,
		CreatedAt: w.CreatedAt,
		IsDefault: w.IsDefault,
	}
}

// validateWalletName validates wallet name
func validateWalletName(name string) error {
	trimmed := strings.TrimSpace(name)

	if trimmed == "" {
		return errors.New("wallet name cannot be empty")
	}

	if len(trimmed) < 2 {
		return errors.New("wallet name must be at least 2 characters long")
	}

	if len(trimmed) > 50 {
		return errors.New("wallet name cannot be longer than 50 characters")
	}

	for _, char := range trimmed {
		if !isAllowedWalletNameChar(char) {
			return errors.New("wallet name contains invalid characters")
		}
	}

	return nil
}

// isAllowedWalletNameChar checks if character is allowed in wallet name
func isAllowedWalletNameChar(char rune) bool {
	return (char >= 'a' && char <= 'z') ||
		(char >= 'A' && char <= 'Z') ||
		(char >= '0' && char <= '9') ||
		char == ' ' ||
		char == '-' ||
		char == '_' ||
		char >= 0x0400
}
