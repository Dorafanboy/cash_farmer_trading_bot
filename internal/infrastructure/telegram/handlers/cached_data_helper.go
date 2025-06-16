package handlers

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"sync"
	"time"

	"cash-farmer/internal/application/services"
	"cash-farmer/internal/domain/entities"
	"cash-farmer/internal/domain/valueobjects"
	"cash-farmer/internal/infrastructure/cache"
)

// CachedDataHelper упрощает работу с кэшем в телеграм обработчиках
type CachedDataHelper struct {
	cache    *cache.TelegramCache
	services *services.SimpleServiceContainer
	mu       sync.RWMutex
}

// NewCachedDataHelper создаёт новый помощник для кэширования данных
func NewCachedDataHelper(services *services.SimpleServiceContainer) *CachedDataHelper {
	helper := &CachedDataHelper{
		cache:    cache.NewTelegramCache(),
		services: services,
	}

	// Запускаем фоновую очистку кэша
	go helper.startCleanupRoutine()
	// Запускаем фоновое обновление SOL цены
	go helper.startSOLPriceUpdater()

	log.Println("✅ CACHE HELPER: Initialized with background services")
	return helper
}

// GetWalletData получает данные кошелька с кэшем
func (cdh *CachedDataHelper) GetWalletData(ctx context.Context, userID int64) (*cache.SimpleWallet, float64, float64, error) {
	// Проверяем кэш
	userData, exists := cdh.cache.GetUserData(userID)
	if exists && cdh.cache.IsDataFresh(userData, "wallet") {
		log.Printf("✅ CACHE HIT: Wallet data for user %d", userID)
		return userData.PrimaryWallet, userData.SOLBalance, userData.USDBalance, nil
	}

	log.Printf("❌ CACHE MISS: Loading wallet data for user %d", userID)

	// Получаем данные из сервисов
	wallets, err := cdh.services.GetWalletService().GetWallets(ctx, userID)
	if err != nil {
		return nil, 0, 0, err
	}

	if len(wallets) == 0 {
		return nil, 0, 0, nil
	}

	// Находим primary кошелёк
	var primaryWallet *services.SimpleWallet
	for _, wallet := range wallets {
		if wallet.IsDefault {
			primaryWallet = &wallet
			break
		}
	}

	if primaryWallet == nil {
		primaryWallet = &wallets[0] // Берём первый если нет primary
	}

	// Получаем баланс
	balance, err := cdh.services.GetWalletService().GetBalance(ctx, primaryWallet.ID)
	if err != nil {
		return nil, 0, 0, err
	}

	// Конвертируем в SimpleWallet для кэша
	cacheWallet := &cache.SimpleWallet{
		ID:      primaryWallet.ID,     // string ID
		UserID:  primaryWallet.UserID, // int64
		Address: primaryWallet.PublicKey,
		Name:    primaryWallet.Name,
	}

	// Получаем USD баланс
	var usdBalance float64
	if solPrice, found := cdh.cache.GetCachedSOLPrice(); found {
		usdBalance = balance * solPrice
	} else {
		// Получаем свежую цену SOL
		if solPriceObj, err := cdh.getSolPrice(ctx); err == nil {
			cdh.cache.SetCachedSOLPrice(solPriceObj)
			usdBalance = balance * solPriceObj
		}
	}

	// Сохраняем в кэш
	cdh.cache.UpdateUserWallet(userID, cacheWallet, balance)

	return cacheWallet, balance, usdBalance, nil
}

// GetPnLData получает PnL данные с кэшем
func (cdh *CachedDataHelper) GetPnLData(ctx context.Context, userID int64) (string, error) {
	// Проверяем кэш
	userData, exists := cdh.cache.GetUserData(userID)
	if exists && cdh.cache.IsDataFresh(userData, "pnl") {
		log.Printf("✅ CACHE HIT: PnL data for user %d", userID)
		return userData.PnLString, nil
	}

	log.Printf("❌ CACHE MISS: Calculating PnL for user %d", userID)

	// Получаем PnL из сервисов
	pnlString := cdh.calculatePnL(ctx, userID)

	// Сохраняем в кэш
	cdh.cache.UpdateUserPnL(userID, 0, 0, pnlString) // Только строка для простоты

	return pnlString, nil
}

// GetSOLPrice получает цену SOL с кэшем
func (cdh *CachedDataHelper) GetSOLPrice(ctx context.Context) (float64, error) {
	// Проверяем кэш
	if price, found := cdh.cache.GetCachedSOLPrice(); found {
		log.Printf("✅ CACHE HIT: SOL price $%.2f", price)
		return price, nil
	}

	log.Printf("❌ CACHE MISS: Fetching SOL price")

	// Получаем цену из сервиса
	price, err := cdh.getSolPrice(ctx)
	if err != nil {
		return 0, err
	}

	// Сохраняем в кэш
	cdh.cache.SetCachedSOLPrice(price)

	return price, nil
}

