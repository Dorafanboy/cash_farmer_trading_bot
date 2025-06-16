package services

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"

	"cash-farmer/internal/infrastructure/api"
	"cash-farmer/internal/infrastructure/blockchain/jito"
)

// TokenSwapService orchestrates token swaps with enhanced preset management
type TokenSwapService struct {
	transactionManager *TransactionManager
	jitoClient         jito.JitoClientInterface
}

// SwapPreset defines predefined swap configurations for users
type SwapPreset struct {
	ID                    string  `json:"id"`
	Name                  string  `json:"name"`
	Description           string  `json:"description"`
	DefaultSlippageBps    int     `json:"default_slippage_bps"`    // Default slippage in basis points
	MaxSlippageBps        int     `json:"max_slippage_bps"`        // Maximum allowed slippage
	UseBundleOnly         bool    `json:"use_bundle_only"`         // Enable bundle-only mode for MEV protection
	JitoTipLamports       *int64  `json:"jito_tip_lamports"`       // Custom Jito tip amount
	PriorityFeeMultiplier float64 `json:"priority_fee_multiplier"` // Priority fee multiplier
	MaxRetries            int     `json:"max_retries"`             // Max retry attempts
	TimeoutSeconds        int     `json:"timeout_seconds"`         // Transaction timeout
}

// UserSwapSettings represents user's personalized swap settings
type UserSwapSettings struct {
	UserID               int64                  `json:"user_id"`
	PreferredPresetID    string                 `json:"preferred_preset_id"`
	CustomSlippageBps    *int                   `json:"custom_slippage_bps,omitempty"`
	AutoRetryEnabled     bool                   `json:"auto_retry_enabled"`
	MEVProtectionEnabled bool                   `json:"mev_protection_enabled"`
	MaxAmountPerSwap     *int64                 `json:"max_amount_per_swap,omitempty"` // in lamports
	WhitelistedTokens    []string               `json:"whitelisted_tokens,omitempty"`
	CustomPresets        map[string]*SwapPreset `json:"custom_presets,omitempty"`
}

// SwapRequest represents a token swap request with user settings
type SwapRequest struct {
	UserID         int64  `json:"user_id"`
	WalletID       int64  `json:"wallet_id"`                 // User's wallet ID for signing
	InputMint      string `json:"input_mint"`                // Token to sell
	OutputMint     string `json:"output_mint"`               // Token to buy
	AmountLamports int64  `json:"amount_lamports"`           // Amount in lamports
	UserPublicKey  string `json:"user_public_key"`           // User's wallet public key
	PresetID       string `json:"preset_id,omitempty"`       // Optional preset ID
	CustomSlippage *int   `json:"custom_slippage,omitempty"` // Override slippage
	MEVProtection  *bool  `json:"mev_protection,omitempty"`  // Override MEV protection
}

// SwapResult represents the result of a token swap
type SwapResult struct {
	Success             bool                      `json:"success"`
	TransactionID       string                    `json:"transaction_id,omitempty"`
	BundleID            string                    `json:"bundle_id,omitempty"`
	ExecutionMethod     string                    `json:"execution_method"` // "jito", "solana", "bundle-only"
	Quote               *api.JupiterQuoteResponse `json:"quote,omitempty"`
	ExecutionTimeMs     int64                     `json:"execution_time_ms"`
	FinalStatus         string                    `json:"final_status"`
	Error               string                    `json:"error,omitempty"`
	PresetUsed          string                    `json:"preset_used,omitempty"`
	MEVProtectionActive bool                      `json:"mev_protection_active"`
	JitoTipUsed         *int64                    `json:"jito_tip_used,omitempty"`
}

// NewTokenSwapService creates a new token swap service
func NewTokenSwapService(transactionManager *TransactionManager, jitoClient jito.JitoClientInterface) *TokenSwapService {
	return &TokenSwapService{
		transactionManager: transactionManager,
		jitoClient:         jitoClient,
	}
}

