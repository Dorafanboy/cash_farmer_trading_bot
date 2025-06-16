package services

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"

	"cash-farmer/internal/application/repositories"
	"cash-farmer/internal/domain/entities"
	"cash-farmer/internal/domain/valueobjects"
	"cash-farmer/internal/infrastructure/crypto"
	infraServices "cash-farmer/internal/infrastructure/services"
	"encoding/hex"
)

// WalletService provides business logic for wallet operations
type WalletService interface {
	CreateWallet(ctx context.Context, req CreateWalletRequest) (*entities.Wallet, error)
	GenerateWallet(ctx context.Context, userID int64, name string) (*entities.Wallet, error)
	ExportPrivateKey(ctx context.Context, userID int64, walletID int64) (string, error)
	GetWallet(ctx context.Context, userID int64, walletID int64) (*entities.Wallet, error)
	GetWallets(ctx context.Context, userID int64) ([]*entities.Wallet, error)
	GetDefaultWallet(ctx context.Context, userID int64) (*entities.Wallet, error)
	UpdateWallet(ctx context.Context, req UpdateWalletRequest) (*entities.Wallet, error)
	DeleteWallet(ctx context.Context, userID int64, walletID int64) error
	SetDefaultWallet(ctx context.Context, userID int64, walletID int64) error
	GetBalance(ctx context.Context, walletID int64) (*WalletBalanceInfo, error)
	SyncBalance(ctx context.Context, walletID int64) error
	ValidateWallet(ctx context.Context, walletID int64) (*WalletValidationResult, error)
}

// WalletServiceImpl implements WalletService
type WalletServiceImpl struct {
	walletRepo         repositories.WalletRepository
	transactionManager *TransactionManager
	keyManager         *infraServices.KeyManager
}

// SetTransactionManager sets the transaction manager (for dependency injection)
func (ws *WalletServiceImpl) SetTransactionManager(tm *TransactionManager) {
	ws.transactionManager = tm
}

// NewWalletService creates a new wallet service
func NewWalletService(
	walletRepo repositories.WalletRepository,
	transactionManager *TransactionManager,
	keyManager *infraServices.KeyManager,
) WalletService {
	return &WalletServiceImpl{
		walletRepo:         walletRepo,
		transactionManager: transactionManager,
		keyManager:         keyManager,
	}
}

// CreateWalletRequest represents request to create a new wallet
type CreateWalletRequest struct {
	UserID     int64  `json:"user_id"`
	Name       string `json:"name"`
	PublicKey  string `json:"public_key"`
	PrivateKey string `json:"private_key"`
	Mnemonic   string `json:"mnemonic,omitempty"`
	SetDefault bool   `json:"set_default,omitempty"`
}

// UpdateWalletRequest represents request to update wallet
type UpdateWalletRequest struct {
	UserID   int64  `json:"user_id"`
	WalletID int64  `json:"wallet_id"`
	Name     string `json:"name,omitempty"`
}

// WalletBalanceInfo represents wallet balance information
type WalletBalanceInfo struct {
	WalletID      int64            `json:"wallet_id"`
	SOLBalance    int64            `json:"sol_balance"`
	SOLBalanceSOL float64          `json:"sol_balance_sol"`
	TokenBalances map[string]int64 `json:"token_balances"`
	Slot          int64            `json:"slot"`
	LastUpdated   int64            `json:"last_updated"`
}

// WalletValidationResult represents wallet validation result
type WalletValidationResult struct {
	IsValid        bool   `json:"is_valid"`
	CanTrade       bool   `json:"can_trade"`
	HasBalance     bool   `json:"has_balance"`
	SOLBalance     int64  `json:"sol_balance"`
	ValidationNote string `json:"validation_note,omitempty"`
}

