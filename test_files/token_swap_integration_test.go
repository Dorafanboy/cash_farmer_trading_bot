package test_files

import (
	"context"
	"testing"
	"time"

	"cash-farmer/internal/application/services"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTokenSwap_BuyToken тестирует покупку токена в DEMO режиме
func TestTokenSwap_BuyToken(t *testing.T) {
	// Arrange
	container, err := services.NewSimpleServiceContainer()
	require.NoError(t, err)

	tokenSwapService := container.GetTokenSwapService()
	require.NotNil(t, tokenSwapService)

	ctx := context.Background()
	userID := int64(123456789)

	// Создаем запрос на покупку токена (SOL -> TOKEN)
	swapRequest := services.SwapRequest{
		UserID:         userID,
		InputMint:      "So11111111111111111111111111111111111111112",  // SOL
		OutputMint:     "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v", // USDC
		AmountLamports: 100000000,                                      // 0.1 SOL
		UserPublicKey:  "9WzDXwBbmkg8ZTbNMqUxvQRAyrZzDsGYdLVL9zYtAWWM",
		CustomSlippage: func() *int { s := 1500; return &s }(), // 15% slippage
	}

	userSettings := &services.UserSwapSettings{
		UserID:               userID,
		PreferredPresetID:    "balanced",
		AutoRetryEnabled:     true,
		MEVProtectionEnabled: false,
	}

	// Act
	result, err := tokenSwapService.ExecuteTokenSwap(ctx, swapRequest, userSettings)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.Success, "Buy swap should succeed in demo mode")
	assert.Equal(t, "demo", result.ExecutionMethod, "Should use demo execution method")
	assert.Equal(t, "DEMO_SUCCESS", result.FinalStatus)
	assert.Equal(t, "balanced", result.PresetUsed)
	assert.Contains(t, result.TransactionID, "demo_tx_")
	assert.Greater(t, result.ExecutionTimeMs, int64(0))
	assert.False(t, result.MEVProtectionActive, "MEV protection should be disabled")
}

// TestTokenSwap_SellToken тестирует продажу токена в DEMO режиме
func TestTokenSwap_SellToken(t *testing.T) {
	// Arrange
	container, err := services.NewSimpleServiceContainer()
	require.NoError(t, err)

	tokenSwapService := container.GetTokenSwapService()
	require.NotNil(t, tokenSwapService)

	ctx := context.Background()
	userID := int64(987654321)

	// Создаем запрос на продажу токена (TOKEN -> SOL)
	swapRequest := services.SwapRequest{
		UserID:         userID,
		InputMint:      "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v", // USDC
		OutputMint:     "So11111111111111111111111111111111111111112",  // SOL
		AmountLamports: 1000000,                                        // 1 USDC worth in lamports
		UserPublicKey:  "7dHbWXmci3dT8UFYWYZweBLXgycu7Y3iL6trKn1Y7ARj",
		CustomSlippage: func() *int { s := 2000; return &s }(), // 20% slippage for selling
	}

	userSettings := &services.UserSwapSettings{
		UserID:               userID,
		PreferredPresetID:    "aggressive",
		AutoRetryEnabled:     true,
		MEVProtectionEnabled: true, // Enable MEV protection for selling
	}

	// Act
	result, err := tokenSwapService.ExecuteTokenSwap(ctx, swapRequest, userSettings)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.Success, "Sell swap should succeed in demo mode")
	assert.Equal(t, "demo", result.ExecutionMethod)
	assert.Equal(t, "DEMO_SUCCESS", result.FinalStatus)
	assert.Equal(t, "balanced", result.PresetUsed) // Should fall back to balanced if aggressive not found
	assert.Contains(t, result.TransactionID, "demo_tx_")
	assert.Greater(t, result.ExecutionTimeMs, int64(0))
}