// GetDefaultPresets returns the default swap presets available to users
func (s *TokenSwapService) GetDefaultPresets() map[string]*SwapPreset {
	return map[string]*SwapPreset{
		"conservative": {
			ID:                    "conservative",
			Name:                  "Консервативный",
			Description:           "Низкий риск, минимальные комиссии, максимальная безопасность",
			DefaultSlippageBps:    100, // 1%
			MaxSlippageBps:        300, // 3%
			UseBundleOnly:         false,
			JitoTipLamports:       nil, // No tip
			PriorityFeeMultiplier: 1.0,
			MaxRetries:            2,
			TimeoutSeconds:        45,
		},
		"balanced": {
			ID:                    "balanced",
			Name:                  "Сбалансированный",
			Description:           "Оптимальный баланс скорости и безопасности",
			DefaultSlippageBps:    200,  // 2%
			MaxSlippageBps:        1500, // 15% - INCREASED for volatile tokens
			UseBundleOnly:         true,
			JitoTipLamports:       func() *int64 { v := int64(10000); return &v }(), // 0.00001 SOL tip
			PriorityFeeMultiplier: 1.2,
			MaxRetries:            3,
			TimeoutSeconds:        30,
		},
		"aggressive": {
			ID:                    "aggressive",
			Name:                  "Агрессивный",
			Description:           "Максимальная скорость, MEV защита, высокие комиссии",
			DefaultSlippageBps:    300,  // 3%
			MaxSlippageBps:        1000, // 10%
			UseBundleOnly:         true,
			JitoTipLamports:       func() *int64 { v := int64(50000); return &v }(), // 0.00005 SOL tip
			PriorityFeeMultiplier: 2.0,
			MaxRetries:            5,
			TimeoutSeconds:        20,
		},
		"mev_protected": {
			ID:                    "mev_protected",
			Name:                  "MEV Защищенный",
			Description:           "Максимальная MEV защита через bundle-only режим",
			DefaultSlippageBps:    250, // 2.5%
			MaxSlippageBps:        500, // 5%
			UseBundleOnly:         true,
			JitoTipLamports:       func() *int64 { v := int64(25000); return &v }(), // 0.000025 SOL tip
			PriorityFeeMultiplier: 1.5,
			MaxRetries:            4,
			TimeoutSeconds:        25,
		},
		"fast_execution": {
			ID:                    "fast_execution",
			Name:                  "Быстрое Исполнение",
			Description:           "Приоритет на скорость исполнения сделок",
			DefaultSlippageBps:    400,                                              // 4%
			MaxSlippageBps:        800,                                              // 8%
			UseBundleOnly:         false,                                            // Faster than bundle-only
			JitoTipLamports:       func() *int64 { v := int64(75000); return &v }(), // 0.000075 SOL tip
			PriorityFeeMultiplier: 2.5,
			MaxRetries:            3,
			TimeoutSeconds:        15,
		},
	}
}