// CreateWallet creates a new wallet for the user
func (ws *WalletServiceImpl) CreateWallet(ctx context.Context, req CreateWalletRequest) (*entities.Wallet, error) {
	log.Printf("Creating wallet for user %d: %s", req.UserID, req.Name)

	// Validate public key
	log.Printf("Validating public key: %s", req.PublicKey)
	publicKeyAddr, err := valueobjects.NewSolanaAddress(req.PublicKey)
	if err != nil {
		log.Printf("ERROR: Invalid public key: %v", err)
		return nil, fmt.Errorf("invalid public key: %w", err)
	}
	log.Printf("Public key validated successfully")

	// TODO: Temporarily disable encryption due to compatibility issues
	// Will re-enable after fixing the database schema and migration
	/*
		// Encrypt private key before storing
		encryptedPrivateKey, err := ws.keyManager.EncryptPrivateKey(req.UserID, req.PrivateKey)
		if err != nil {
			return nil, fmt.Errorf("failed to encrypt private key: %w", err)
		}

		// Convert encrypted data to string for storage (base64 encoding)
		// TODO: Store encrypted data in proper binary format or JSON in the future
		encryptedKeyString := fmt.Sprintf("encrypted:%x:%x", encryptedPrivateKey.Data, encryptedPrivateKey.Nonce)
		log.Printf("Private key encrypted for user %d (length: %d)", req.UserID, len(encryptedKeyString))
	*/

	// For now, store private key without encryption (backward compatibility)
	log.Printf("Storing private key without encryption for user %d (temp solution)", req.UserID)

	// Create wallet entity with private key (unencrypted for now)
	log.Printf("Creating wallet entity...")
	wallet, err := entities.NewWallet(req.UserID, req.Name, publicKeyAddr, req.PrivateKey, req.Mnemonic)
	if err != nil {
		log.Printf("ERROR: Failed to create wallet entity: %v", err)
		return nil, fmt.Errorf("failed to create wallet entity: %w", err)
	}
	log.Printf("Wallet entity created successfully")

	// Check if user already has wallets
	log.Printf("Checking existing wallets for user %d", req.UserID)
	existingWallets, err := ws.walletRepo.FindByUserID(ctx, req.UserID)
	if err != nil {
		log.Printf("ERROR: Failed to check existing wallets: %v", err)
		return nil, fmt.Errorf("failed to check existing wallets: %w", err)
	}
	log.Printf("Found %d existing wallets", len(existingWallets))

	// Set as default if requested or if it's the first wallet
	if req.SetDefault || len(existingWallets) == 0 {
		log.Printf("Setting wallet as default")
		wallet.SetAsDefault()
	}

	// Save wallet
	log.Printf("Saving wallet to database...")
	if err := ws.walletRepo.Save(ctx, wallet); err != nil {
		log.Printf("ERROR: Failed to save wallet: %v", err)
		return nil, fmt.Errorf("failed to save wallet: %w", err)
	}
	log.Printf("Wallet saved to database successfully")

	// If setting as default, update other wallets
	if wallet.IsDefault {
		log.Printf("Updating default wallet status...")
		if err := ws.walletRepo.SetAsDefault(ctx, req.UserID, wallet.ID); err != nil {
			log.Printf("WARNING: Failed to update default wallet status: %v", err)
		} else {
			log.Printf("Default wallet status updated successfully")
		}
	}

	log.Printf("Wallet created successfully: ID=%d, Name=%s, Default=%t", wallet.ID, wallet.Name, wallet.IsDefault)
	return wallet, nil
}

// GenerateWallet creates a new wallet with generated Solana keypair
func (ws *WalletServiceImpl) GenerateWallet(ctx context.Context, userID int64, name string) (*entities.Wallet, error) {
	log.Printf("Generating new wallet for user %d: %s", userID, name)

	// Generate new Solana keypair
	privateKey, publicKey, err := crypto.GenerateNewSolanaKeypair()
	if err != nil {
		return nil, fmt.Errorf("failed to generate keypair: %w", err)
	}

	// Create wallet using existing CreateWallet
	// DON'T set as default - let CreateWallet decide based on existing wallets
	req := CreateWalletRequest{
		UserID:     userID,
		Name:       name,
		PublicKey:  publicKey.String(),
		PrivateKey: privateKey.String(),
		SetDefault: false, // Let CreateWallet decide if this should be default
	}

	return ws.CreateWallet(ctx, req)
}