// InvalidateUserCache инвалидирует кэш пользователя
func (cdh *CachedDataHelper) InvalidateUserCache(userID int64) {
	cdh.cache.InvalidateUser(userID)
	log.Printf("🗑️ CACHE INVALIDATED: User %d", userID)
}

// GetCacheStats возвращает статистику кэша
func (cdh *CachedDataHelper) GetCacheStats() map[string]interface{} {
	return cdh.cache.GetCacheStats()
}

// getSolPrice получает цену SOL из сервиса
func (cdh *CachedDataHelper) getSolPrice(ctx context.Context) (float64, error) {
	solPriceService := cdh.services.GetSolPriceService()
	if solPriceService == nil {
		return 100.0, nil // Fallback цена
	}

	solPriceObj, err := solPriceService.GetCachedSolPrice(ctx)
	if err != nil {
		return 100.0, nil // Fallback цена
	}

	if solPriceObj == nil {
		return 100.0, nil // Fallback цена
	}

	solPriceFloat, _ := solPriceObj.PriceUSD().Float64()
	return solPriceFloat, nil
}

// calculatePnL вычисляет PnL строку из сервисов
func (cdh *CachedDataHelper) calculatePnL(ctx context.Context, userID int64) string {
	// Fallback значение
	fallback := "🚀"

	// Получаем кошельки пользователя
	wallets, err := cdh.services.GetWalletService().GetWallets(ctx, userID)
	if err != nil || len(wallets) == 0 {
		return fallback
	}

	// Найти primary кошелёк
	var primaryWallet *services.SimpleWallet
	for _, wallet := range wallets {
		if wallet.IsDefault {
			primaryWallet = &wallet
			break
		}
	}

	if primaryWallet == nil {
		primaryWallet = &wallets[0]
	}

	// Парсим ID кошелька
	walletIDInt, parseErr := strconv.ParseInt(primaryWallet.ID, 10, 64)
	if parseErr != nil {
		return fallback
	}

	// Получаем PnL через PortfolioService
	portfolioService := cdh.services.GetPortfolioService()
	if portfolioService == nil {
		return fallback
	}

	pnlSummary, pnlErr := portfolioService.CalculatePnL(ctx, walletIDInt)
	if pnlErr != nil || pnlSummary == nil {
		return fallback
	}

	pnlValue := pnlSummary.TotalPnLUSD
	if pnlValue == 0 {
		return fallback
	}

	// Получаем позиции для расчета процента
	positions, posErr := portfolioService.GetPositions(ctx, walletIDInt, true)
	if posErr != nil || len(positions) == 0 {
		// Fallback без процентов
		if pnlValue > 0 {
			return fmt.Sprintf("📈 +$%.2f", pnlValue)
		} else {
			return fmt.Sprintf("📉 $%.2f", pnlValue)
		}
	}

	// Вычисляем общую стоимость входа
	var totalEntryValue float64 = 0.0
	for _, position := range positions {
		if position != nil && position.IsActive {
			totalEntryValue += position.EntryPrice * position.Amount
		}
	}

	// Вычисляем процент PnL
	var pnlPercent float64 = 0.0
	if totalEntryValue > 0 {
		pnlPercent = (pnlValue / totalEntryValue) * 100
	}

	// Форматируем результат
	if pnlValue > 0 {
		return fmt.Sprintf("📈 +$%.2f (+%.1f%%)", pnlValue, pnlPercent)
	} else {
		return fmt.Sprintf("📉 $%.2f (%.1f%%)", pnlValue, pnlPercent)
	}
}

// startCleanupRoutine запускает фоновую очистку кэша
func (cdh *CachedDataHelper) startCleanupRoutine() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		cdh.cache.CleanupExpired()
	}
}

// startSOLPriceUpdater запускает фоновое обновление цены SOL
func (cdh *CachedDataHelper) startSOLPriceUpdater() {
	ticker := time.NewTicker(45 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		ctx := context.Background()
		if price, err := cdh.getSolPrice(ctx); err == nil {
			cdh.cache.SetCachedSOLPrice(price)
			log.Printf("🔄 CACHE: SOL price updated to $%.2f", price)
		}
	}
}