// ExecuteTokenSwap executes a token swap with user settings and MEV protection
func (s *TokenSwapService) ExecuteTokenSwap(ctx context.Context, req SwapRequest, userSettings *UserSwapSettings) (*SwapResult, error) {
	startTime := time.Now()

	log.Printf("🔄 Starting token swap: %s → %s, amount: %d lamports, user: %d",
		req.InputMint, req.OutputMint, req.AmountLamports, req.UserID)

	result := &SwapResult{
		Success:             false,
		MEVProtectionActive: false,
	}

	// DEMO MODE: If no transaction manager, return mock success
	if s.transactionManager == nil {
		log.Printf("🎭 DEMO MODE: TokenSwapService running without TransactionManager")
		result.Success = true
		result.TransactionID = "demo_tx_" + fmt.Sprintf("%d", time.Now().Unix())
		result.ExecutionMethod = "demo"
		result.FinalStatus = "DEMO_SUCCESS"
		result.ExecutionTimeMs = time.Since(startTime).Milliseconds()
		result.PresetUsed = "balanced"

		log.Printf("✅ DEMO Swap completed: success=%v, method=%s, time=%dms",
			result.Success, result.ExecutionMethod, result.ExecutionTimeMs)

		return result, nil
	}

	// Step 1: Determine swap preset
	preset, err := s.determineSwapPreset(req, userSettings)
	if err != nil {
		result.Error = fmt.Sprintf("preset selection failed: %v", err)
		result.ExecutionTimeMs = time.Since(startTime).Milliseconds()
		return result, err
	}

	result.PresetUsed = preset.ID
	log.Printf("📋 Using preset: %s (%s)", preset.Name, preset.ID)

	// Step 2: Validate swap request against user settings
	if err := s.validateSwapRequest(req, userSettings, preset); err != nil {
		result.Error = fmt.Sprintf("validation failed: %v", err)
		result.ExecutionTimeMs = time.Since(startTime).Milliseconds()
		return result, err
	}

	// Step 3: Determine final slippage
	slippageBps := preset.DefaultSlippageBps
	if req.CustomSlippage != nil {
		slippageBps = *req.CustomSlippage
	} else if userSettings != nil && userSettings.CustomSlippageBps != nil {
		slippageBps = *userSettings.CustomSlippageBps
	}

	// Ensure slippage doesn't exceed preset maximum
	if slippageBps > preset.MaxSlippageBps {
		slippageBps = preset.MaxSlippageBps
		log.Printf("⚠️ Slippage capped at %d bps (preset maximum)", slippageBps)
	}

	// Step 4: Determine MEV protection settings
	useMEVProtection := preset.UseBundleOnly
	if req.MEVProtection != nil {
		useMEVProtection = *req.MEVProtection
	} else if userSettings != nil && userSettings.MEVProtectionEnabled {
		useMEVProtection = true
	}

	result.MEVProtectionActive = useMEVProtection

	// Step 5: Build transaction request
	tradeReq := TradeRequest{
		InputMint:     req.InputMint,
		OutputMint:    req.OutputMint,
		Amount:        strconv.FormatInt(req.AmountLamports, 10),
		SlippageBps:   slippageBps,
		UserPublicKey: req.UserPublicKey,
		UserID:        req.UserID,   // Передаем UserID для получения private key
		WalletID:      req.WalletID, // Передаем WalletID для получения private key
		SwapMode:      "ExactIn",    // Default swap mode
	}

	// Step 6: Configure transaction manager for this swap
	originalConfig := s.transactionManager.GetConfig()
	swapConfig := s.buildSwapConfig(preset, useMEVProtection)
	s.transactionManager.UpdateConfig(swapConfig)

	// Restore original config after swap
	defer s.transactionManager.UpdateConfig(originalConfig)

	// Step 7: Execute the swap
	log.Printf("🚀 Executing swap with MEV protection: %v, slippage: %d bps", useMEVProtection, slippageBps)

	tradeResult, err := s.transactionManager.ExecuteTrade(ctx, tradeReq)
	if err != nil {
		result.Error = fmt.Sprintf("trade execution failed: %v", err)
		result.ExecutionTimeMs = time.Since(startTime).Milliseconds()
		return result, err
	}

	// Step 8: Map trade result to swap result
	result.Success = tradeResult.Success
	result.TransactionID = tradeResult.TransactionID
	result.BundleID = tradeResult.BundleID
	result.ExecutionMethod = tradeResult.Method
	result.Quote = tradeResult.Quote
	result.FinalStatus = tradeResult.FinalStatus
	result.ExecutionTimeMs = time.Since(startTime).Milliseconds()

	if preset.JitoTipLamports != nil {
		result.JitoTipUsed = preset.JitoTipLamports
	}

	if !result.Success {
		result.Error = tradeResult.Error
	}

	log.Printf("✅ Swap completed: success=%v, method=%s, time=%dms",
		result.Success, result.ExecutionMethod, result.ExecutionTimeMs)

	return result, nil
}