// ExportPrivateKey decrypts and returns private key for wallet
func (ws *WalletServiceImpl) ExportPrivateKey(ctx context.Context, userID int64, walletID int64) (string, error) {
	log.Printf("Exporting private key for wallet ID=%d, user=%d", walletID, userID)

	// Get wallet
	wallet, err := ws.GetWallet(ctx, userID, walletID)
	if err != nil {
		return "", err
	}

	// Check if private key is encrypted (starts with "encrypted:")
	if strings.HasPrefix(wallet.PrivateKey, "encrypted:") {
		log.Printf("Decrypting private key for wallet %d", walletID)

		// Parse encrypted data from storage format: "encrypted:data:nonce"
		parts := strings.Split(wallet.PrivateKey, ":")
		if len(parts) != 3 {
			return "", fmt.Errorf("invalid encrypted private key format")
		}

		// Decode hex data and nonce
		data, err := hex.DecodeString(parts[1])
		if err != nil {
			return "", fmt.Errorf("failed to decode encrypted data: %w", err)
		}

		nonce, err := hex.DecodeString(parts[2])
		if err != nil {
			return "", fmt.Errorf("failed to decode nonce: %w", err)
		}

		// Create encrypted data structure
		encryptedData := &infraServices.EncryptedData{
			Data:  data,
			Nonce: nonce,
		}

		// Decrypt using KeyManager
		decryptedKey, err := ws.keyManager.DecryptPrivateKey(userID, encryptedData)
		if err != nil {
			return "", fmt.Errorf("failed to decrypt private key: %w", err)
		}

		log.Printf("Private key decrypted successfully for wallet %d", walletID)
		return decryptedKey, nil
	}

	// For backward compatibility - if key doesn't start with "encrypted:", assume it's legacy unencrypted
	log.Printf("Returning legacy unencrypted private key for wallet %d", walletID)
	return wallet.PrivateKey, nil
}

// GetWallet retrieves a specific wallet by ID for the user
func (ws *WalletServiceImpl) GetWallet(ctx context.Context, userID int64, walletID int64) (*entities.Wallet, error) {
	log.Printf("Getting wallet ID=%d for user %d", walletID, userID)

	wallet, err := ws.walletRepo.FindByID(ctx, walletID)
	if err != nil {
		return nil, fmt.Errorf("failed to find wallet: %w", err)
	}

	// Verify wallet belongs to user
	if wallet.UserID != userID {
		return nil, errors.New("wallet does not belong to user")
	}

	return wallet, nil
}

// GetWallets retrieves all wallets for the user
func (ws *WalletServiceImpl) GetWallets(ctx context.Context, userID int64) ([]*entities.Wallet, error) {
	log.Printf("Getting all wallets for user %d", userID)

	wallets, err := ws.walletRepo.FindByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to find wallets: %w", err)
	}

	return wallets, nil
}

// GetDefaultWallet retrieves the default wallet for the user
func (ws *WalletServiceImpl) GetDefaultWallet(ctx context.Context, userID int64) (*entities.Wallet, error) {
	log.Printf("Getting default wallet for user %d", userID)

	wallet, err := ws.walletRepo.FindDefaultByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to find default wallet: %w", err)
	}

	return wallet, nil
}

