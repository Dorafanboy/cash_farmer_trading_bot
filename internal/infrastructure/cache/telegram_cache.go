package cache

import (
	"context"
	"sync"
	"time"

	"cash-farmer/internal/domain/entities"
)

// SimpleWallet представляет упрощённую структуру кошелька для кэша
type SimpleWallet struct {
	ID      string `json:"id"` // Соответствует services.SimpleWallet.ID
	UserID  int64  `json:"user_id"`
	Address string `json:"address"`
	Name    string `json:"name"`
}

// PnLSummary представляет сводку P&L для кэша
type PnLSummary struct {
	WalletID             int64   `json:"wallet_id"`
	TotalActivePositions int64   `json:"total_active_positions"`
	TotalPnLUSD          float64 `json:"total_pnl_usd"`
	AveragePnLUSD        float64 `json:"average_pnl_usd"`
	TotalProfitUSD       float64 `json:"total_profit_usd"`
	TotalLossUSD         float64 `json:"total_loss_usd"`
	WinningPositions     int64   `json:"winning_positions"`
	LosingPositions      int64   `json:"losing_positions"`
	WinRate              float64 `json:"win_rate"`
}

// CachedUserData содержит кэшированные данные пользователя с TTL
type CachedUserData struct {
	UserID    int64
	UpdatedAt time.Time

	// Кошелёк и баланс (TTL: 30 секунд)
	PrimaryWallet *SimpleWallet
	SOLBalance    float64
	SOLPriceUSD   float64
	USDBalance    float64

	// PnL данные (TTL: 15 секунд)
	PnLValue   float64
	PnLPercent float64
	PnLString  string

	// Позиции (TTL: 15 секунд)
	Positions    []*entities.TokenPosition
	PositionsPnL *PnLSummary

	// Настройки пользователя (TTL: 5 минут)
	UserSettings *entities.UserSettings

	// Временные метки для разных типов данных
	WalletUpdatedAt    time.Time
	PnLUpdatedAt       time.Time
	PositionsUpdatedAt time.Time
	SettingsUpdatedAt  time.Time
}

// TelegramCache кэширует данные для быстрого отклика телеграм-бота
type TelegramCache struct {
	mu    sync.RWMutex
	cache map[int64]*CachedUserData

	// TTL для разных типов данных
	walletTTL    time.Duration // Кошелёк и баланс
	priceTTL     time.Duration // Цены токенов и SOL
	positionsTTL time.Duration // Позиции и PnL
	settingsTTL  time.Duration // Настройки пользователя

	// Глобальный кэш для общих данных
	globalSOLPrice   float64
	globalSOLPriceAt time.Time
	tokenPrices      map[string]float64 // tokenAddress -> price
	tokenPricesAt    map[string]time.Time
}

// NewTelegramCache создаёт новый кэш для телеграм-бота
func NewTelegramCache() *TelegramCache {
	return &TelegramCache{
		cache:         make(map[int64]*CachedUserData),
		tokenPrices:   make(map[string]float64),
		tokenPricesAt: make(map[string]time.Time),

		// TTL конфигурация
		walletTTL:    30 * time.Second, // Баланс кошелька
		priceTTL:     60 * time.Second, // Цены токенов и SOL
		positionsTTL: 15 * time.Second, // Позиции и PnL
		settingsTTL:  5 * time.Minute,  // Настройки пользователя
	}
}

// GetUserData получает кэшированные данные пользователя
func (tc *TelegramCache) GetUserData(userID int64) (*CachedUserData, bool) {
	tc.mu.RLock()
	defer tc.mu.RUnlock()

	data, exists := tc.cache[userID]
	if !exists {
		return nil, false
	}

	return data, true
}

// SetUserData сохраняет данные пользователя в кэш
func (tc *TelegramCache) SetUserData(userID int64, data *CachedUserData) {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	data.UserID = userID
	data.UpdatedAt = time.Now()
	tc.cache[userID] = data
}