// TestTokenSwap_DifferentPresets тестирует разные presets
func TestTokenSwap_DifferentPresets(t *testing.T) {
	container, err := services.NewSimpleServiceContainer()
	require.NoError(t, err)

	tokenSwapService := container.GetTokenSwapService()
	require.NotNil(t, tokenSwapService)

	ctx := context.Background()

	testCases := []struct {
		name     string
		presetID string
		expected string
	}{
		{"Fast preset", "fast", "balanced"}, // Should fallback to balanced
		{"Balanced preset", "balanced", "balanced"},
		{"Safe preset", "safe", "balanced"}, // Should fallback to balanced
		{"Empty preset", "", "balanced"},    // Should use default
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			swapRequest := services.SwapRequest{
				UserID:         int64(111111),
				InputMint:      "So11111111111111111111111111111111111111112",
				OutputMint:     "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
				AmountLamports: 50000000, // 0.05 SOL
				UserPublicKey:  "9WzDXwBbmkg8ZTbNMqUxvQRAyrZzDsGYdLVL9zYtAWWM",
				PresetID:       tc.presetID,
			}

			userSettings := &services.UserSwapSettings{
				UserID:               int64(111111),
				PreferredPresetID:    tc.presetID,
				AutoRetryEnabled:     false,
				MEVProtectionEnabled: false,
			}

			result, err := tokenSwapService.ExecuteTokenSwap(ctx, swapRequest, userSettings)

			require.NoError(t, err)
			assert.True(t, result.Success)
			assert.Equal(t, tc.expected, result.PresetUsed)
		})
	}
}

// TestTokenSwap_AmountValidation тестирует валидацию amounts
func TestTokenSwap_AmountValidation(t *testing.T) {
	container, err := services.NewSimpleServiceContainer()
	require.NoError(t, err)

	tokenSwapService := container.GetTokenSwapService()
	require.NotNil(t, tokenSwapService)

	ctx := context.Background()

	testCases := []struct {
		name           string
		amountLamports int64
		maxAmount      *int64
		shouldSucceed  bool
	}{
		{
			name:           "Normal amount",
			amountLamports: 100000000,                                             // 0.1 SOL
			maxAmount:      func() *int64 { v := int64(1000000000); return &v }(), // 1 SOL limit
			shouldSucceed:  true,
		},
		{
			name:           "Amount exceeds limit",
			amountLamports: 2000000000,                                            // 2 SOL
			maxAmount:      func() *int64 { v := int64(1000000000); return &v }(), // 1 SOL limit
			shouldSucceed:  false,
		},
		{
			name:           "No limit set",
			amountLamports: 5000000000, // 5 SOL
			maxAmount:      nil,
			shouldSucceed:  true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			swapRequest := services.SwapRequest{
				UserID:         int64(333333),
				InputMint:      "So11111111111111111111111111111111111111112",
				OutputMint:     "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
				AmountLamports: tc.amountLamports,
				UserPublicKey:  "9WzDXwBbmkg8ZTbNMqUxvQRAyrZzDsGYdLVL9zYtAWWM",
			}

			userSettings := &services.UserSwapSettings{
				UserID:               int64(333333),
				PreferredPresetID:    "balanced",
				AutoRetryEnabled:     true,
				MEVProtectionEnabled: false,
				MaxAmountPerSwap:     tc.maxAmount,
			}

			result, err := tokenSwapService.ExecuteTokenSwap(ctx, swapRequest, userSettings)

			if tc.shouldSucceed {
				require.NoError(t, err)
				assert.True(t, result.Success)
			} else {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "exceeds user limit")
			}
		})
	}
}

// TestTokenSwap_WhitelistValidation тестирует валидацию whitelist
func TestTokenSwap_WhitelistValidation(t *testing.T) {
	container, err := services.NewSimpleServiceContainer()
	require.NoError(t, err)

	tokenSwapService := container.GetTokenSwapService()
	require.NotNil(t, tokenSwapService)

	ctx := context.Background()

	testCases := []struct {
		name              string
		outputMint        string
		whitelistedTokens []string
		shouldSucceed     bool
	}{
		{
			name:              "Token in whitelist",
			outputMint:        "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v", // USDC
			whitelistedTokens: []string{"EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v", "Es9vMFrzaCERmJfrF4H2FYD4KCoNkY11McCe8BenwNYB"},
			shouldSucceed:     true,
		},
		{
			name:              "Token not in whitelist",
			outputMint:        "SomeRandomTokenMint123456789",
			whitelistedTokens: []string{"EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v", "Es9vMFrzaCERmJfrF4H2FYD4KCoNkY11McCe8BenwNYB"},
			shouldSucceed:     false,
		},
		{
			name:              "Empty whitelist allows all",
			outputMint:        "SomeRandomTokenMint123456789",
			whitelistedTokens: []string{},
			shouldSucceed:     true,
		},
		{
			name:              "No whitelist allows all",
			outputMint:        "SomeRandomTokenMint123456789",
			whitelistedTokens: nil,
			shouldSucceed:     true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			swapRequest := services.SwapRequest{
				UserID:         int64(444444),
				InputMint:      "So11111111111111111111111111111111111111112",
				OutputMint:     tc.outputMint,
				AmountLamports: 100000000, // 0.1 SOL
				UserPublicKey:  "9WzDXwBbmkg8ZTbNMqUxvQRAyrZzDsGYdLVL9zYtAWWM",
			}

			userSettings := &services.UserSwapSettings{
				UserID:               int64(444444),
				PreferredPresetID:    "balanced",
				AutoRetryEnabled:     true,
				MEVProtectionEnabled: false,
				WhitelistedTokens:    tc.whitelistedTokens,
			}

			result, err := tokenSwapService.ExecuteTokenSwap(ctx, swapRequest, userSettings)

			if tc.shouldSucceed {
				require.NoError(t, err)
				assert.True(t, result.Success)
			} else {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "not in whitelist")
			}
		})
	}
}

