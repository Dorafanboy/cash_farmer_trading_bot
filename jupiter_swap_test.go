package main

import (
	"context"
	"fmt"
	"strconv"
	"testing"
	"time"

	"cash-farmer/internal/infrastructure/api"
)

// TestJupiterSwapDiagnosis - TDD подход для диагностики проблем Jupiter swap
func TestJupiterSwapDiagnosis(t *testing.T) {
	// Настройка клиентов
	jupiterClient := api.NewJupiterClient("https://lite-api.jup.ag")
	solanaClient := api.NewSolanaClient("https://api.mainnet-beta.solana.com")
	ctx := context.Background()

	// Тестовые данные
	testWallet := "2o9PRCfeAow3LMK6FjV1GMiM4DaKU62QLUoFPrUpWN2J"

	tests := []struct {
		name       string
		inputMint  string
		outputMint string
		amount     string
		expectPass bool
		reason     string
	}{
		{
			name:       "USDC_Stable_Small",
			inputMint:  "So11111111111111111111111111111111111111112",  // SOL
			outputMint: "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v", // USDC
			amount:     "100000",                                       // 0.0001 SOL
			expectPass: true,
			reason:     "USDC стабильный, минимальная сумма",
		},
		{
			name:       "BONK_Medium_Amount",
			inputMint:  "So11111111111111111111111111111111111111112",  // SOL
			outputMint: "DezXAZ8z7PnrnRJjz3wXBoRgixCa6xjnB7YaB1pPB263", // BONK
			amount:     "500000",                                       // 0.0005 SOL
			expectPass: true,
			reason:     "BONK популярный, средняя сумма",
		},
		{
			name:       "BONK_Original_Amount",
			inputMint:  "So11111111111111111111111111111111111111112",  // SOL
			outputMint: "DezXAZ8z7PnrnRJjz3wXBoRgixCa6xjnB7YaB1pPB263", // BONK
			amount:     "1000000",                                      // 0.001 SOL (оригинальная проблемная сумма)
			expectPass: false,
			reason:     "Оригинальная проблемная сумма",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fmt.Printf("\n🧪 ТЕСТ: %s (%s)\n", tt.name, tt.reason)
			fmt.Printf("💰 Сумма: %s lamports\n", tt.amount)

			// 1. Проверяем баланс пользователя
			balance, err := solanaClient.GetBalance(ctx, testWallet)
			if err != nil {
				t.Fatalf("❌ Ошибка получения баланса: %v", err)
			}
			fmt.Printf("💳 Баланс пользователя: %d lamports (%.6f SOL)\n", balance, float64(balance)/1e9)

			// 2. Тестируем quote с нашими исправлениями
			quoteReq := api.JupiterQuoteRequest{
				InputMint:                  tt.inputMint,
				OutputMint:                 tt.outputMint,
				Amount:                     tt.amount,
				SlippageBps:                1500,
				SwapMode:                   "ExactIn",
				OnlyDirectRoutes:           false,
				AsLegacyTransaction:        false,
				RestrictIntermediateTokens: true, // 🔥 НАШЕ ИСПРАВЛЕНИЕ!
			}

			fmt.Printf("🔍 Запрашиваем quote с RestrictIntermediateTokens=true...\n")
			quote, err := jupiterClient.GetQuote(ctx, quoteReq)
			if err != nil {
				t.Fatalf("❌ Ошибка quote: %v", err)
			}

			fmt.Printf("✅ Quote получен:\n")
			fmt.Printf("   - Input: %s lamports\n", quote.InAmount)
			fmt.Printf("   - Output: %s tokens\n", quote.OutAmount)
			fmt.Printf("   - Slippage: %d bps\n", quote.SlippageBps)
			fmt.Printf("   - Price Impact: %s%%\n", quote.PriceImpactPct)

			if len(quote.RoutePlan) > 0 {
				fmt.Printf("   - AMM: %s (%s)\n", quote.RoutePlan[0].SwapInfo.Label, quote.RoutePlan[0].SwapInfo.AmmKey)
			}

			// 3. Тестируем swap transaction с исправлениями
			swapReq := api.JupiterSwapRequest{
				QuoteResponse:            *quote,
				UserPublicKey:            testWallet,
				WrapAndUnwrapSol:         false, // 🔥 НАШЕ ИСПРАВЛЕНИЕ!
				UseSharedAccounts:        true,  // 🔥 НАШЕ ИСПРАВЛЕНИЕ!
				DynamicComputeUnitLimit:  true,  // 🔥 НАШЕ ИСПРАВЛЕНИЕ!
				DynamicSlippage:          true,  // 🔥 НАШЕ ИСПРАВЛЕНИЕ!
				SkipUserAccountsRpcCalls: false,
				AsLegacyTransaction:      false,
				CreateTokenAccount:       true,
				PrioritizationFeeLamports: map[string]interface{}{
					"priorityLevelWithMaxLamports": map[string]interface{}{
						"maxLamports":   50000, // Низкие fees для теста
						"priorityLevel": "medium",
					},
				},
			}

			fmt.Printf("🔥 Создаем swap transaction с НАШИМИ ИСПРАВЛЕНИЯМИ...\n")
			swapResp, err := jupiterClient.GetSwapTransaction(ctx, swapReq)
			if err != nil {
				t.Fatalf("❌ Ошибка swap transaction: %v", err)
			}

			// 4. Анализируем ответ Jupiter
			fmt.Printf("📊 АНАЛИЗ JUPITER ОТВЕТА:\n")
			fmt.Printf("   - Transaction size: %d chars\n", len(swapResp.SwapTransaction))
			fmt.Printf("   - Priority fee: %d lamports\n", swapResp.PrioritizationFeeLamports)
			fmt.Printf("   - Compute limit: %d CU\n", swapResp.ComputeUnitLimit)

			// 5. Проверяем simulationError
			hasSimulationError := swapResp.SimulationError != nil
			fmt.Printf("   - Simulation Error: %t\n", hasSimulationError)

			if hasSimulationError {
				fmt.Printf("     ❌ Error: %s\n", swapResp.SimulationError.Error)
				fmt.Printf("     ❌ Code: %s\n", swapResp.SimulationError.ErrorCode)
			}

			// 6. Расчет реальной стоимости транзакции
			swapAmount, _ := strconv.ParseInt(tt.amount, 10, 64)
			totalCost := swapAmount + int64(swapResp.PrioritizationFeeLamports) + 5000 // +5000 за base fee

			fmt.Printf("💸 РАСЧЕТ СТОИМОСТИ:\n")
			fmt.Printf("   - Swap amount: %d lamports (%.6f SOL)\n", swapAmount, float64(swapAmount)/1e9)
			fmt.Printf("   - Priority fee: %d lamports (%.6f SOL)\n", swapResp.PrioritizationFeeLamports, float64(swapResp.PrioritizationFeeLamports)/1e9)
			fmt.Printf("   - Base fee: ~5000 lamports (%.6f SOL)\n", 5000.0/1e9)
			fmt.Printf("   - TOTAL: %d lamports (%.6f SOL)\n", totalCost, float64(totalCost)/1e9)
			fmt.Printf("   - Доступно: %d lamports (%.6f SOL)\n", balance, float64(balance)/1e9)
			fmt.Printf("   - Остаток: %d lamports (%.6f SOL)\n", balance-totalCost, float64(balance-totalCost)/1e9)

			// 7. Проверяем достаточность баланса
			hasEnoughBalance := balance >= totalCost
			fmt.Printf("   - Достаточно баланса: %t\n", hasEnoughBalance)

			// 8. Анализ результата
			fmt.Printf("\n🎯 РЕЗУЛЬТАТ ТЕСТА:\n")
			if !hasSimulationError && hasEnoughBalance {
				fmt.Printf("   ✅ ДОЛЖЕН РАБОТАТЬ: Нет simulation error, достаточно баланса\n")
				if !tt.expectPass {
					t.Errorf("🚨 НЕОЖИДАННО УСПЕШНО: Ожидался провал, но тест прошел")
				}
			} else {
				fmt.Printf("   ❌ ПРОБЛЕМА НАЙДЕНА:\n")
				if hasSimulationError {
					fmt.Printf("      - Jupiter simulation error (проблема в токене/маршруте)\n")
				}
				if !hasEnoughBalance {
					fmt.Printf("      - Недостаточно баланса (проблема в расчете стоимости)\n")
				}
				if tt.expectPass {
					t.Errorf("🚨 НЕОЖИДАННО ПРОВАЛИЛСЯ: Ожидался успех, но есть проблемы")
				}
			}

			// 9. Дополнительная диагностика для проваленных тестов
			if hasSimulationError {
				fmt.Printf("\n🔬 ДИАГНОСТИКА SIMULATION ERROR:\n")
				// Попробуем с минимальными параметрами
				minimalReq := api.JupiterSwapRequest{
					QuoteResponse:      *quote,
					UserPublicKey:      testWallet,
					WrapAndUnwrapSol:   false,
					UseSharedAccounts:  false, // Отключаем shared accounts
					DynamicSlippage:    false, // Отключаем dynamic slippage
					CreateTokenAccount: false, // Отключаем auto-create
				}

				fmt.Printf("   🧪 Пробуем минимальные параметры...\n")
				minimalResp, err := jupiterClient.GetSwapTransaction(ctx, minimalReq)
				if err == nil && minimalResp.SimulationError == nil {
					fmt.Printf("   ✅ МИНИМАЛЬНЫЕ ПАРАМЕТРЫ РАБОТАЮТ! Проблема в наших 'улучшениях'\n")
				} else {
					fmt.Printf("   ❌ Даже минимальные параметры не работают - проблема в токене\n")
				}
			}

			time.Sleep(1 * time.Second) // Избегаем rate limiting
		})
	}
}