// IsDataFresh проверяет свежесть определённого типа данных
func (tc *TelegramCache) IsDataFresh(data *CachedUserData, dataType string) bool {
	if data == nil {
		return false
	}

	now := time.Now()
	switch dataType {
	case "wallet":
		return now.Sub(data.WalletUpdatedAt) <= tc.walletTTL
	case "price":
		return now.Sub(data.UpdatedAt) <= tc.priceTTL
	case "positions":
		return now.Sub(data.PositionsUpdatedAt) <= tc.positionsTTL
	case "settings":
		return now.Sub(data.SettingsUpdatedAt) <= tc.settingsTTL
	case "pnl":
		return now.Sub(data.PnLUpdatedAt) <= tc.positionsTTL
	default:
		// Проверяем общую свежесть данных
		return now.Sub(data.UpdatedAt) <= tc.walletTTL
	}
}

// UpdateUserWallet обновляет данные кошелька пользователя
func (tc *TelegramCache) UpdateUserWallet(userID int64, wallet *SimpleWallet, balance float64) {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	data, exists := tc.cache[userID]
	if !exists {
		data = &CachedUserData{UserID: userID}
		tc.cache[userID] = data
	}

	data.PrimaryWallet = wallet
	data.SOLBalance = balance
	data.WalletUpdatedAt = time.Now()
	data.UpdatedAt = time.Now()

	// Обновляем USD баланс если есть цена SOL
	if tc.globalSOLPrice > 0 && time.Since(tc.globalSOLPriceAt) <= tc.priceTTL {
		data.SOLPriceUSD = tc.globalSOLPrice
		data.USDBalance = balance * tc.globalSOLPrice
	}
}

// UpdateUserPnL обновляет PnL данные пользователя
func (tc *TelegramCache) UpdateUserPnL(userID int64, pnlValue, pnlPercent float64, pnlString string) {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	data, exists := tc.cache[userID]
	if !exists {
		data = &CachedUserData{UserID: userID}
		tc.cache[userID] = data
	}

	data.PnLValue = pnlValue
	data.PnLPercent = pnlPercent
	data.PnLString = pnlString
	data.PnLUpdatedAt = time.Now()
	data.UpdatedAt = time.Now()
}

// UpdateUserPositions обновляет позиции пользователя
func (tc *TelegramCache) UpdateUserPositions(userID int64, positions []*entities.TokenPosition, pnlSummary *PnLSummary) {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	data, exists := tc.cache[userID]
	if !exists {
		data = &CachedUserData{UserID: userID}
		tc.cache[userID] = data
	}

	data.Positions = positions
	data.PositionsPnL = pnlSummary
	data.PositionsUpdatedAt = time.Now()
	data.UpdatedAt = time.Now()
}

// UpdateUserSettings обновляет настройки пользователя
func (tc *TelegramCache) UpdateUserSettings(userID int64, settings *entities.UserSettings) {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	data, exists := tc.cache[userID]
	if !exists {
		data = &CachedUserData{UserID: userID}
		tc.cache[userID] = data
	}

	data.UserSettings = settings
	data.SettingsUpdatedAt = time.Now()
	data.UpdatedAt = time.Now()
}

// GetCachedSOLPrice получает кэшированную цену SOL
func (tc *TelegramCache) GetCachedSOLPrice() (float64, bool) {
	tc.mu.RLock()
	defer tc.mu.RUnlock()

	if time.Since(tc.globalSOLPriceAt) <= tc.priceTTL && tc.globalSOLPrice > 0 {
		return tc.globalSOLPrice, true
	}

	return 0, false
}

// SetCachedSOLPrice устанавливает кэшированную цену SOL
func (tc *TelegramCache) SetCachedSOLPrice(price float64) {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	tc.globalSOLPrice = price
	tc.globalSOLPriceAt = time.Now()

	// Обновляем USD балансы всех пользователей
	for _, data := range tc.cache {
		if data.SOLBalance > 0 {
			data.SOLPriceUSD = price
			data.USDBalance = data.SOLBalance * price
		}
	}
}

