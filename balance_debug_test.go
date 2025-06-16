package main

import (
	"context"
	"fmt"
	"strconv"
	"testing"

	"cash-farmer/internal/infrastructure/api"
)

// TestRealBalanceDebug - проверяем реальный баланс кошелька
func TestRealBalanceDebug(t *testing.T) {
	solanaClient := api.NewSolanaClient("https://api.mainnet-beta.solana.com")
	jupiterClient := api.NewJupiterClient("https://lite-api.jup.ag")
	ctx := context.Background()

	userWallet := "2o9PRCfeAow3LMK6FjV1GMiM4DaKU62QLUoFPrUpWN2J"

	fmt.Printf("\n🔍 ДИАГНОСТИКА РЕАЛЬНОГО БАЛАНСА\n")

	// 1. Проверяем реальный баланс
	balance, err := solanaClient.GetBalance(ctx, userWallet)
	if err != nil {
		t.Fatalf("❌ Ошибка получения баланса: %v", err)
	}

	fmt.Printf("💳 РЕАЛЬНЫЙ БАЛАНС: %d lamports (%.6f SOL)\n", balance, float64(balance)/1e9)

	// 2. Проверяем что за сумма была в логах
	problemAmount := int64(1000000) // 0.001 SOL из логов
	fmt.Printf("💸 СУММА СВАПА: %d lamports (%.6f SOL)\n", problemAmount, float64(problemAmount)/1e9)

	// 3. Рассчитываем нужную сумму для USDC ATA
	usdcATACost := int64(2039280) // Стандартная стоимость ATA
	txFee := int64(5000)          // Базовая комиссия
	safetyBuffer := int64(100000) // Буфер безопасности

	totalNeeded := problemAmount + usdcATACost + txFee + safetyBuffer
	fmt.Printf("💰 ОБЩАЯ ПОТРЕБНОСТЬ:\n")
	fmt.Printf("   - Swap: %d lamports\n", problemAmount)
	fmt.Printf("   - USDC ATA: %d lamports\n", usdcATACost)
	fmt.Printf("   - TX fee: %d lamports\n", txFee)
	fmt.Printf("   - Buffer: %d lamports\n", safetyBuffer)
	fmt.Printf("   - ИТОГО: %d lamports (%.6f SOL)\n", totalNeeded, float64(totalNeeded)/1e9)

	fmt.Printf("\n📊 СРАВНЕНИЕ:\n")
	fmt.Printf("   - ЕСТЬ: %.6f SOL\n", float64(balance)/1e9)
	fmt.Printf("   - НУЖНО: %.6f SOL\n", float64(totalNeeded)/1e9)
	fmt.Printf("   - РАЗНИЦА: %.6f SOL\n", float64(balance-totalNeeded)/1e9)

	if balance < totalNeeded {
		shortage := totalNeeded - balance
		fmt.Printf("   ❌ НЕ ХВАТАЕТ: %d lamports (%.6f SOL)\n", shortage, float64(shortage)/1e9)
		fmt.Printf("\n💡 РЕШЕНИЯ:\n")
		fmt.Printf("   1. Пополнить кошелек на %.6f SOL\n", float64(shortage+100000)/1e9)
		fmt.Printf("   2. Уменьшить сумму свапа до %d lamports\n", balance-usdcATACost-txFee-safetyBuffer)
		fmt.Printf("   3. Создать USDC ATA заранее\n")
	} else {
		fmt.Printf("   ✅ БАЛАНСА ДОСТАТОЧНО\n")
	}

	// 4. Тестируем минимальную сумму
	maxSafeAmount := balance - usdcATACost - txFee - safetyBuffer
	if maxSafeAmount > 0 {
		fmt.Printf("\n🧪 ТЕСТИРУЕМ МИНИМАЛЬНУЮ СУММУ: %d lamports\n", maxSafeAmount)

		// Создаем quote для минимальной суммы
		minQuoteReq := api.JupiterQuoteRequest{
			InputMint:                  "So11111111111111111111111111111111111111112",  // SOL
			OutputMint:                 "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v", // USDC
			Amount:                     strconv.FormatInt(maxSafeAmount, 10),
			SlippageBps:                1500,
			SwapMode:                   "ExactIn",
			RestrictIntermediateTokens: true,
		}

		quote, err := jupiterClient.GetQuote(ctx, minQuoteReq)
		if err != nil {
			fmt.Printf("❌ Quote ошибка: %v\n", err)
			return
		}

		fmt.Printf("✅ Quote успешен для минимальной суммы\n")
		fmt.Printf("   - Input: %s lamports\n", quote.InAmount)
		fmt.Printf("   - Output: %s USDC\n", quote.OutAmount)

		// Тестируем минимальные параметры
		minSwapReq := api.JupiterSwapRequest{
			QuoteResponse:    *quote,
			UserPublicKey:    userWallet,
			WrapAndUnwrapSol: false, // TDD исправление
		}

		swapResp, err := jupiterClient.GetSwapTransaction(ctx, minSwapReq)
		if err != nil {
			fmt.Printf("❌ Swap ошибка: %v\n", err)
		} else {
			hasError := swapResp.SimulationError != nil
			fmt.Printf("   - Simulation Error: %t\n", hasError)
			fmt.Printf("   - Priority Fee: %d lamports\n", swapResp.PrioritizationFeeLamports)
			if hasError {
				fmt.Printf("   - Error: %s\n", swapResp.SimulationError.Error)
				fmt.Printf("\n🚨 ДАЖЕ МИНИМАЛЬНАЯ СУММА НЕ РАБОТАЕТ!\n")
				fmt.Printf("💡 ВОЗМОЖНЫЕ ПРИЧИНЫ:\n")
				fmt.Printf("   1. USDC ATA не существует - нужно создать заранее\n")
				fmt.Printf("   2. Кошелек заблокирован или проблемы с сетью\n")
				fmt.Printf("   3. Нужно использовать другой RPC endpoint\n")
			} else {
				fmt.Printf("\n🎉 МИНИМАЛЬНАЯ СУММА РАБОТАЕТ!\n")
				fmt.Printf("💡 РЕКОМЕНДАЦИЯ: Используй максимум %d lamports для свапа\n", maxSafeAmount)
			}
		}
	} else {
		fmt.Printf("\n❌ БАЛАНС СЛИШКОМ НИЗКИЙ ДЛЯ ЛЮБЫХ СВАПОВ\n")
		fmt.Printf("💡 НУЖНО ПОПОЛНИТЬ КОШЕЛЕК НА %.6f SOL\n", float64(totalNeeded-balance+100000)/1e9)
	}
}