// TestBalanceCalculation - тест правильности расчета баланса
func TestBalanceCalculation(t *testing.T) {
	fmt.Printf("\n💰 ТЕСТ РАСЧЕТА БАЛАНСА\n")

	// Реальные данные Solana (должны быть копейки, не доллары!)
	realWorldCosts := map[string]int64{
		"base_fee":          5000,    // ~$0.000005
		"priority_fee_low":  10000,   // ~$0.00001
		"priority_fee_med":  50000,   // ~$0.00005
		"priority_fee_high": 100000,  // ~$0.0001
		"ata_creation":      2039280, // ~$0.002 (одноразово)
	}

	solPrice := 100.0 // $100 за SOL для расчета

	fmt.Printf("📊 РЕАЛЬНЫЕ СТОИМОСТИ SOLANA (при SOL = $%.0f):\n", solPrice)
	for name, lamports := range realWorldCosts {
		dollarCost := float64(lamports) / 1e9 * solPrice
		fmt.Printf("   - %s: %d lamports = $%.6f\n", name, lamports, dollarCost)
	}

	// Тест нашего свапа
	swapAmount := int64(1000000) // 0.001 SOL
	maxReasonableCost := swapAmount + realWorldCosts["ata_creation"] + realWorldCosts["priority_fee_med"] + realWorldCosts["base_fee"]

	userBalance := int64(5000000) // 0.005 SOL у пользователя

	fmt.Printf("\n🧮 РАСЧЕТ ДЛЯ НАШЕГО СВАПА:\n")
	fmt.Printf("   - Swap: %d lamports = $%.6f\n", swapAmount, float64(swapAmount)/1e9*solPrice)
	fmt.Printf("   - Max costs: %d lamports = $%.6f\n", maxReasonableCost-swapAmount, float64(maxReasonableCost-swapAmount)/1e9*solPrice)
	fmt.Printf("   - TOTAL max: %d lamports = $%.6f\n", maxReasonableCost, float64(maxReasonableCost)/1e9*solPrice)
	fmt.Printf("   - User has: %d lamports = $%.6f\n", userBalance, float64(userBalance)/1e9*solPrice)
	fmt.Printf("   - Remaining: %d lamports = $%.6f\n", userBalance-maxReasonableCost, float64(userBalance-maxReasonableCost)/1e9*solPrice)

	if userBalance >= maxReasonableCost {
		fmt.Printf("   ✅ БАЛАНСА БОЛЕЕ ЧЕМ ДОСТАТОЧНО!\n")
	} else {
		fmt.Printf("   ❌ Недостаточно баланса\n")
		t.Errorf("Баланс должен быть достаточным для копеечных транзакций Solana!")
	}
}

