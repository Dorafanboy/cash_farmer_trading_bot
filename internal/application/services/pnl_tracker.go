package services

import (
	"context"
	"fmt"
	"log"
	"time"

	"cash-farmer/internal/infrastructure/database/sqlc"
)

// PnLTracker отвечает за отслеживание PnL при торговых операциях
// Реализует Hybrid Architecture: Event-Driven + Polling подходы
type PnLTracker interface {
	// Event-driven methods (вызываются при торговых операциях)
	OnBuyComplete(ctx context.Context, event BuyEvent) error
	OnSellComplete(ctx context.Context, event SellEvent) error

	// Polling methods (периодические обновления)
	StartPeriodicPnLUpdates(ctx context.Context) error
	UpdateUnrealizedPnL(ctx context.Context, walletID int64) error

	// Query methods
	GetWalletPnL(ctx context.Context, walletID int64) (*PnLResult, error)
}

// PnLTrackerImpl реализует PnL Tracker
type PnLTrackerImpl struct {
	db               *sqlc.Queries
	portfolioService PortfolioService
	priceService     PriceService
	logger           *log.Logger
	updateInterval   time.Duration
}

// NewPnLTracker создает новый PnL Tracker
func NewPnLTracker(
	db *sqlc.Queries,
	portfolioService PortfolioService,
	priceService PriceService,
	logger *log.Logger,
) PnLTracker {
	return &PnLTrackerImpl{
		db:               db,
		portfolioService: portfolioService,
		priceService:     priceService,
		logger:           logger,
		updateInterval:   5 * time.Minute, // Обновляем каждые 5 минут
	}
}

// BuyEvent представляет событие покупки токена
type BuyEvent struct {
	UserID        int64     `json:"user_id"`
	WalletID      int64     `json:"wallet_id"`
	TokenAddress  string    `json:"token_address"`
	TokenSymbol   string    `json:"token_symbol"`
	Amount        float64   `json:"amount"`       // Количество SOL потрачено
	TokenAmount   float64   `json:"token_amount"` // Количество токенов получено
	EntryPrice    float64   `json:"entry_price"`  // Цена входа за токен
	TransactionID string    `json:"transaction_id"`
	Timestamp     time.Time `json:"timestamp"`
}

// SellEvent представляет событие продажи токена
type SellEvent struct {
	UserID        int64     `json:"user_id"`
	WalletID      int64     `json:"wallet_id"`
	TokenAddress  string    `json:"token_address"`
	TokenSymbol   string    `json:"token_symbol"`
	TokenAmount   float64   `json:"token_amount"` // Количество токенов продано
	SellPrice     float64   `json:"sell_price"`   // Цена продажи за токен
	SOLReceived   float64   `json:"sol_received"` // Количество SOL получено
	Percentage    float64   `json:"percentage"`   // Процент позиции проданной
	TransactionID string    `json:"transaction_id"`
	Timestamp     time.Time `json:"timestamp"`
}

// PnLResult представляет результат расчета PnL
type PnLResult struct {
	WalletID      int64     `json:"wallet_id"`
	RealizedPnL   float64   `json:"realized_pnl"`   // Завершенные сделки
	UnrealizedPnL float64   `json:"unrealized_pnl"` // Открытые позиции
	TotalPnL      float64   `json:"total_pnl"`      // Общий PnL
	LastUpdated   time.Time `json:"last_updated"`
}

// OnBuyComplete обрабатывает событие успешной покупки токена
func (p *PnLTrackerImpl) OnBuyComplete(ctx context.Context, event BuyEvent) error {
	p.logger.Printf("📊 PnL Tracker: Processing buy event for user %d, token %s, amount %.6f SOL",
		event.UserID, event.TokenSymbol, event.Amount)

	// 1. Проверяем есть ли уже активная позиция для этого токена
	existingPosition, err := p.portfolioService.GetActivePositionByToken(ctx, event.WalletID, event.TokenAddress)
	if err != nil && existingPosition == nil {
		// Новая позиция - создаем
		p.logger.Printf("📊 PnL: Creating new position for %s", event.TokenSymbol)

		_, err = p.portfolioService.OpenPosition(ctx, OpenPositionRequest{
			WalletID:     event.WalletID,
			TokenAddress: event.TokenAddress,
			TokenSymbol:  event.TokenSymbol,
			Amount:       event.TokenAmount, // Количество токенов
			EntryPrice:   event.EntryPrice,  // Цена за токен
		})
		if err != nil {
			p.logger.Printf("❌ PnL: Failed to create position: %v", err)
			return fmt.Errorf("failed to create position: %w", err)
		}

		p.logger.Printf("✅ PnL: New position created for %s", event.TokenSymbol)
	} else if existingPosition != nil {
		// Существующая позиция - обновляем (усредняем цену входа)
		p.logger.Printf("📊 PnL: Updating existing position for %s", event.TokenSymbol)

		// Рассчитываем новую среднюю цену входа
		totalValue := (existingPosition.Amount * existingPosition.EntryPrice) + (event.TokenAmount * event.EntryPrice)
		totalAmount := existingPosition.Amount + event.TokenAmount
		newAvgPrice := totalValue / totalAmount

		// Обновляем количество
		err = p.portfolioService.UpdatePositionAmount(ctx, existingPosition.ID, totalAmount)
		if err != nil {
			p.logger.Printf("❌ PnL: Failed to update position amount: %v", err)
			return fmt.Errorf("failed to update position amount: %w", err)
		}

		// Здесь нужно добавить метод для обновления entry price в PortfolioService
		// TODO: Добавить метод UpdatePositionEntryPrice в PortfolioService

		p.logger.Printf("✅ PnL: Position updated for %s, new amount: %.6f, new avg price: %.6f",
			event.TokenSymbol, totalAmount, newAvgPrice)
	}

	// 2. Записываем событие в PnL History
	err = p.recordBuyEvent(ctx, event)
	if err != nil {
		p.logger.Printf("❌ PnL: Failed to record buy event: %v", err)
		// Не возвращаем ошибку - позиция уже создана
	}

	p.logger.Printf("✅ PnL Tracker: Buy event processed successfully")
	return nil
}

