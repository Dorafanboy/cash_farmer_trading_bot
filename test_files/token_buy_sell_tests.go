package test_files

import (
	"context"
	"testing"
	"time"

	"cash-farmer/internal/application/services"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBuyToken_Demo тестирует покупку токена (SOL -> TOKEN) в DEMO режиме
func TestBuyToken_Demo(t *testing.T) {
	// Arrange
	container, err := services.NewSimpleServiceContainer()
	require.NoError(t, err)

	tokenSwapService := container.GetTokenSwapService()
	require.NotNil(t, tokenSwapService)

	ctx := context.Background()
	userID := int64(123456789)

	// Создаем запрос на покупку токена (SOL -> USDC)
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

	t.Logf("✅ BUY успешно: SOL → USDC, tx: %s, время: %dms", result.TransactionID, result.ExecutionTimeMs)
}

// TestSellToken_Demo тестирует продажу токена (TOKEN -> SOL) в DEMO режиме
func TestSellToken_Demo(t *testing.T) {
	// Arrange
	container, err := services.NewSimpleServiceContainer()
	require.NoError(t, err)

	tokenSwapService := container.GetTokenSwapService()
	require.NotNil(t, tokenSwapService)

	ctx := context.Background()
	userID := int64(987654321)

	// Создаем запрос на продажу токена (USDC -> SOL)
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
		PreferredPresetID:    "balanced",
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
	assert.Equal(t, "balanced", result.PresetUsed) // Should fall back to balanced
	assert.Contains(t, result.TransactionID, "demo_tx_")
	assert.Greater(t, result.ExecutionTimeMs, int64(0))

	t.Logf("✅ SELL успешно: USDC → SOL, tx: %s, время: %dms", result.TransactionID, result.ExecutionTimeMs)
}

// TestSwapAmounts_MultipleScenarios тестирует разные суммы свапов
func TestSwapAmounts_MultipleScenarios(t *testing.T) {
	container, err := services.NewSimpleServiceContainer()
	require.NoError(t, err)

	tokenSwapService := container.GetTokenSwapService()
	require.NotNil(t, tokenSwapService)

	ctx := context.Background()

	testCases := []struct {
		name           string
		amountLamports int64
		description    string
	}{
		{"Очень маленький", 1000000, "0.001 SOL"}, // 0.001 SOL
		{"Маленький", 10000000, "0.01 SOL"},       // 0.01 SOL
		{"Обычный", 100000000, "0.1 SOL"},         // 0.1 SOL
		{"Большой", 1000000000, "1 SOL"},          // 1 SOL
		{"Очень большой", 10000000000, "10 SOL"},  // 10 SOL
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			swapRequest := services.SwapRequest{
				UserID:         int64(111111),
				InputMint:      "So11111111111111111111111111111111111111112",
				OutputMint:     "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
				AmountLamports: tc.amountLamports,
				UserPublicKey:  "9WzDXwBbmkg8ZTbNMqUxvQRAyrZzDsGYdLVL9zYtAWWM",
			}

			userSettings := &services.UserSwapSettings{
				UserID:               int64(111111),
				PreferredPresetID:    "balanced",
				AutoRetryEnabled:     true,
				MEVProtectionEnabled: false,
			}

			result, err := tokenSwapService.ExecuteTokenSwap(ctx, swapRequest, userSettings)

			require.NoError(t, err)
			assert.True(t, result.Success)
			assert.Equal(t, "demo", result.ExecutionMethod)

			t.Logf("✅ %s (%s): успешно за %dms", tc.name, tc.description, result.ExecutionTimeMs)
		})
	}
}

// TestSwapPairs_DifferentTokens тестирует разные пары токенов
func TestSwapPairs_DifferentTokens(t *testing.T) {
	container, err := services.NewSimpleServiceContainer()
	require.NoError(t, err)

	tokenSwapService := container.GetTokenSwapService()
	require.NotNil(t, tokenSwapService)

	ctx := context.Background()

	pairs := []struct {
		name       string
		inputMint  string
		outputMint string
		swapType   string
	}{
		{
			"SOL → USDC (покупка stablecoin)",
			"So11111111111111111111111111111111111111112",
			"EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
			"BUY_STABLE",
		},
		{
			"USDC → SOL (продажа stablecoin)",
			"EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
			"So11111111111111111111111111111111111111112",
			"SELL_STABLE",
		},
		{
			"SOL → Random Token (покупка alt)",
			"So11111111111111111111111111111111111111112",
			"TokenABC123456789TokenABC123456789TokenABC",
			"BUY_ALT",
		},
		{
			"Random Token → SOL (продажа alt)",
			"TokenXYZ987654321TokenXYZ987654321TokenXYZ",
			"So11111111111111111111111111111111111111112",
			"SELL_ALT",
		},
	}

	for _, pair := range pairs {
		t.Run(pair.name, func(t *testing.T) {
			swapRequest := services.SwapRequest{
				UserID:         int64(222222),
				InputMint:      pair.inputMint,
				OutputMint:     pair.outputMint,
				AmountLamports: 50000000, // 0.05 SOL equivalent
				UserPublicKey:  "9WzDXwBbmkg8ZTbNMqUxvQRAyrZzDsGYdLVL9zYtAWWM",
			}

			userSettings := &services.UserSwapSettings{
				UserID:               int64(222222),
				PreferredPresetID:    "balanced",
				AutoRetryEnabled:     true,
				MEVProtectionEnabled: false,
			}

			result, err := tokenSwapService.ExecuteTokenSwap(ctx, swapRequest, userSettings)

			require.NoError(t, err)
			assert.True(t, result.Success)
			assert.Equal(t, "demo", result.ExecutionMethod)

			t.Logf("✅ %s (%s): успешно за %dms", pair.name, pair.swapType, result.ExecutionTimeMs)
		})
	}
}