// UpdateWallet updates wallet information
func (ws *WalletServiceImpl) UpdateWallet(ctx context.Context, req UpdateWalletRequest) (*entities.Wallet, error) {
	log.Printf("Updating wallet ID=%d for user %d", req.WalletID, req.UserID)

	// Get existing wallet
	wallet, err := ws.GetWallet(ctx, req.UserID, req.WalletID)
	if err != nil {
		return nil, err
	}

	// Update name if provided
	if req.Name != "" {
		if err := wallet.UpdateName(req.Name); err != nil {
			return nil, fmt.Errorf("failed to update wallet name: %w", err)
		}
	}

	// Save updated wallet
	if err := ws.walletRepo.Update(ctx, wallet); err != nil {
		return nil, fmt.Errorf("failed to update wallet: %w", err)
	}

	log.Printf("Wallet updated successfully: ID=%d, Name=%s", wallet.ID, wallet.Name)
	return wallet, nil
}

// DeleteWallet deletes a wallet for the user
func (ws *WalletServiceImpl) DeleteWallet(ctx context.Context, userID int64, walletID int64) error {
	log.Printf("Deleting wallet ID=%d for user %d", walletID, userID)

	// Verify wallet exists and belongs to user
	wallet, err := ws.GetWallet(ctx, userID, walletID)
	if err != nil {
		return err
	}

	// Check if this is the only wallet (prevent deletion)
	wallets, err := ws.walletRepo.FindByUserID(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to check wallet count: %w", err)
	}

	if len(wallets) <= 1 {
		return errors.New("cannot delete the only wallet")
	}

	// If deleting default wallet, set another as default
	if wallet.IsDefault {
		for _, w := range wallets {
			if w.ID != walletID {
				if err := ws.walletRepo.SetAsDefault(ctx, userID, w.ID); err != nil {
					return fmt.Errorf("failed to set new default wallet: %w", err)
				}
				break
			}
		}
	}

	// Delete wallet
	if err := ws.walletRepo.Delete(ctx, walletID, userID); err != nil {
		return fmt.Errorf("failed to delete wallet: %w", err)
	}

	log.Printf("Wallet deleted successfully: ID=%d", walletID)
	return nil
}

// SetDefaultWallet sets a wallet as the default for the user
func (ws *WalletServiceImpl) SetDefaultWallet(ctx context.Context, userID int64, walletID int64) error {
	log.Printf("Setting wallet ID=%d as default for user %d", walletID, userID)

	// Verify wallet exists and belongs to user
	_, err := ws.GetWallet(ctx, userID, walletID)
	if err != nil {
		return err
	}

	// Set as default
	if err := ws.walletRepo.SetAsDefault(ctx, userID, walletID); err != nil {
		return fmt.Errorf("failed to set default wallet: %w", err)
	}

	log.Printf("Default wallet set successfully: ID=%d", walletID)
	return nil
}