// OnSellComplete обрабатывает событие успешной продажи токена
func (p *PnLTrackerImpl) OnSellComplete(ctx context.Context, event SellEvent) error {
	p.logger.Printf("📊 PnL Tracker: Processing sell event for user %d, token %s, %.2f%% sold",
		event.UserID, event.TokenSymbol, event.Percentage)

	// 1. Получаем активную позицию
	position, err := p.portfolioService.GetActivePositionByToken(ctx, event.WalletID, event.TokenAddress)
	if err != nil || position == nil {
		p.logger.Printf("❌ PnL: No active position found for %s", event.TokenSymbol)
		return fmt.Errorf("no active position found for token %s", event.TokenSymbol)
	}

	// 2. Рассчитываем realized PnL для проданной части
	soldAmount := event.TokenAmount
	entryPrice := position.EntryPrice
	sellPrice := event.SellPrice

	realizedPnL := (sellPrice - entryPrice) * soldAmount

	p.logger.Printf("📊 PnL: Realized PnL calculation:")
	p.logger.Printf("  - Sold amount: %.6f %s", soldAmount, event.TokenSymbol)
	p.logger.Printf("  - Entry price: %.6f", entryPrice)
	p.logger.Printf("  - Sell price: %.6f", sellPrice)
	p.logger.Printf("  - Realized PnL: %.2f USD", realizedPnL)

	// 3. Обновляем позицию
	remainingAmount := position.Amount - soldAmount
	if remainingAmount <= 0.000001 { // Практически 0
		// Закрываем позицию полностью
		err = p.portfolioService.ClosePosition(ctx, position.ID, event.WalletID)
		if err != nil {
			p.logger.Printf("❌ PnL: Failed to close position: %v", err)
			return fmt.Errorf("failed to close position: %w", err)
		}
		p.logger.Printf("✅ PnL: Position closed completely for %s", event.TokenSymbol)
	} else {
		// Обновляем количество
		err = p.portfolioService.UpdatePositionAmount(ctx, position.ID, remainingAmount)
		if err != nil {
			p.logger.Printf("❌ PnL: Failed to update position amount: %v", err)
			return fmt.Errorf("failed to update position amount: %w", err)
		}
		p.logger.Printf("✅ PnL: Position updated for %s, remaining: %.6f", event.TokenSymbol, remainingAmount)
	}

	// 4. Записываем realized PnL в историю
	err = p.recordSellEvent(ctx, event, realizedPnL)
	if err != nil {
		p.logger.Printf("❌ PnL: Failed to record sell event: %v", err)
		// Не возвращаем ошибку - позиция уже обновлена
	}

	p.logger.Printf("✅ PnL Tracker: Sell event processed, realized PnL: %.2f USD", realizedPnL)
	return nil
}

// StartPeriodicPnLUpdates запускает периодическое обновление unrealized PnL
func (p *PnLTrackerImpl) StartPeriodicPnLUpdates(ctx context.Context) error {
	p.logger.Printf("🔄 PnL Tracker: Starting periodic updates every %v", p.updateInterval)

	ticker := time.NewTicker(p.updateInterval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				p.logger.Printf("🔄 PnL: Running periodic unrealized PnL update")
				// Здесь можно добавить логику обновления всех кошельков
				// Пока оставляем заглушку для будущего развития
			case <-ctx.Done():
				p.logger.Printf("🔄 PnL: Stopping periodic updates")
				return
			}
		}
	}()

	return nil
}