// TestSwapValidation_AmountLimits тестирует валидацию лимитов сумм
func TestSwapValidation_AmountLimits(t *testing.T) {
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
		errorMessage   string
	}{
		{
			"В пределах лимита",
			500000000, // 0.5 SOL
			func() *int64 { v := int64(1000000000); return &v }(), // 1 SOL limit
			true,
			"",
		},
		{
			"Превышает лимит",
			2000000000, // 2 SOL
			func() *int64 { v := int64(1000000000); return &v }(), // 1 SOL limit
			false,
			"exceeds user limit",
		},
		{
			"Нет лимита - разрешено всё",
			5000000000, // 5 SOL
			nil,
			true,
			"",
		},
		{
			"На границе лимита",
			1000000000, // 1 SOL exactly
			func() *int64 { v := int64(1000000000); return &v }(), // 1 SOL limit
			true,
			"",
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
				t.Logf("✅ %s: успешно", tc.name)
			} else {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.errorMessage)
				t.Logf("❌ %s: ожидаемая ошибка - %v", tc.name, err)
			}
		})
	}
}

// TestSwapValidation_TokenWhitelist тестирует валидацию whitelist токенов
func TestSwapValidation_TokenWhitelist(t *testing.T) {
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
		description       string
	}{
		{
			"Токен в whitelist",
			"EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v", // USDC
			[]string{"EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v", "TokenABC123"},
			true,
			"USDC разрешен",
		},
		{
			"Токен НЕ в whitelist",
			"ForbiddenToken123456789",
			[]string{"EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v", "TokenABC123"},
			false,
			"Запрещенный токен",
		},
		{
			"Пустой whitelist - всё разрешено",
			"AnyToken123456789",
			[]string{},
			true,
			"Пустой список",
		},
		{
			"Нет whitelist - всё разрешено",
			"AnyToken123456789",
			nil,
			true,
			"Нет ограничений",
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
				t.Logf("✅ %s (%s): успешно", tc.name, tc.description)
			} else {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "not in whitelist")
				t.Logf("❌ %s (%s): ожидаемая ошибка - whitelist", tc.name, tc.description)
			}
		})
	}
}

// TestSwapConcurrency_MultipleUsers тестирует одновременные свапы от разных пользователей
func TestSwapConcurrency_MultipleUsers(t *testing.T) {
	container, err := services.NewSimpleServiceContainer()
	require.NoError(t, err)

	tokenSwapService := container.GetTokenSwapService()
	require.NotNil(t, tokenSwapService)

	ctx := context.Background()
	numUsers := 5

	results := make(chan *services.SwapResult, numUsers)
	errors := make(chan error, numUsers)

	// Запускаем свапы от разных пользователей одновременно
	for i := 0; i < numUsers; i++ {
		go func(userID int64) {
			swapRequest := services.SwapRequest{
				UserID:         userID,
				InputMint:      "So11111111111111111111111111111111111111112",
				OutputMint:     "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
				AmountLamports: 25000000, // 0.025 SOL
				UserPublicKey:  "9WzDXwBbmkg8ZTbNMqUxvQRAyrZzDsGYdLVL9zYtAWWM",
			}

			userSettings := &services.UserSwapSettings{
				UserID:               userID,
				PreferredPresetID:    "balanced",
				AutoRetryEnabled:     true,
				MEVProtectionEnabled: false,
			}

			result, err := tokenSwapService.ExecuteTokenSwap(ctx, swapRequest, userSettings)
			if err != nil {
				errors <- err
			} else {
				results <- result
			}
		}(int64(555550 + i))
	}

	// Собираем результаты
	successCount := 0
	totalTime := int64(0)

	for i := 0; i < numUsers; i++ {
		select {
		case result := <-results:
			assert.True(t, result.Success)
			assert.Equal(t, "demo", result.ExecutionMethod)
			successCount++
			totalTime += result.ExecutionTimeMs
		case err := <-errors:
			t.Errorf("Ошибка в concurrent swap: %v", err)
		case <-time.After(5 * time.Second):
			t.Fatal("Timeout ожидания результатов concurrent swaps")
		}
	}

	assert.Equal(t, numUsers, successCount, "Все concurrent swaps должны быть успешными")
	avgTime := totalTime / int64(successCount)
	t.Logf("✅ Concurrent swaps: %d пользователей, среднее время: %dms", successCount, avgTime)
}

// TestSwapPerformance_ResponseTime тестирует производительность и время ответа
func TestSwapPerformance_ResponseTime(t *testing.T) {
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

	// Измеряем реальное время выполнения
	startTime := time.Now()
	result, err := tokenSwapService.ExecuteTokenSwap(ctx, swapRequest, userSettings)
	actualTime := time.Since(startTime).Milliseconds()

	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Greater(t, result.ExecutionTimeMs, int64(0))
	assert.LessOrEqual(t, result.ExecutionTimeMs, actualTime+100) // Allow tolerance

	// В demo режиме должно быть быстро
	assert.Less(t, result.ExecutionTimeMs, int64(100), "Demo mode should be fast")

	t.Logf("✅ Performance: reported=%dms, actual=%dms, быстро=%v",
		result.ExecutionTimeMs, actualTime, result.ExecutionTimeMs < 100)
}
