package test_files

import (
	"context"
	"testing"

	"cash-farmer/internal/application/services"
	"cash-farmer/internal/infrastructure/api"
)

// TestDebugTokenSwapService проверяет debug логи GetTokenSwapService
func TestDebugTokenSwapService(t *testing.T) {
	t.Log("=== Тестируем ServiceContainerAdapter ===")

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

	// Получаем TokenSwapService - должны увидеть DEBUG логи
	tokenSwapService := simpleServiceContainer.GetTokenSwapService()

	if tokenSwapService == nil {
		t.Fatal("TokenSwapService не должен быть nil")
	}

	// Тестируем swap
	swapRequest := services.SwapRequest{
		UserID:         810859639,
		InputMint:      "So11111111111111111111111111111111111111112",
		OutputMint:     "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
		AmountLamports: 1000000,
		UserPublicKey:  "11111111111111111111111111111111",
	}

	ctx := context.Background()
	result, err := tokenSwapService.ExecuteTokenSwap(ctx, swapRequest, nil)

	if err == nil && result != nil && result.ExecutionMethod == "demo" {
		t.Error("❌ Все еще DEMO режим!")
	} else {
		t.Log("✅ Работает в реальном режиме")
	}
}

// TestDebugSimpleContainer проверяет demo режим
func TestDebugSimpleContainer(t *testing.T) {
	t.Log("=== Тестируем простой SimpleServiceContainer ===")

	// Создаем простой контейнер
	simpleContainer, err := services.NewSimpleServiceContainer()
	if err != nil {
		t.Fatalf("Не удалось создать SimpleServiceContainer: %v", err)
	}
	defer simpleContainer.Close()

	// Получаем TokenSwapService - должны увидеть DEBUG логи
	tokenSwapService := simpleContainer.GetTokenSwapService()

	if tokenSwapService == nil {
		t.Fatal("TokenSwapService не должен быть nil")
	}

	// Тестируем swap
	swapRequest := services.SwapRequest{
		UserID:         810859639,
		InputMint:      "So11111111111111111111111111111111111111112",
		OutputMint:     "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
		AmountLamports: 1000000,
		UserPublicKey:  "11111111111111111111111111111111",
	}

	ctx := context.Background()
	result, err := tokenSwapService.ExecuteTokenSwap(ctx, swapRequest, nil)

	if err != nil {
		t.Fatalf("Ошибка в demo режиме: %v", err)
	}

	if result == nil || result.ExecutionMethod != "demo" {
		t.Error("❌ Должен быть DEMO режим!")
	} else {
		t.Log("✅ Работает в demo режиме")
	}
}