// TestJupiterParameterImpact - тест влияния наших параметров
func TestJupiterParameterImpact(t *testing.T) {
	fmt.Printf("\n🔬 ТЕСТ ВЛИЯНИЯ ПАРАМЕТРОВ JUPITER\n")

	jupiterClient := api.NewJupiterClient("https://lite-api.jup.ag")
	ctx := context.Background()

	// Базовый quote
	quoteReq := api.JupiterQuoteRequest{
		InputMint:   "So11111111111111111111111111111111111111112",  // SOL
		OutputMint:  "DezXAZ8z7PnrnRJjz3wXBoRgixCa6xjnB7YaB1pPB263", // BONK
		Amount:      "500000",                                       // 0.0005 SOL
		SlippageBps: 1500,
		SwapMode:    "ExactIn",
	}

	quote, err := jupiterClient.GetQuote(ctx, quoteReq)
	if err != nil {
		t.Fatalf("Ошибка quote: %v", err)
	}

	// Параметры для тестирования
	testConfigs := []struct {
		name               string
		wrapAndUnwrapSol   bool
		useSharedAccounts  bool
		dynamicSlippage    bool
		createTokenAccount bool
	}{
		{"Original_Bad", true, false, false, false},
		{"Our_Fix", false, true, true, true},
		{"Minimal", false, false, false, false},
		{"Hybrid", false, true, false, true},
	}

	testWallet := "2o9PRCfeAow3LMK6FjV1GMiM4DaKU62QLUoFPrUpWN2J"

	for _, config := range testConfigs {
		fmt.Printf("\n🧪 Конфигурация: %s\n", config.name)

		swapReq := api.JupiterSwapRequest{
			QuoteResponse:      *quote,
			UserPublicKey:      testWallet,
			WrapAndUnwrapSol:   config.wrapAndUnwrapSol,
			UseSharedAccounts:  config.useSharedAccounts,
			DynamicSlippage:    config.dynamicSlippage,
			CreateTokenAccount: config.createTokenAccount,
		}

		swapResp, err := jupiterClient.GetSwapTransaction(ctx, swapReq)
		if err != nil {
			fmt.Printf("   ❌ Ошибка: %v\n", err)
			continue
		}

		hasError := swapResp.SimulationError != nil
		fmt.Printf("   - WrapSol=%t, Shared=%t, DynSlip=%t, CreateATA=%t\n",
			config.wrapAndUnwrapSol, config.useSharedAccounts, config.dynamicSlippage, config.createTokenAccount)
		fmt.Printf("   - Simulation Error: %t\n", hasError)
		fmt.Printf("   - Priority Fee: %d lamports\n", swapResp.PrioritizationFeeLamports)

		if hasError {
			fmt.Printf("   - Error: %s\n", swapResp.SimulationError.Error)
		}

		time.Sleep(500 * time.Millisecond)
	}
}