// TestTokenSwap_CustomSlippageHandling тестирует обработку custom slippage
func TestTokenSwap_CustomSlippageHandling(t *testing.T) {
	container, err := services.NewSimpleServiceContainer()
	require.NoError(t, err)

	tokenSwapService := container.GetTokenSwapService()
	require.NotNil(t, tokenSwapService)

	ctx := context.Background()

	testCases := []struct {
		name             string
		requestSlippage  *int
		userSlippage     *int
		expectedSucceeds bool
	}{
		{
			name:             "Request slippage used",
			requestSlippage:  func() *int { s := 1000; return &s }(), // 10%
			userSlippage:     func() *int { s := 2000; return &s }(), // 20%
			expectedSucceeds: true,
		},
		{
			name:             "User slippage used when request empty",
			requestSlippage:  nil,
			userSlippage:     func() *int { s := 1500; return &s }(), // 15%
			expectedSucceeds: true,
		},
		{
			name:             "Default slippage when both empty",
			requestSlippage:  nil,
			userSlippage:     nil,
			expectedSucceeds: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			swapRequest := services.SwapRequest{
				UserID:         int64(555555),
				InputMint:      "So11111111111111111111111111111111111111112",
				OutputMint:     "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
				AmountLamports: 100000000, // 0.1 SOL
				UserPublicKey:  "9WzDXwBbmkg8ZTbNMqUxvQRAyrZzDsGYdLVL9zYtAWWM",
				CustomSlippage: tc.requestSlippage,
			}

			userSettings := &services.UserSwapSettings{
				UserID:               int64(555555),
				PreferredPresetID:    "balanced",
				AutoRetryEnabled:     true,
				MEVProtectionEnabled: false,
				CustomSlippageBps:    tc.userSlippage,
			}

			result, err := tokenSwapService.ExecuteTokenSwap(ctx, swapRequest, userSettings)

			if tc.expectedSucceeds {
				require.NoError(t, err)
				assert.True(t, result.Success)
			} else {
				require.Error(t, err)
			}
		})
	}
}

// TestTokenSwap_TimingMetrics тестирует метрики времени выполнения
func TestTokenSwap_TimingMetrics(t *testing.T) {
	container, err := services.NewSimpleServiceContainer()
	require.NoError(t, err)

	tokenSwapService := container.GetTokenSwapService()
	require.NotNil(t, tokenSwapService)

	ctx := context.Background()

	swapRequest := services.SwapRequest{
		UserID:         int64(666666),
		InputMint:      "So11111111111111111111111111111111111111112",
		OutputMint:     "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
		AmountLamports: 100000000, // 0.1 SOL
		UserPublicKey:  "9WzDXwBbmkg8ZTbNMqUxvQRAyrZzDsGYdLVL9zYtAWWM",
	}

	userSettings := &services.UserSwapSettings{
		UserID:               int64(666666),
		PreferredPresetID:    "balanced",
		AutoRetryEnabled:     true,
		MEVProtectionEnabled: false,
	}

	startTime := time.Now()
	result, err := tokenSwapService.ExecuteTokenSwap(ctx, swapRequest, userSettings)
	actualTime := time.Since(startTime).Milliseconds()

	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Greater(t, result.ExecutionTimeMs, int64(0))
	assert.LessOrEqual(t, result.ExecutionTimeMs, actualTime+10) // Allow 10ms tolerance
}
