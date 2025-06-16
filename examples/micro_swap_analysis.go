package main

import (
	"fmt"
	"strings"
)

// 🔍 MICRO SWAP ANALYZER
// Показывает пользователю почему маленькие свапы дорогие

const (
	ATACreationCost = 2039280 // lamports (0.002039 SOL)
	SOLToUSD        = 150     // примерный курс
	SafetyBuffer    = 100000  // оптимизированный буфер
)

type SwapAnalyzer struct {
	swapAmount        int64
	currentSOLBalance int64
	hasUSDCATA        bool
	priorityFeesLevel string
}

func NewSwapAnalyzer(swapAmount int64, balance int64, hasUSDCATA bool) *SwapAnalyzer {
	return &SwapAnalyzer{
		swapAmount:        swapAmount,
		currentSOLBalance: balance,
		hasUSDCATA:        hasUSDCATA,
		priorityFeesLevel: "medium",
	}
}

func (sa *SwapAnalyzer) CalculateCosts() map[string]int64 {
	costs := make(map[string]int64)

	// 1. Основной swap
	costs["swap"] = sa.swapAmount

	// 2. ATA costs (если нужно создать)
	if !sa.hasUSDCATA {
		costs["usdc_ata"] = ATACreationCost
	}

	// 3. Priority fees (оптимизированные)
	switch sa.priorityFeesLevel {
	case "high":
		costs["priority_fees"] = 150000
	case "medium":
		costs["priority_fees"] = 75000 // 50% экономия
	case "low":
		costs["priority_fees"] = 30000 // 80% экономия
	}

	// 4. Base transaction fees
	costs["base_fees"] = 25000

	// 5. Safety buffer (оптимизированный)
	costs["safety_buffer"] = SafetyBuffer

	// 6. Slippage buffer (1% от суммы)
	costs["slippage"] = sa.swapAmount / 100

	return costs
}

func (sa *SwapAnalyzer) GetTotalCost() int64 {
	costs := sa.CalculateCosts()
	var total int64
	for _, cost := range costs {
		total += cost
	}
	return total
}

func (sa *SwapAnalyzer) PrintAnalysis() {
	fmt.Println(strings.Repeat("=", 70))
	fmt.Println("🔍 АНАЛИЗ: Почему маленький swap стоит дорого?")
	fmt.Println(strings.Repeat("=", 70))

	costs := sa.CalculateCosts()

	fmt.Printf("💰 СУММА СВАПА: %.6f SOL ($%.2f)\n",
		float64(sa.swapAmount)/1e9,
		float64(sa.swapAmount)/1e9*SOLToUSD)

	fmt.Println("\n📊 ДЕТАЛИЗАЦИЯ РАСХОДОВ:")

	totalFeeCost := int64(0)
	for key, cost := range costs {
		if cost > 0 && key != "swap" {
			usdCost := float64(cost) / 1e9 * SOLToUSD
			fmt.Printf("  %-15s: %.6f SOL ($%.3f)\n", key, float64(cost)/1e9, usdCost)
			totalFeeCost += cost
		}
	}

	fmt.Printf("\n💸 ИТОГО КОМИССИЙ: %.6f SOL ($%.2f)\n",
		float64(totalFeeCost)/1e9,
		float64(totalFeeCost)/1e9*SOLToUSD)

	totalCost := sa.GetTotalCost()
	fmt.Printf("💳 ИТОГО К СПИСАНИЮ: %.6f SOL ($%.2f)\n",
		float64(totalCost)/1e9,
		float64(totalCost)/1e9*SOLToUSD)

	// Процент комиссии от суммы свапа
	feePercentage := float64(totalFeeCost) / float64(sa.swapAmount) * 100
	fmt.Printf("📈 КОМИССИЯ ОТ СУММЫ СВАПА: %.0f%%\n", feePercentage)

	// Проблема и решения
	fmt.Println("\n🚨 ГЛАВНАЯ ПРОБЛЕМА:")
	if !sa.hasUSDCATA {
		fmt.Printf("  USDC ATA создается ОДИН РАЗ за $%.2f\n", float64(ATACreationCost)/1e9*SOLToUSD)
		fmt.Println("  После создания все следующие свапы будут стоить копейки!")
	}

	fmt.Println("\n🎯 РЕКОМЕНДАЦИИ:")

	if feePercentage > 100 {
		fmt.Println("  ❌ SWAP НЕ ВЫГОДЕН: Комиссия больше суммы свапа!")
		fmt.Println("  💡 УВЕЛИЧЬТЕ СУММУ: минимум 0.01 SOL ($1.50)")
	} else if feePercentage > 50 {
		fmt.Println("  ⚠️ SWAP ДОРОГОЙ: Комиссия составляет больше 50%")
		fmt.Println("  💡 ЛУЧШЕ НАКОПИТЬ: свапать от 0.005 SOL ($0.75)")
	} else if feePercentage > 20 {
		fmt.Println("  ✅ SWAP ПРИЕМЛЕМЫЙ: Комиссия в разумных пределах")
	} else {
		fmt.Println("  ✅ SWAP ВЫГОДНЫЙ: Низкая комиссия")
	}

	// Баланс-чек
	if sa.currentSOLBalance < totalCost {
		shortage := totalCost - sa.currentSOLBalance
		fmt.Printf("\n❌ НЕДОСТАТОЧНО СРЕДСТВ: Нужно ещё %.6f SOL ($%.2f)\n",
			float64(shortage)/1e9,
			float64(shortage)/1e9*SOLToUSD)

		fmt.Printf("🏦 АДРЕС ДЛЯ ПОПОЛНЕНИЯ: 6G8CFkxLVBHNE8TgSDdGW3UJNKHNBgV7TKRVgmBKF7uE\n")
	} else {
		fmt.Printf("\n✅ ДОСТАТОЧНО СРЕДСТВ: Остаток %.6f SOL\n",
			float64(sa.currentSOLBalance-totalCost)/1e9)
	}
}