// BuildOptimizedWelcomeText создаёт welcome text с использованием кэша
func (cdh *CachedDataHelper) BuildOptimizedWelcomeText(ctx context.Context, userID int64) (string, error) {
	// Получаем данные кошелька с кэшем
	wallet, balance, usdValue, err := cdh.GetWalletData(ctx, userID)
	if err != nil {
		return "", err
	}

	if wallet == nil {
		return "❌ No wallet found", nil
	}

	// Получаем PnL с кэшем
	pnlString, err := cdh.GetPnLData(ctx, userID)
	if err != nil {
		pnlString = "🚀" // Fallback
	}

	// Add deposit warning for zero balance
	var depositWarning string
	if balance == 0 {
		depositWarning = "\n\n❗️❗️<b>No SOL balance. To start trading, transfer SOL to wallet address</b>"
	}

	// Add timestamp to ensure message content is always different on refresh
	currentTime := time.Now().Format("15:04:05")

	return fmt.Sprintf(`🚀 <b>Welcome to Solana Cash Farmer Trading Bot!</b>

💳 <b>Wallet:</b>
<code>%s</code> (Tap to copy)

💰 <b>Balance:</b> %.4f SOL / $%.2f (PnL %s)

🕒 <b>Last Updated:</b> %s

👉🏻 <b>Start To Use:</b>
  ·  Start Trading: Send token contract address%s

Click the Refresh button to update your current balance.`,
		wallet.Address, balance, usdValue, pnlString, currentTime, depositWarning), nil
}

// BuildOptimizedPositionsText создаёт portfolio text с использованием кэша
func (cdh *CachedDataHelper) BuildOptimizedPositionsText(ctx context.Context, userID int64) (string, error) {
	// Получаем данные кошелька с кэшем
	wallet, balance, _, err := cdh.GetWalletData(ctx, userID)
	if err != nil {
		return "", err
	}

	if wallet == nil {
		return `📊 <b>Your Portfolio</b>

❌ No wallet found. Please create a wallet first.

💡 Go to Wallet → Generate Wallet to get started.`, nil
	}

	// Получаем позиции с кэшем
	positions, err := cdh.GetPositionsData(ctx, userID)
	if err != nil {
		log.Printf("⚠️ CACHE: Error getting positions for user %d: %v", userID, err)
		positions = []*TokenPosition{} // Empty fallback
	}

	// Получаем PnL с кэшем
	pnlString, err := cdh.GetPnLData(ctx, userID)
	if err != nil {
		pnlString = "🚀" // Fallback
	}

	// Строим список токенов
	var tokensList string
	if len(positions) == 0 {
		tokensList = "<i>No token positions found</i>"
	} else {
		for i, position := range positions {
			if position == nil {
				continue
			}

			// Форматируем строку позиции
			tokensList += fmt.Sprintf(
				"%d. %s 💰%.6f %s%+.1f%% ⏱%s\n",
				i+1,
				position.TokenSymbol,
				position.ValueSOL,
				position.PnLEmoji,
				position.PnLPercent,
				position.HoldingTime,
			)
		}
	}

	// Формируем итоговое сообщение
	message := fmt.Sprintf(`📊 <b>Your Portfolio</b>

<b>Balance:</b> %.8f SOL (PnL %s)

<b>Token     Holding (SOL)    PnL     Time</b>
%s`,
		balance,
		pnlString,
		tokensList,
	)

	return message, nil
}

// TokenPosition представляет кэшированную позицию токена
type TokenPosition struct {
	TokenAddress string
	TokenSymbol  string
	ValueSOL     float64
	PnLPercent   float64
	PnLEmoji     string
	HoldingTime  string
}

// GetPositionsData получает позиции с кэшем
func (cdh *CachedDataHelper) GetPositionsData(ctx context.Context, userID int64) ([]*TokenPosition, error) {
	// Проверяем кэш
	userData, exists := cdh.cache.GetUserData(userID)
	if exists && cdh.cache.IsDataFresh(userData, "positions") {
		log.Printf("✅ CACHE HIT: Positions data for user %d", userID)
		// Конвертируем из entities.TokenPosition в cache.TokenPosition
		return cdh.convertPositions(userData.Positions), nil
	}

	log.Printf("❌ CACHE MISS: Loading positions for user %d", userID)

	// Получаем кошельки
	wallets, err := cdh.services.GetWalletService().GetWallets(ctx, userID)
	if err != nil || len(wallets) == 0 {
		return []*TokenPosition{}, nil
	}

	// Берём primary кошелёк
	var primaryWallet *services.SimpleWallet
	for _, wallet := range wallets {
		if wallet.IsDefault {
			primaryWallet = &wallet
			break
		}
	}

	if primaryWallet == nil {
		primaryWallet = &wallets[0]
	}

	// Парсим ID кошелька
	walletIDInt, parseErr := strconv.ParseInt(primaryWallet.ID, 10, 64)
	if parseErr != nil {
		return []*TokenPosition{}, parseErr
	}

	// Получаем позиции через PortfolioService
	portfolioService := cdh.services.GetPortfolioService()
	if portfolioService == nil {
		return []*TokenPosition{}, nil
	}

	positions, err := portfolioService.GetPositions(ctx, walletIDInt, true) // Только активные
	if err != nil {
		return []*TokenPosition{}, err
	}

	// Обрабатываем позиции и получаем актуальные цены
	processedPositions := cdh.processPositions(ctx, positions)

	// Сохраняем в кэш
	cdh.cache.UpdateUserPositions(userID, positions, nil) // Простое кэширование

	return processedPositions, nil
}