// UpdateUnrealizedPnL обновляет unrealized PnL for конкретного кошелька
func (p *PnLTrackerImpl) UpdateUnrealizedPnL(ctx context.Context, walletID int64) error {
	// Получаем все активные позиции кошелька
	positions, err := p.portfolioService.GetPositions(ctx, walletID, true)
	if err != nil {
		return fmt.Errorf("failed to get positions: %w", err)
	}

	if len(positions) == 0 {
		p.logger.Printf("📊 PnL: No active positions for wallet %d", walletID)
		return nil
	}

	p.logger.Printf("📊 PnL: Updating unrealized PnL for %d positions in wallet %d", len(positions), walletID)

	// Обновляем цены для всех позиций через PortfolioService
	err = p.portfolioService.RefreshStalePositions(ctx, 100) // Обновляем до 100 позиций
	if err != nil {
		p.logger.Printf("❌ PnL: Failed to refresh positions: %v", err)
		return fmt.Errorf("failed to refresh positions: %w", err)
	}

	p.logger.Printf("✅ PnL: Unrealized PnL updated for wallet %d", walletID)
	return nil
}

// GetWalletPnL возвращает полный PnL для кошелька
func (p *PnLTrackerImpl) GetWalletPnL(ctx context.Context, walletID int64) (*PnLResult, error) {
	// Используем существующий метод PortfolioService
	summary, err := p.portfolioService.CalculatePnL(ctx, walletID)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate PnL: %w", err)
	}

	// Конвертируем в наш формат
	result := &PnLResult{
		WalletID:      walletID,
		RealizedPnL:   summary.TotalProfitUSD + summary.TotalLossUSD,                         // Сумма реализованного PnL
		UnrealizedPnL: summary.TotalPnLUSD - (summary.TotalProfitUSD + summary.TotalLossUSD), // Нереализованный
		TotalPnL:      summary.TotalPnLUSD,
		LastUpdated:   time.Now(),
	}

	return result, nil
}

// recordBuyEvent записывает событие покупки в PnL History
// ВРЕМЕННО ОТКЛЮЧЕНО - проблемы с sqlc
func (p *PnLTrackerImpl) recordBuyEvent(ctx context.Context, event BuyEvent) error {
	p.logger.Printf("📊 PnL: Buy event recording temporarily disabled")
	return nil
	/*
		// Создаем запись в pnl_history для события покупки
		_, err := p.db.CreatePnLHistory(ctx, sqlc.CreatePnLHistoryParams{
			WalletID:         event.WalletID,
			TokenAddress:     event.TokenAddress,
			TokenSymbol:      sqlc.NullString{String: event.TokenSymbol, Valid: true},
			RealizedPnlUsd:   sqlc.NullFloat8{Float8: 0, Valid: true}, // При покупке realized PnL = 0
			UnrealizedPnlUsd: sqlc.NullFloat8{Float8: 0, Valid: true}, // При покупке unrealized PnL = 0
			TradeType:        sqlc.NullString{String: "BUY", Valid: true},
			Amount:           sqlc.NullFloat8{Float8: event.Amount, Valid: true},
			Price:            sqlc.NullFloat8{Float8: event.EntryPrice, Valid: true},
			TransactionHash:  sqlc.NullString{String: event.TransactionID, Valid: true},
		})

		if err != nil {
			return fmt.Errorf("failed to create PnL history record: %w", err)
		}

		p.logger.Printf("📊 PnL: Buy event recorded in history")
		return nil
	*/
}

// recordSellEvent записывает событие продажи в PnL History
// ВРЕМЕННО ОТКЛЮЧЕНО - проблемы с sqlc
func (p *PnLTrackerImpl) recordSellEvent(ctx context.Context, event SellEvent, realizedPnL float64) error {
	p.logger.Printf("📊 PnL: Sell event recording temporarily disabled, realized PnL: %.2f USD", realizedPnL)
	return nil
	/*
		// Создаем запись в pnl_history для события продажи
		_, err := p.db.CreatePnLHistory(ctx, sqlc.CreatePnLHistoryParams{
			WalletID:         event.WalletID,
			TokenAddress:     event.TokenAddress,
			TokenSymbol:      sqlc.NullString{String: event.TokenSymbol, Valid: true},
			RealizedPnlUsd:   sqlc.NullString{String: fmt.Sprintf("%.2f", realizedPnL), Valid: true},
			UnrealizedPnlUsd: sqlc.NullFloat8{Float8: 0, Valid: true}, // При продаже unrealized обнуляется для проданной части
			TradeType:        sqlc.NullString{String: "SELL", Valid: true},
			Amount:           sqlc.NullFloat8{Float8: event.TokenAmount, Valid: true},
			Price:            sqlc.NullFloat8{Float8: event.SellPrice, Valid: true},
			TransactionHash:  sqlc.NullString{String: event.TransactionID, Valid: true},
		})

		if err != nil {
			return fmt.Errorf("failed to create PnL history record: %w", err)
		}

		p.logger.Printf("📊 PnL: Sell event recorded in history, realized PnL: %.2f USD", realizedPnL)
		return nil
	*/
}
