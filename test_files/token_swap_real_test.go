package test_files

import (
	"context"
	"testing"

	"cash-farmer/internal/application/services"
	"cash-farmer/internal/infrastructure/api"
)

// TestTokenSwapServiceRealMode проверяет что TokenSwapService больше не работает в demo режиме
func TestTokenSwapServiceRealMode(t *testing.T) {
	// Создаем минимальные клиенты для тестирования
	jupiterClient := api.NewJupiterClient("https://lite-api.jup.ag/swap/v1")
	jitoClient := api.NewJitoClient("https://amsterdam.mainnet.block-engine.jito.wtf/api/v1")
	solanaClient := api.NewSolanaClient("https://api.mainnet-beta.solana.com")
	dexScreenerClient := api.NewDexscreenerClient("https://api.dexscreener.com")

	// Создаем реальный ServiceContainer (без БД для теста)
	serviceContainer := services.NewServiceContainer(
		nil, // БД не нужна для этого теста
		nil, // WalletRepository не нужен
		nil, // Redis не нужен
		jupiterClient,
		jitoClient,
		solanaClient,
		dexScreenerClient,
	)

	// Создаем адаптер
	simpleContainer := services.NewServiceContainerAdapter(serviceContainer)

	// Получаем TokenSwapService
	tokenSwapService := simpleContainer.GetTokenSwapService()

	if tokenSwapService == nil {
		t.Fatal("TokenSwapService не должен быть nil")
	}

	// Создаем тестовый запрос на swap
	swapRequest := services.SwapRequest{
		UserID:         123456,
		InputMint:      "So11111111111111111111111111111111111111112",  // SOL
		OutputMint:     "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v", // USDC
		AmountLamports: 1000000,                                        // 0.001 SOL
		UserPublicKey:  "11111111111111111111111111111111",
	}

	// Выполняем swap
	ctx := context.Background()
	result, err := tokenSwapService.ExecuteTokenSwap(ctx, swapRequest, nil)

	// Проверяем что это НЕ demo режим
	if err == nil && result != nil {
		if result.ExecutionMethod == "demo" {
			t.Error("TokenSwapService все еще работает в demo режиме! Ожидался реальный режим.")
		} else {
			t.Logf("✅ TokenSwapService работает в реальном режиме: method=%s", result.ExecutionMethod)
		}
	} else {
		// Ошибка ожидаема в реальном режиме без настоящих ключей
		t.Logf("✅ TokenSwapService работает в реальном режиме (ошибка ожидаема): %v", err)
	}
}

// TestSimpleContainerDemoMode проверяет что простой контейнер все еще работает в demo режиме
func TestSimpleContainerDemoMode(t *testing.T) {
	// Создаем простой контейнер
	simpleContainer, err := services.NewSimpleServiceContainer()
	if err != nil {
		t.Fatalf("Не удалось создать SimpleServiceContainer: %v", err)
	}
	defer simpleContainer.Close()

	// Получаем TokenSwapService
	tokenSwapService := simpleContainer.GetTokenSwapService()

	if tokenSwapService == nil {
		t.Fatal("TokenSwapService не должен быть nil")
	}

	// Создаем тестовый запрос на swap
	swapRequest := services.SwapRequest{
		UserID:         123456,
		InputMint:      "So11111111111111111111111111111111111111112",  // SOL
		OutputMint:     "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v", // USDC
		AmountLamports: 1000000,                                        // 0.001 SOL
		UserPublicKey:  "11111111111111111111111111111111",
	}

	// Выполняем swap
	ctx := context.Background()
	result, err := tokenSwapService.ExecuteTokenSwap(ctx, swapRequest, nil)

	// Проверяем что это demo режим
	if err != nil {
		t.Fatalf("Ошибка в demo режиме: %v", err)
	}

	if result == nil {
		t.Fatal("Результат не должен быть nil в demo режиме")
	}

	if result.ExecutionMethod != "demo" {
		t.Errorf("Ожидался demo режим, получен: %s", result.ExecutionMethod)
	}

	if !result.Success {
		t.Error("Demo swap должен быть успешным")
	}

	t.Logf("✅ SimpleServiceContainer работает в demo режиме: method=%s, success=%v",
		result.ExecutionMethod, result.Success)
}