// GetBalance retrieves current balance for the wallet
func (ws *WalletServiceImpl) GetBalance(ctx context.Context, walletID int64) (*WalletBalanceInfo, error) {
	log.Printf("Getting balance for wallet ID=%d", walletID)

	// Get wallet to find public key
	wallet, err := ws.walletRepo.FindByID(ctx, walletID)
	if err != nil {
		return nil, fmt.Errorf("failed to find wallet: %w", err)
	}

	if wallet == nil {
		return nil, errors.New("wallet is nil")
	}

	if wallet.PublicKey == nil {
		return nil, errors.New("wallet has no public key")
	}

	// Check if transaction manager is available
	if ws.transactionManager == nil {
		log.Printf("Warning: transaction manager is nil, returning zero balance for wallet %d", walletID)
		return &WalletBalanceInfo{
			WalletID:      walletID,
			SOLBalance:    0,
			SOLBalanceSOL: 0,
			TokenBalances: make(map[string]int64),
			Slot:          0,
			LastUpdated:   0,
		}, nil
	}

	// Get balance using transaction manager
	balanceInfo, err := ws.transactionManager.GetAccountBalance(ctx, wallet.PublicKey.String(), []string{})
	if err != nil {
		log.Printf("Warning: failed to get balance from transaction manager for wallet %d: %v", walletID, err)
		// Return zero balance instead of error
		return &WalletBalanceInfo{
			WalletID:      walletID,
			SOLBalance:    0,
			SOLBalanceSOL: 0,
			TokenBalances: make(map[string]int64),
			Slot:          0,
			LastUpdated:   0,
		}, nil
	}

	// Check if balanceInfo is nil
	if balanceInfo == nil {
		log.Printf("Warning: balance info is nil for wallet %d", walletID)
		return &WalletBalanceInfo{
			WalletID:      walletID,
			SOLBalance:    0,
			SOLBalanceSOL: 0,
			TokenBalances: make(map[string]int64),
			Slot:          0,
			LastUpdated:   0,
		}, nil
	}

	// Convert lamports to SOL (1 SOL = 1,000,000,000 lamports)
	solBalance := float64(balanceInfo.SOLBalance) / 1_000_000_000

	return &WalletBalanceInfo{
		WalletID:      walletID,
		SOLBalance:    balanceInfo.SOLBalance,
		SOLBalanceSOL: solBalance,
		TokenBalances: balanceInfo.TokenBalances,
		Slot:          balanceInfo.Slot,
		LastUpdated:   balanceInfo.Slot, // Using slot as timestamp for now
	}, nil
}

// SyncBalance forces a balance sync for the wallet
func (ws *WalletServiceImpl) SyncBalance(ctx context.Context, walletID int64) error {
	log.Printf("Syncing balance for wallet ID=%d", walletID)

	// This is effectively the same as GetBalance since we're always fetching fresh data
	_, err := ws.GetBalance(ctx, walletID)
	if err != nil {
		return fmt.Errorf("failed to sync balance: %w", err)
	}

	log.Printf("Balance synced successfully for wallet ID=%d", walletID)
	return nil
}

// ValidateWallet validates wallet for trading operations
func (ws *WalletServiceImpl) ValidateWallet(ctx context.Context, walletID int64) (*WalletValidationResult, error) {
	log.Printf("Validating wallet ID=%d", walletID)

	// Get wallet
	wallet, err := ws.walletRepo.FindByID(ctx, walletID)
	if err != nil {
		return &WalletValidationResult{
			IsValid:        false,
			CanTrade:       false,
			ValidationNote: fmt.Sprintf("Wallet not found: %v", err),
		}, nil
	}

	result := &WalletValidationResult{
		IsValid:  wallet.IsValidForTrading(),
		CanTrade: false,
	}

	if !result.IsValid {
		result.ValidationNote = "Wallet is not valid for trading (missing keys or invalid format)"
		return result, nil
	}

	// Check balance
	balanceInfo, err := ws.GetBalance(ctx, walletID)
	if err != nil {
		result.ValidationNote = fmt.Sprintf("Failed to get balance: %v", err)
		return result, nil
	}

	result.SOLBalance = balanceInfo.SOLBalance
	result.HasBalance = balanceInfo.SOLBalance > 0

	// Can trade if has some SOL for transaction fees (at least 0.001 SOL = 1,000,000 lamports)
	minTradingBalance := int64(1_000_000) // 0.001 SOL
	result.CanTrade = result.HasBalance && balanceInfo.SOLBalance >= minTradingBalance

	if !result.CanTrade {
		if !result.HasBalance {
			result.ValidationNote = "Wallet has no SOL balance"
		} else {
			result.ValidationNote = fmt.Sprintf("SOL balance too low for trading (need at least 0.001 SOL, have %.6f SOL)",
				float64(balanceInfo.SOLBalance)/1_000_000_000)
		}
	} else {
		result.ValidationNote = "Wallet is valid and ready for trading"
	}

	log.Printf("Wallet validation complete: ID=%d, Valid=%t, CanTrade=%t", walletID, result.IsValid, result.CanTrade)
	return result, nil
}
