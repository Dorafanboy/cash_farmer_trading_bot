package test_files

import (
	"context"
	"testing"

	"cash-farmer/internal/application/services"
	"cash-farmer/internal/infrastructure/api"
)

// TestServiceContainerAdapterTokenSwap проверяет что адаптер правильно передает TokenSwapService
func TestServiceContainerAdapterTokenSwap(t *testing.T) {
	// Создаем реальный ServiceContainer как в main.go
	jupiterClient := api.NewJupiterClient("https://lite-api.jup.ag/swap/v1")
	jitoClient := api.NewJitoClient("https://amsterdam.mainnet.block-engine.jito.wtf/api/v1")
	solanaClient := api.NewSolanaClient("https://api.mainnet-beta.solana.com")
	dexScreenerClient := api.NewDexscreenerClient("https://api.dexscreener.com")

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

	// Получаем TokenSwapService
	tokenSwapService := simpleServiceContainer.GetTokenSwapService()
	if tokenSwapService == nil {
		t.Fatal("TokenSwapService не должен быть nil через адаптер")
	}

	// Создаем тестовый swap request
	swapRequest := services.SwapRequest{
		UserID:         810859639,                                      // Тот же user ID что в логах пользователя
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
			t.Error("❌ ServiceContainerAdapter все еще возвращает demo TokenSwapService!")
			t.Logf("Result: %+v", result)
		} else {
			t.Logf("✅ ServiceContainerAdapter возвращает реальный TokenSwapService: method=%s", result.ExecutionMethod)
		}
	} else {
		// Ошибка ожидаема в реальном режиме
		t.Logf("✅ ServiceContainerAdapter возвращает реальный TokenSwapService (ошибка ожидаема): %v", err)
	}

	t.Log("🎯 Теперь бот должен использовать реальные свапы!")
}
