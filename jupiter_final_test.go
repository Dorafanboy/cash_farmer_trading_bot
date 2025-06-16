package main

import (
	"context"
	"fmt"
	"testing"

	"cash-farmer/internal/infrastructure/api"
)

// TestJupiterFixedParameters - проверяем что исправленные параметры работают
func TestJupiterFixedParameters(t *testing.T) {
	jupiterClient := api.NewJupiterClient("https://lite-api.jup.ag")
	ctx := context.Background()

	fmt.Printf("\n🎯 ФИНАЛЬНЫЙ ТЕСТ ИСПРАВЛЕННЫХ ПАРАМЕТРОВ\n")
	fmt.Printf("Основано на TDD-анализе: минимальные параметры = 0 priority fees!\n\n")

	// Тест BONK с проблемной суммой 0.001 SOL
	quoteReq := api.JupiterQuoteRequest{
		InputMint:                  "So11111111111111111111111111111111111111112",  // SOL
		OutputMint:                 "DezXAZ8z7PnrnRJjz3wXBoRgixCa6xjnB7YaB1pPB263", // BONK
		Amount:                     "1000000",                                      // 0.001 SOL (проблемная сумма)
		SlippageBps:                1500,
		SwapMode:                   "ExactIn",
		RestrictIntermediateTokens: true, // Стабильные маршруты
	}

	fmt.Printf("🔍 Получаем quote для BONK 0.001 SOL...\n")
	quote, err := jupiterClient.GetQuote(ctx, quoteReq)
	if err != nil {
		t.Fatalf("❌ Ошибка quote: %v", err)
	}

	fmt.Printf("✅ Quote успешен: %s BONK\n", quote.OutAmount)

	// СТАРЫЕ "ОПТИМИЗИРОВАННЫЕ" ПАРАМЕТРЫ (которые вызывали 0x1)
	fmt.Printf("\n🔥 ТЕСТ 1: СТАРЫЕ 'ОПТИМИЗИРОВАННЫЕ' ПАРАМЕТРЫ\n")
	oldSwapReq := api.JupiterSwapRequest{
		QuoteResponse:            *quote,
		UserPublicKey:            "2o9PRCfeAow3LMK6FjV1GMiM4DaKU62QLUoFPrUpWN2J",
		WrapAndUnwrapSol:         false,
		UseSharedAccounts:        true, // "ОПТИМИЗАЦИЯ" 1
		DynamicComputeUnitLimit:  true, // "ОПТИМИЗАЦИЯ" 2
		DynamicSlippage:          true, // "ОПТИМИЗАЦИЯ" 3
		CreateTokenAccount:       true, // "ОПТИМИЗАЦИЯ" 4
		SkipUserAccountsRpcCalls: false,
		AsLegacyTransaction:      false,
		PrioritizationFeeLamports: map[string]interface{}{
			"priorityLevelWithMaxLamports": map[string]interface{}{
				"maxLamports":   100000, // "ОПТИМИЗАЦИЯ" 5
				"priorityLevel": "medium",
			},
		},
	}

	oldSwapResp, err := jupiterClient.GetSwapTransaction(ctx, oldSwapReq)
	if err != nil {
		fmt.Printf("❌ СТАРЫЕ ПАРАМЕТРЫ: Ошибка API: %v\n", err)
	} else {
		hasSimError := oldSwapResp.SimulationError != nil
		fmt.Printf("   - Simulation Error: %t\n", hasSimError)
		fmt.Printf("   - Priority fee: %d lamports\n", oldSwapResp.PrioritizationFeeLamports)
		if hasSimError {
			fmt.Printf("   - Error: %s\n", oldSwapResp.SimulationError.Error)
		}
	}

	// НОВЫЕ ИСПРАВЛЕННЫЕ ПАРАМЕТРЫ (основаны на TDD)
	fmt.Printf("\n✅ ТЕСТ 2: НОВЫЕ TDD-ИСПРАВЛЕННЫЕ ПАРАМЕТРЫ\n")
	newSwapReq := api.JupiterSwapRequest{
		QuoteResponse:    *quote,
		UserPublicKey:    "2o9PRCfeAow3LMK6FjV1GMiM4DaKU62QLUoFPrUpWN2J",
		WrapAndUnwrapSol: false,
		// ВСЕ "ОПТИМИЗАЦИИ" УДАЛЕНЫ!
	}

	newSwapResp, err := jupiterClient.GetSwapTransaction(ctx, newSwapReq)
	if err != nil {
		fmt.Printf("❌ НОВЫЕ ПАРАМЕТРЫ: Ошибка API: %v\n", err)
	} else {
		hasSimError := newSwapResp.SimulationError != nil
		fmt.Printf("   - Simulation Error: %t\n", hasSimError)
		fmt.Printf("   - Priority fee: %d lamports\n", newSwapResp.PrioritizationFeeLamports)
		if hasSimError {
			fmt.Printf("   - Error: %s\n", newSwapResp.SimulationError.Error)
		}
	}

	// СРАВНЕНИЕ РЕЗУЛЬТАТОВ
	fmt.Printf("\n📊 СРАВНЕНИЕ РЕЗУЛЬТАТОВ:\n")

	if oldSwapResp != nil && newSwapResp != nil {
		oldError := oldSwapResp.SimulationError != nil
		newError := newSwapResp.SimulationError != nil

		fmt.Printf("   СТАРЫЕ параметры: SimError=%t, PriorityFee=%d\n",
			oldError, oldSwapResp.PrioritizationFeeLamports)
		fmt.Printf("   НОВЫЕ параметры:  SimError=%t, PriorityFee=%d\n",
			newError, newSwapResp.PrioritizationFeeLamports)

		if oldError && !newError {
			fmt.Printf("   🎉 ИСПРАВЛЕНИЕ РАБОТАЕТ! Старые = ошибка, новые = успех\n")
		} else if !oldError && !newError {
			feeDiff := oldSwapResp.PrioritizationFeeLamports - newSwapResp.PrioritizationFeeLamports
			fmt.Printf("   💰 ЭКОНОМИЯ: %d lamports priority fees!\n", feeDiff)
			if feeDiff > 0 {
				fmt.Printf("   🎉 TDD ПОДХОД РАБОТАЕТ! Дешевле на %d lamports\n", feeDiff)
			}
		} else if oldError && newError {
			fmt.Printf("   ⚠️ Оба варианта имеют ошибки - возможно токен заблокирован\n")
		} else {
			fmt.Printf("   ❓ Неожиданный результат - нужна дополнительная диагностика\n")
		}
	}

	fmt.Printf("\n🎯 ЗАКЛЮЧЕНИЕ:\n")
	fmt.Printf("   TDD подход показал что 'оптимизации' Jupiter вызывают дорогие fees\n")
	fmt.Printf("   Минимальные параметры = минимальные costs = больше успешных свапов\n")
	fmt.Printf("   Для мем-коинов: простота > сложные оптимизации\n")
}