func main() {
	fmt.Println("🔍 АНАЛИЗ ВАШЕГО СЛУЧАЯ:")
	fmt.Println("Swap: 0.001 SOL ($0.15)")
	fmt.Println("Баланс: 0.005 SOL ($0.75)")
	fmt.Println("USDC ATA: НЕ СОЗДАН\n")

	// Параметры пользователя
	swapAmount := int64(1000000)  // 0.001 SOL что хочет пользователь
	userBalance := int64(5000000) // 0.005 SOL (текущий баланс)
	hasUSDCATA := false           // ATA USDC не создан - ВОТ ПРОБЛЕМА!

	analyzer := NewSwapAnalyzer(swapAmount, userBalance, hasUSDCATA)
	analyzer.PrintAnalysis()

	fmt.Println("\n" + strings.Repeat("=", 70))
	fmt.Println("🎯 СЦЕНАРИИ ОПТИМИЗАЦИИ")
	fmt.Println(strings.Repeat("=", 70))

	// Сценарий 1: После создания ATA (следующие свапы)
	fmt.Println("\n🏗️ СЦЕНАРИЙ 1: После создания USDC ATA (последующие свапы)")
	analyzer1 := NewSwapAnalyzer(swapAmount, userBalance, true) // ATA уже есть
	totalCost1 := analyzer1.GetTotalCost()
	feeCost1 := totalCost1 - swapAmount
	fmt.Printf("   Комиссия: %.6f SOL ($%.2f) - всего %.0f%% от суммы!\n",
		float64(feeCost1)/1e9,
		float64(feeCost1)/1e9*SOLToUSD,
		float64(feeCost1)/float64(swapAmount)*100)

	// Сценарий 2: Увеличить сумму свапа
	fmt.Println("\n📈 СЦЕНАРИЙ 2: Увеличить сумму до 0.01 SOL ($1.50)")
	analyzer2 := NewSwapAnalyzer(10000000, 15000000, hasUSDCATA) // 0.01 SOL
	totalCost2 := analyzer2.GetTotalCost()
	feeCost2 := totalCost2 - 10000000
	fmt.Printf("   Комиссия: %.6f SOL ($%.2f) - всего %.0f%% от суммы!\n",
		float64(feeCost2)/1e9,
		float64(feeCost2)/1e9*SOLToUSD,
		float64(feeCost2)/10000000*100)

	// Сценарий 3: Optimal strategy
	fmt.Println("\n💡 ОПТИМАЛЬНАЯ СТРАТЕГИЯ:")
	fmt.Println("   1. Пополните кошелек до 0.01 SOL ($1.50)")
	fmt.Println("   2. Сделайте первый swap 0.005 SOL (создастся ATA)")
	fmt.Println("   3. Все следующие свапы будут стоить ~$0.02!")
	fmt.Println("   4. Торгуйте маленькими суммами за копейки")

	fmt.Println("\n🏦 ДЛЯ ПОПОЛНЕНИЯ КОШЕЛЬКА:")
	fmt.Println("   Адрес: 6G8CFkxLVBHNE8TgSDdGW3UJNKHNBgV7TKRVgmBKF7uE")
	fmt.Printf("   Нужно: %.3f SOL ($%.2f)\n", 0.005, 0.005*SOLToUSD)
}