// GetCachedTokenPrice получает кэшированную цену токена
func (tc *TelegramCache) GetCachedTokenPrice(tokenAddress string) (float64, bool) {
	tc.mu.RLock()
	defer tc.mu.RUnlock()

	price, exists := tc.tokenPrices[tokenAddress]
	if !exists {
		return 0, false
	}

	priceTime, timeExists := tc.tokenPricesAt[tokenAddress]
	if !timeExists || time.Since(priceTime) > tc.priceTTL {
		return 0, false
	}

	return price, true
}

// SetCachedTokenPrice устанавливает кэшированную цену токена
func (tc *TelegramCache) SetCachedTokenPrice(tokenAddress string, price float64) {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	tc.tokenPrices[tokenAddress] = price
	tc.tokenPricesAt[tokenAddress] = time.Now()
}

// CleanupExpired удаляет устаревшие записи из кэша
func (tc *TelegramCache) CleanupExpired() {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	now := time.Now()

	// Очистка пользовательских данных (старше 10 минут)
	for userID, data := range tc.cache {
		if now.Sub(data.UpdatedAt) > 10*time.Minute {
			delete(tc.cache, userID)
		}
	}

	// Очистка цен токенов
	for tokenAddress, priceTime := range tc.tokenPricesAt {
		if now.Sub(priceTime) > tc.priceTTL*2 { // Двойной TTL для безопасности
			delete(tc.tokenPrices, tokenAddress)
			delete(tc.tokenPricesAt, tokenAddress)
		}
	}
}

// StartCleanupRoutine запускает горутину для периодической очистки кэша
func (tc *TelegramCache) StartCleanupRoutine(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				tc.CleanupExpired()
			}
		}
	}()
}

// GetCacheStats возвращает статистику кэша
func (tc *TelegramCache) GetCacheStats() map[string]interface{} {
	tc.mu.RLock()
	defer tc.mu.RUnlock()

	stats := make(map[string]interface{})
	stats["total_users"] = len(tc.cache)
	stats["total_token_prices"] = len(tc.tokenPrices)

	fresh := 0
	for _, data := range tc.cache {
		if tc.IsDataFresh(data, "wallet") {
			fresh++
		}
	}
	stats["fresh_entries"] = fresh
	stats["wallet_ttl_seconds"] = tc.walletTTL.Seconds()
	stats["price_ttl_seconds"] = tc.priceTTL.Seconds()
	stats["positions_ttl_seconds"] = tc.positionsTTL.Seconds()

	// SOL price info
	stats["sol_price"] = tc.globalSOLPrice
	stats["sol_price_age_seconds"] = time.Since(tc.globalSOLPriceAt).Seconds()
	stats["sol_price_fresh"] = time.Since(tc.globalSOLPriceAt) <= tc.priceTTL

	return stats
}

// InvalidateUser удаляет данные пользователя из кэша
func (tc *TelegramCache) InvalidateUser(userID int64) {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	delete(tc.cache, userID)
}

// GetMissingDataTypes определяет какие типы данных нужно обновить
func (tc *TelegramCache) GetMissingDataTypes(data *CachedUserData) []string {
	if data == nil {
		return []string{"wallet", "positions", "pnl", "settings"}
	}

	var missing []string
	if !tc.IsDataFresh(data, "wallet") {
		missing = append(missing, "wallet")
	}
	if !tc.IsDataFresh(data, "positions") {
		missing = append(missing, "positions")
	}
	if !tc.IsDataFresh(data, "pnl") {
		missing = append(missing, "pnl")
	}
	if !tc.IsDataFresh(data, "settings") {
		missing = append(missing, "settings")
	}

	return missing
}