// determineSwapPreset selects the appropriate swap preset based on request and user settings
func (s *TokenSwapService) determineSwapPreset(req SwapRequest, userSettings *UserSwapSettings) (*SwapPreset, error) {
	defaultPresets := s.GetDefaultPresets()

	// Use preset from request if specified
	if req.PresetID != "" {
		if preset, exists := defaultPresets[req.PresetID]; exists {
			return preset, nil
		}

		// Check user custom presets
		if userSettings != nil && userSettings.CustomPresets != nil {
			if customPreset, exists := userSettings.CustomPresets[req.PresetID]; exists {
				return customPreset, nil
			}
		}

		return nil, fmt.Errorf("preset '%s' not found", req.PresetID)
	}

	// Use user's preferred preset
	if userSettings != nil && userSettings.PreferredPresetID != "" {
		if preset, exists := defaultPresets[userSettings.PreferredPresetID]; exists {
			return preset, nil
		}

		if userSettings.CustomPresets != nil {
			if customPreset, exists := userSettings.CustomPresets[userSettings.PreferredPresetID]; exists {
				return customPreset, nil
			}
		}
	}

	// Default to balanced preset
	return defaultPresets["balanced"], nil
}

// validateSwapRequest validates the swap request against user settings and limits
func (s *TokenSwapService) validateSwapRequest(req SwapRequest, userSettings *UserSwapSettings, preset *SwapPreset) error {
	// Validate amount limits
	if userSettings != nil && userSettings.MaxAmountPerSwap != nil {
		if req.AmountLamports > *userSettings.MaxAmountPerSwap {
			return fmt.Errorf("amount %d exceeds user limit %d lamports", req.AmountLamports, *userSettings.MaxAmountPerSwap)
		}
	}

	// Validate whitelisted tokens
	if userSettings != nil && len(userSettings.WhitelistedTokens) > 0 {
		outputAllowed := false
		for _, token := range userSettings.WhitelistedTokens {
			if token == req.OutputMint {
				outputAllowed = true
				break
			}
		}
		if !outputAllowed {
			return fmt.Errorf("token %s is not in whitelist", req.OutputMint)
		}
	}

	return nil
}

// buildSwapConfig creates transaction config based on swap preset
func (s *TokenSwapService) buildSwapConfig(preset *SwapPreset, useMEVProtection bool) *TransactionConfig {
	config := &TransactionConfig{
		UseJitoPrimary:        useMEVProtection,
		JitoTimeoutSeconds:    preset.TimeoutSeconds,
		JitoFallbackEnabled:   true, // ALWAYS enable fallback for reliability
		SolanaTimeoutSeconds:  preset.TimeoutSeconds + 15,
		MaxRetries:            preset.MaxRetries,
		MaxSlippageBps:        preset.MaxSlippageBps,
		MaxPriceImpactPercent: 5.0,
		JitoTipLamports:       preset.JitoTipLamports,
	}

	// CRITICAL: Ensure Jito tip is set when using MEV protection
	if useMEVProtection && config.JitoTipLamports == nil {
		defaultTip := int64(100000) // 0.0001 SOL default tip
		config.JitoTipLamports = &defaultTip
		log.Printf("🔧 AUTO-TIP: Set default Jito tip %d lamports for MEV protection", defaultTip)
	}

	// Calculate compute unit price based on priority fee multiplier
	if preset.PriorityFeeMultiplier > 1.0 {
		baseFee := 1000 // Base micro lamports
		adjustedFee := int(float64(baseFee) * preset.PriorityFeeMultiplier)
		config.ComputeUnitPriceMicroLamports = &adjustedFee
	}

	log.Printf("🔍 SWAP CONFIG: UseJitoPrimary=%t, JitoFallbackEnabled=%t, JitoTipLamports=%v",
		config.UseJitoPrimary, config.JitoFallbackEnabled, config.JitoTipLamports)

	return config
}
