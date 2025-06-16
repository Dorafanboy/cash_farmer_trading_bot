package test_files

import (
	"testing"

	"cash-farmer/internal/application/services"
	"cash-farmer/internal/infrastructure/api"
	"cash-farmer/internal/infrastructure/blockchain/jito"
)

// TestMainGoSimulation точно имитирует код из cmd/bot/main.go
func TestMainGoSimulation(t *testing.T) {
	t.Log("=== Имитируем точный код из main.go ===")

	// Точно тот же код что в main.go строки 67-107
	solanaRPCURL := "https://mainnet.helius-rpc.com/?api-key=97f4303e-3cbd-4417-8c0f-bfdcf31e3cec"
	solanaClient := api.NewSolanaClient(solanaRPCURL)

	jupiterAPIURL := "https://lite-api.jup.ag/swap/v1"
	jupiterClient := api.NewJupiterClient(jupiterAPIURL)

	jitoRPCURL := "https://amsterdam.mainnet.block-engine.jito.wtf/api/v1"
	jitoClient := jito.NewJitoClient(jitoRPCURL)

	dexScreenerAPIURL := "https://api.dexscreener.com"
	dexScreenerClient := api.NewDexscreenerClient(dexScreenerAPIURL)

	// Initialize ServiceContainer with correct parameter order - ТОЧНО КАК В MAIN.GO
	serviceContainer := services.NewServiceContainer(
		nil,               // *sqlc.Queries
		nil,               // repositories.WalletRepository
		nil,               // redis.UniversalClient
		jupiterClient,     // *api.JupiterClient
		jitoClient,        // Legacy Jito Client
		solanaClient,      // *api.SolanaClient
		dexScreenerClient, // *api.DexscreenerClient
	)

	// Create adapter for compatibility with Telegram handlers - ТОЧНО КАК В MAIN.GO
	simpleServiceContainer := services.NewServiceContainerAdapter(serviceContainer)

	t.Log("✅ ServiceContainer создан, адаптер создан")

	// Проверяем TokenSwapService - ДОЛЖНЫ УВИДЕТЬ DEBUG ЛОГИ
	tokenSwapService := simpleServiceContainer.GetTokenSwapService()

	if tokenSwapService == nil {
		t.Fatal("TokenSwapService не должен быть nil")
	}

	t.Log("✅ Если видите 'adapter=true' и 'Using adapter.GetTokenSwapService()' - то код правильный!")
}
