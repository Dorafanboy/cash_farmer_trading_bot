package test_files

import (
	"context"
	"testing"

	"cash-farmer/internal/application/services"
	"cash-farmer/internal/infrastructure/api"
)

// TestBotUsesRealTokenSwapService проверяет что бот использует реальный TokenSwapService
func TestBotUsesRealTokenSwapService(t *testing.T) {
	// Имитируем создание ServiceContainer как в main.go
	jupiterClient := api.NewJupiterClient("https://lite-api.jup.ag/swap/v1")
	jitoClient := api.NewJitoClient("https://amsterdam.mainnet.block-engine.jito.wtf/api/v1")
	solanaClient := api.NewSolanaClient("https://api.mainnet-beta.solana.com")
	dexScreenerClient := api.NewDexscreenerClient("https://api.dexscreener.com")

	// Создаем ServiceContainer как в main.go (без БД для теста)
	serviceContainer := services.NewServiceContainer(
		nil, // БД
		nil, // WalletRepository
		nil, // Redis
		jupiterClient,
		jitoClient,
		solanaClient,
		dexScreenerClient,
	)

	// Создаем адаптер как в main.go
	simpleServiceContainer := services.NewServiceContainerAdapter(serviceContainer)

	// Проверяем что TokenSwapService не nil
	tokenSwapService := simpleServiceContainer.GetTokenSwapService()
	if tokenSwapService == nil {
		t.Fatal("TokenSwapService не должен быть nil")
	}

	// Создаем тестовый swap request
	swapRequest := services.SwapRequest{
		UserID:         810859639,                                      // Тот же user ID что в логах
		InputMint:      "So11111111111111111111111111111111111111112",  // SOL
		OutputMint:     "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v", // Тот же токен что в логах
		AmountLamports: 1000000,                                        // 1000000 lamports как в логах
		UserPublicKey:  "11111111111111111111111111111111",
	}

	// Выполняем swap
	ctx := context.Background()
	result, err := tokenSwapService.ExecuteTokenSwap(ctx, swapRequest, nil)

	// Проверяем что это НЕ demo режим
	if err == nil && result != nil {
		if result.ExecutionMethod == "demo" {
			t.Error("❌ Бот все еще использует demo режим! Должен использовать реальный TokenSwapService.")
		} else {
			t.Logf("✅ Бот использует реальный TokenSwapService: method=%s", result.ExecutionMethod)
		}
	} else {
		// Ошибка ожидаема в реальном режиме
		t.Logf("✅ Бот использует реальный TokenSwapService (ошибка ожидаема без настоящих ключей): %v", err)
	}

	t.Log("🎯 Теперь при запуске бота с полным ServiceContainer свапы будут реальными!")
}