// convertPositions конвертирует entities.TokenPosition в cache.TokenPosition
func (cdh *CachedDataHelper) convertPositions(positions []*entities.TokenPosition) []*TokenPosition {
	var result []*TokenPosition

	for _, pos := range positions {
		if pos == nil || !pos.IsActive {
			continue
		}

		// Простая конвертация без дополнительных API вызовов
		tokenSymbol := pos.TokenSymbol
		if tokenSymbol == "" && len(pos.TokenAddress) >= 8 {
			tokenSymbol = pos.TokenAddress[:8] + "..."
		}

		// Базовые расчеты
		pnlPercent := 0.0
		pnlEmoji := "📊"
		valueSOL := 0.0

		// Время холдинга
		holdingTime := "Unknown"
		if !pos.OpenedAt.IsZero() {
			duration := pos.GetAge()
			if duration.Hours() < 24 {
				holdingTime = fmt.Sprintf("%.1fh", duration.Hours())
			} else {
				holdingTime = fmt.Sprintf("%.1fd", duration.Hours()/24)
			}
		}

		result = append(result, &TokenPosition{
			TokenAddress: pos.TokenAddress,
			TokenSymbol:  tokenSymbol,
			ValueSOL:     valueSOL,
			PnLPercent:   pnlPercent,
			PnLEmoji:     pnlEmoji,
			HoldingTime:  holdingTime,
		})
	}

	return result
}

// processPositions обрабатывает позиции с получением актуальных цен
func (cdh *CachedDataHelper) processPositions(ctx context.Context, positions []*entities.TokenPosition) []*TokenPosition {
	var result []*TokenPosition

	// Получаем цену SOL для конвертации
	solPrice, err := cdh.GetSOLPrice(ctx)
	if err != nil {
		solPrice = 100.0 // Fallback
	}

	for _, pos := range positions {
		if pos == nil || !pos.IsActive {
			continue
		}

		// Получаем цену токена через кэш
		tokenPrice := cdh.getTokenPrice(ctx, pos.TokenAddress)

		// Расчеты
		positionValueUSD := pos.Amount * tokenPrice
		valueSOL := positionValueUSD / solPrice

		pnlPercent := 0.0
		if pos.EntryPrice > 0 {
			pnlPercent = ((tokenPrice - pos.EntryPrice) / pos.EntryPrice) * 100
		}

		pnlEmoji := "📊"
		if pnlPercent > 0 {
			pnlEmoji = "🚀"
		} else if pnlPercent < 0 {
			pnlEmoji = "📉"
		}

		// Символ токена
		tokenSymbol := pos.TokenSymbol
		if tokenSymbol == "" && len(pos.TokenAddress) >= 8 {
			tokenSymbol = pos.TokenAddress[:8] + "..."
		}

		// Время холдинга
		holdingTime := "Unknown"
		if !pos.OpenedAt.IsZero() {
			duration := pos.GetAge()
			if duration.Hours() < 24 {
				holdingTime = fmt.Sprintf("%.1fh", duration.Hours())
			} else {
				holdingTime = fmt.Sprintf("%.1fd", duration.Hours()/24)
			}
		}

		result = append(result, &TokenPosition{
			TokenAddress: pos.TokenAddress,
			TokenSymbol:  tokenSymbol,
			ValueSOL:     valueSOL,
			PnLPercent:   pnlPercent,
			PnLEmoji:     pnlEmoji,
			HoldingTime:  holdingTime,
		})
	}

	return result
}

// getTokenPrice получает цену токена с кэшем
func (cdh *CachedDataHelper) getTokenPrice(ctx context.Context, tokenAddress string) float64 {
	// Проверяем кэш цен токенов
	if price, found := cdh.cache.GetCachedTokenPrice(tokenAddress); found {
		return price
	}

	// Cache miss - получаем цену через TokenDataService
	tokenDataService := cdh.services.GetTokenDataService()
	if tokenDataService == nil {
		return 0.0
	}

	// Создаем SolanaTokenAddress
	tokenAddr, err := valueobjects.NewSolanaTokenAddress(tokenAddress)
	if err != nil {
		return 0.0
	}

	// Получаем метрики токена
	tokenMetrics, err := tokenDataService.GetTokenMetrics(ctx, *tokenAddr)
	if err != nil || tokenMetrics == nil {
		return 0.0
	}

	// Получаем цену и кэшируем
	priceDecimal := tokenMetrics.PriceUSD()
	price, _ := priceDecimal.Float64()

	cdh.cache.SetCachedTokenPrice(tokenAddress, price)

	return price
}
