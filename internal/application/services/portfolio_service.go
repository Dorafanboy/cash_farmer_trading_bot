package services

import (
	"context"
	"fmt"
	"log"
	"strconv"

	"cash-farmer/internal/application/repositories"
	"cash-farmer/internal/domain/entities"
	"cash-farmer/internal/infrastructure/database/sqlc"

	"github.com/jackc/pgx/v5/pgtype"
)

// PortfolioService provides business logic for portfolio and token position operations
type PortfolioService interface {
	// Position management
	OpenPosition(ctx context.Context, req OpenPositionRequest) (*entities.TokenPosition, error)
	ClosePosition(ctx context.Context, positionID int64, walletID int64) error
	UpdatePositionAmount(ctx context.Context, positionID int64, newAmount float64) error

	// Position queries
	GetPosition(ctx context.Context, positionID int64) (*entities.TokenPosition, error)
	GetPositions(ctx context.Context, walletID int64, activeOnly bool) ([]*entities.TokenPosition, error)
	GetPositionsByUser(ctx context.Context, userID int64) ([]*entities.TokenPosition, error)
	GetActivePositionByToken(ctx context.Context, walletID int64, tokenAddress string) (*entities.TokenPosition, error)

	// Price updates
	UpdatePositionPrice(ctx context.Context, positionID int64, newPrice float64) error
	BulkUpdatePrices(ctx context.Context, priceUpdates []PositionPriceUpdate) error
	RefreshStalePositions(ctx context.Context, limit int) error

	// Analytics
	CalculatePnL(ctx context.Context, walletID int64) (*PnLSummary, error)
	GetTopPositions(ctx context.Context, walletID int64, limit int) ([]*entities.TokenPosition, error)
	GetPortfolioSummary(ctx context.Context, userID int64) (*PortfolioSummary, error)

	// Cleanup
	CleanupSuspiciousPositions(ctx context.Context, walletID int64) error

	// Demo
	// CreateDemoPositions удалена - позиции создаются при реальных покупках
}

// PortfolioServiceImpl implements PortfolioService
type PortfolioServiceImpl struct {
	db           *sqlc.Queries
	walletRepo   repositories.WalletRepository
	priceService PriceService // Interface for price updates
}

// NewPortfolioService creates a new portfolio service
func NewPortfolioService(
	db *sqlc.Queries,
	walletRepo repositories.WalletRepository,
	priceService PriceService,
) PortfolioService {
	return &PortfolioServiceImpl{
		db:           db,
		walletRepo:   walletRepo,
		priceService: priceService,
	}
}

// PriceService defines interface for price operations
type PriceService interface {
	GetTokenPrice(ctx context.Context, tokenAddress string) (float64, error)
	GetMultipleTokenPrices(ctx context.Context, tokenAddresses []string) (map[string]float64, error)
}

// OpenPositionRequest represents request to open a new position
type OpenPositionRequest struct {
	WalletID     int64   `json:"wallet_id"`
	TokenAddress string  `json:"token_address"`
	TokenSymbol  string  `json:"token_symbol"`
	Amount       float64 `json:"amount"`
	EntryPrice   float64 `json:"entry_price"`
}

// PositionPriceUpdate represents a price update for a position
type PositionPriceUpdate struct {
	TokenAddress string  `json:"token_address"`
	NewPrice     float64 `json:"new_price"`
}

// PnLSummary represents P&L summary for a wallet
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

// PortfolioSummary represents overall portfolio summary for a user
type PortfolioSummary struct {
	UserID          int64                     `json:"user_id"`
	TotalWallets    int                       `json:"total_wallets"`
	TotalPositions  int64                     `json:"total_positions"`
	TotalValue      float64                   `json:"total_value"`
	TotalPnL        float64                   `json:"total_pnl"`
	TopPositions    []*entities.TokenPosition `json:"top_positions"`
	WalletSummaries []PnLSummary              `json:"wallet_summaries"`
}

// Helper functions for type conversion
func floatToNumeric(f float64) pgtype.Numeric {
	var n pgtype.Numeric
	// Use string conversion to ensure valid numeric value
	err := n.Scan(fmt.Sprintf("%.6f", f))
	if err != nil {
		// Fallback to 0 if scan fails
		n.Scan("0")
	}
	return n
}

func numericToFloat(n pgtype.Numeric) float64 {
	if !n.Valid {
		return 0
	}
	// Use Float64Value method for proper conversion
	f, err := n.Float64Value()
	if err != nil {
		return 0
	}
	return f.Float64
}

// interfaceToFloat safely converts interface{} to float64 for database results with COALESCE
func interfaceToFloat(v interface{}) float64 {
	if v == nil {
		return 0
	}

	switch val := v.(type) {
	case int64:
		return float64(val)
	case float64:
		return val
	case int32:
		return float64(val)
	case string:
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			return f
		}
		return 0
	default:
		// Try to convert to string first and then parse
		if str := fmt.Sprintf("%v", val); str != "" {
			if f, err := strconv.ParseFloat(str, 64); err == nil {
				return f
			}
		}
		return 0
	}
}

func stringToText(s string) pgtype.Text {
	return pgtype.Text{String: s, Valid: true}
}

func textToString(t pgtype.Text) string {
	if !t.Valid {
		return ""
	}
	return t.String
}

// OpenPosition opens a new token position
func (ps *PortfolioServiceImpl) OpenPosition(ctx context.Context, req OpenPositionRequest) (*entities.TokenPosition, error) {
	log.Printf("Opening position: wallet=%d, token=%s, amount=%.6f, price=%.6f",
		req.WalletID, req.TokenSymbol, req.Amount, req.EntryPrice)

	// Check if there's already an active position for this token in this wallet
	existingPosition, err := ps.GetActivePositionByToken(ctx, req.WalletID, req.TokenAddress)
	if err == nil && existingPosition != nil {
		return nil, fmt.Errorf("active position already exists for token %s in wallet %d", req.TokenSymbol, req.WalletID)
	}

	// Create position entity
	position := entities.NewTokenPosition(req.WalletID, req.TokenAddress, req.TokenSymbol, req.Amount, req.EntryPrice)

	// Validate position
	if err := position.Validate(); err != nil {
		return nil, fmt.Errorf("invalid position data: %w", err)
	}

	// Create in database
	dbPosition, err := ps.db.CreateTokenPosition(ctx, sqlc.CreateTokenPositionParams{
		WalletID:     req.WalletID,
		TokenAddress: req.TokenAddress,
		TokenSymbol:  stringToText(req.TokenSymbol),
		Amount:       floatToNumeric(req.Amount),
		EntryPrice:   floatToNumeric(req.EntryPrice),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create position in database: %w", err)
	}

	// Convert back to entity
	result := ps.dbPositionToEntity(&dbPosition)
	log.Printf("Position opened successfully: ID=%d, Token=%s", result.ID, result.TokenSymbol)
	return result, nil
}

// ClosePosition closes an existing position
func (ps *PortfolioServiceImpl) ClosePosition(ctx context.Context, positionID int64, walletID int64) error {
	log.Printf("Closing position: ID=%d, wallet=%d", positionID, walletID)

	// Verify position exists and belongs to wallet
	position, err := ps.GetPosition(ctx, positionID)
	if err != nil {
		return fmt.Errorf("position not found: %w", err)
	}

	if position.WalletID != walletID {
		return fmt.Errorf("position %d does not belong to wallet %d", positionID, walletID)
	}

	// Close position
	err = ps.db.CloseTokenPosition(ctx, positionID)
	if err != nil {
		return fmt.Errorf("failed to close position: %w", err)
	}

	log.Printf("Position closed successfully: ID=%d", positionID)
	return nil
}

// UpdatePositionAmount updates the amount of tokens in a position
func (ps *PortfolioServiceImpl) UpdatePositionAmount(ctx context.Context, positionID int64, newAmount float64) error {
	log.Printf("Updating position amount: ID=%d, new amount=%.6f", positionID, newAmount)

	if newAmount <= 0 {
		return fmt.Errorf("position amount must be positive, got %.6f", newAmount)
	}

	err := ps.db.UpdateTokenPositionAmount(ctx, sqlc.UpdateTokenPositionAmountParams{
		ID:     positionID,
		Amount: floatToNumeric(newAmount),
	})
	if err != nil {
		return fmt.Errorf("failed to update position amount: %w", err)
	}

	log.Printf("Position amount updated successfully: ID=%d", positionID)
	return nil
}

// GetPosition retrieves a specific position by ID
func (ps *PortfolioServiceImpl) GetPosition(ctx context.Context, positionID int64) (*entities.TokenPosition, error) {
	dbPosition, err := ps.db.GetTokenPosition(ctx, positionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get position: %w", err)
	}

	return ps.dbPositionToEntity(&dbPosition), nil
}

// GetPositions retrieves all positions for a wallet
func (ps *PortfolioServiceImpl) GetPositions(ctx context.Context, walletID int64, activeOnly bool) ([]*entities.TokenPosition, error) {
	log.Printf("Getting positions for wallet %d, active only: %t", walletID, activeOnly)

	var dbPositions []sqlc.TokenPositions
	var err error

	if activeOnly {
		dbPositions, err = ps.db.GetActivePositionsByWallet(ctx, walletID)
	} else {
		dbPositions, err = ps.db.GetAllPositionsByWallet(ctx, walletID)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get positions: %w", err)
	}

	positions := make([]*entities.TokenPosition, len(dbPositions))
	for i, dbPos := range dbPositions {
		positions[i] = ps.dbPositionToEntity(&dbPos)
	}

	log.Printf("Retrieved %d positions for wallet %d", len(positions), walletID)
	return positions, nil
}

// GetPositionsByUser retrieves all positions for all user's wallets
func (ps *PortfolioServiceImpl) GetPositionsByUser(ctx context.Context, userID int64) ([]*entities.TokenPosition, error) {
	log.Printf("Getting all positions for user %d", userID)

	// Get all user's wallets
	wallets, err := ps.walletRepo.FindByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user wallets: %w", err)
	}

	var allPositions []*entities.TokenPosition

	// Get positions for each wallet
	for _, wallet := range wallets {
		positions, err := ps.GetPositions(ctx, wallet.ID, true) // Only active positions
		if err != nil {
			log.Printf("Warning: failed to get positions for wallet %d: %v", wallet.ID, err)
			continue
		}
		allPositions = append(allPositions, positions...)
	}

	log.Printf("Retrieved %d total positions for user %d", len(allPositions), userID)
	return allPositions, nil
}

// GetActivePositionByToken finds active position for specific token in wallet
func (ps *PortfolioServiceImpl) GetActivePositionByToken(ctx context.Context, walletID int64, tokenAddress string) (*entities.TokenPosition, error) {
	dbPosition, err := ps.db.GetActiveTokenPosition(ctx, sqlc.GetActiveTokenPositionParams{
		WalletID:     walletID,
		TokenAddress: tokenAddress,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get active position: %w", err)
	}

	return ps.dbPositionToEntity(&dbPosition), nil
}

// UpdatePositionPrice updates the current price of a position
func (ps *PortfolioServiceImpl) UpdatePositionPrice(ctx context.Context, positionID int64, newPrice float64) error {
	log.Printf("Updating position price: ID=%d, new price=%.6f", positionID, newPrice)

	if newPrice <= 0 {
		return fmt.Errorf("price must be positive, got %.6f", newPrice)
	}

	// Get current position to calculate PnL
	position, err := ps.GetPosition(ctx, positionID)
	if err != nil {
		return fmt.Errorf("failed to get position for PnL calculation: %w", err)
	}

	// Calculate new PnL
	pnl := (newPrice - position.EntryPrice) * position.Amount

	// Update both price and PnL
	err = ps.db.UpdateTokenPositionPnL(ctx, sqlc.UpdateTokenPositionPnLParams{
		ID:           positionID,
		CurrentPrice: floatToNumeric(newPrice),
		PnlUsd:       floatToNumeric(pnl),
	})
	if err != nil {
		return fmt.Errorf("failed to update position price and PnL: %w", err)
	}

	log.Printf("Position price updated: ID=%d, Price=%.6f, PnL=%.2f", positionID, newPrice, pnl)
	return nil
}

// BulkUpdatePrices updates prices for multiple positions
func (ps *PortfolioServiceImpl) BulkUpdatePrices(ctx context.Context, priceUpdates []PositionPriceUpdate) error {
	log.Printf("Bulk updating prices for %d tokens", len(priceUpdates))

	// For simplicity, we'll update positions one by one
	// In a real implementation, you might want to batch these operations
	for _, update := range priceUpdates {
		// Get all active positions for this token
		positions, err := ps.db.GetPositionsByToken(ctx, sqlc.GetPositionsByTokenParams{
			TokenAddress: update.TokenAddress,
			IsActive:     true,
		})
		if err != nil {
			log.Printf("Warning: failed to get positions for token %s: %v", update.TokenAddress, err)
			continue
		}

		// Update each position
		for _, pos := range positions {
			err := ps.UpdatePositionPrice(ctx, pos.ID, update.NewPrice)
			if err != nil {
				log.Printf("Warning: failed to update position %d price: %v", pos.ID, err)
			}
		}
	}

	log.Printf("Bulk price update completed")
	return nil
}

// RefreshStalePositions updates prices for positions that haven't been updated recently
func (ps *PortfolioServiceImpl) RefreshStalePositions(ctx context.Context, limit int) error {
	log.Printf("Refreshing stale positions (limit: %d)", limit)

	// Get positions that need price updates
	stalePositions, err := ps.db.GetPositionsForPriceUpdate(ctx, int32(limit))
	if err != nil {
		return fmt.Errorf("failed to get stale positions: %w", err)
	}

	if len(stalePositions) == 0 {
		log.Printf("No stale positions found")
		return nil
	}

	// Collect unique token addresses
	tokenAddresses := make([]string, 0, len(stalePositions))
	addressSet := make(map[string]bool)
	for _, pos := range stalePositions {
		if !addressSet[pos.TokenAddress] {
			tokenAddresses = append(tokenAddresses, pos.TokenAddress)
			addressSet[pos.TokenAddress] = true
		}
	}

	// Get current prices (if price service is available)
	if ps.priceService != nil {
		prices, err := ps.priceService.GetMultipleTokenPrices(ctx, tokenAddresses)
		if err != nil {
			log.Printf("Warning: failed to get current prices: %v", err)
			return nil // Don't fail the operation, just log warning
		}

		// Update positions with new prices
		for _, pos := range stalePositions {
			if newPrice, exists := prices[pos.TokenAddress]; exists {
				err := ps.UpdatePositionPrice(ctx, pos.ID, newPrice)
				if err != nil {
					log.Printf("Warning: failed to update position %d: %v", pos.ID, err)
				}
			}
		}
	}

	log.Printf("Refreshed %d stale positions", len(stalePositions))
	return nil
}

// CalculatePnL calculates P&L summary for a wallet
func (ps *PortfolioServiceImpl) CalculatePnL(ctx context.Context, walletID int64) (*PnLSummary, error) {
	log.Printf("Calculating P&L for wallet %d", walletID)

	// Получаем активные позиции
	positions, err := ps.GetPositions(ctx, walletID, true)
	if err != nil {
		return nil, fmt.Errorf("failed to get positions: %w", err)
	}

	var totalPnL float64 = 0
	var totalProfit float64 = 0
	var totalLoss float64 = 0
	var winningPositions int64 = 0
	var losingPositions int64 = 0
	activeCount := int64(len(positions))

	// Рассчитываем PnL в реальном времени для каждой позиции
	for _, pos := range positions {
		if pos != nil && pos.IsActive {
			// Рассчитываем PnL: (текущая_цена - цена_входа) * количество
			var currentPrice float64 = pos.EntryPrice // Fallback к цене входа

			// Получаем реальную цену через PriceService
			if ps.priceService != nil {
				if realPrice, err := ps.priceService.GetTokenPrice(ctx, pos.TokenAddress); err == nil && realPrice > 0 {
					currentPrice = realPrice
				}
			}

			positionPnL := (currentPrice - pos.EntryPrice) * pos.Amount
			totalPnL += positionPnL

			if positionPnL > 0 {
				totalProfit += positionPnL
				winningPositions++
			} else if positionPnL < 0 {
				totalLoss += positionPnL
				losingPositions++
			}

			log.Printf("🔍 PnL CALC: %s - Entry: $%.5f, Current: $%.5f, Amount: %.2f, PnL: $%.2f",
				pos.TokenSymbol, pos.EntryPrice, currentPrice, pos.Amount, positionPnL)
		}
	}

	// Calculate win rate and average PnL
	var winRate float64
	var averagePnL float64
	if activeCount > 0 {
		winRate = (float64(winningPositions) / float64(activeCount)) * 100
		averagePnL = totalPnL / float64(activeCount)
	}

	result := &PnLSummary{
		WalletID:             walletID,
		TotalActivePositions: activeCount,
		TotalPnLUSD:          totalPnL,
		AveragePnLUSD:        averagePnL,
		TotalProfitUSD:       totalProfit,
		TotalLossUSD:         totalLoss,
		WinningPositions:     winningPositions,
		LosingPositions:      losingPositions,
		WinRate:              winRate,
	}

	log.Printf("🎯 REAL-TIME PnL: wallet=%d, total_pnl=%.2f, positions=%d, wins=%d, losses=%d",
		walletID, result.TotalPnLUSD, result.TotalActivePositions, winningPositions, losingPositions)
	return result, nil
}

// GetTopPositions retrieves top performing positions by P&L
func (ps *PortfolioServiceImpl) GetTopPositions(ctx context.Context, walletID int64, limit int) ([]*entities.TokenPosition, error) {
	log.Printf("Getting top %d positions for wallet %d", limit, walletID)

	dbPositions, err := ps.db.GetTopPnLPositions(ctx, sqlc.GetTopPnLPositionsParams{
		WalletID: walletID,
		Limit:    int32(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get top positions: %w", err)
	}

	positions := make([]*entities.TokenPosition, len(dbPositions))
	for i, dbPos := range dbPositions {
		positions[i] = ps.dbPositionToEntity(&dbPos)
	}

	log.Printf("Retrieved %d top positions for wallet %d", len(positions), walletID)
	return positions, nil
}

// GetPortfolioSummary generates overall portfolio summary for a user
func (ps *PortfolioServiceImpl) GetPortfolioSummary(ctx context.Context, userID int64) (*PortfolioSummary, error) {
	log.Printf("Generating portfolio summary for user %d", userID)

	// Get all user's wallets
	wallets, err := ps.walletRepo.FindByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user wallets: %w", err)
	}

	summary := &PortfolioSummary{
		UserID:          userID,
		TotalWallets:    len(wallets),
		WalletSummaries: make([]PnLSummary, 0, len(wallets)),
	}

	var allTopPositions []*entities.TokenPosition

	// Process each wallet
	for _, wallet := range wallets {
		// Get P&L summary for this wallet
		pnlSummary, err := ps.CalculatePnL(ctx, wallet.ID)
		if err != nil {
			log.Printf("Warning: failed to calculate P&L for wallet %d: %v", wallet.ID, err)
			continue
		}
		summary.WalletSummaries = append(summary.WalletSummaries, *pnlSummary)

		// Add to totals
		summary.TotalPositions += pnlSummary.TotalActivePositions
		summary.TotalPnL += pnlSummary.TotalPnLUSD

		// Get top positions for this wallet
		topPositions, err := ps.GetTopPositions(ctx, wallet.ID, 3)
		if err != nil {
			log.Printf("Warning: failed to get top positions for wallet %d: %v", wallet.ID, err)
			continue
		}
		allTopPositions = append(allTopPositions, topPositions...)
	}

	// Sort and limit top positions across all wallets
	// For simplicity, just take first 10. In real implementation, sort by PnL
	if len(allTopPositions) > 10 {
		summary.TopPositions = allTopPositions[:10]
	} else {
		summary.TopPositions = allTopPositions
	}

	// Calculate total portfolio value (entry values + PnL)
	summary.TotalValue = summary.TotalPnL // Simplified for now

	log.Printf("Portfolio summary generated: user=%d, wallets=%d, positions=%d, pnl=%.2f",
		userID, summary.TotalWallets, summary.TotalPositions, summary.TotalPnL)
	return summary, nil
}

// dbPositionToEntity converts database position to domain entity
func (ps *PortfolioServiceImpl) dbPositionToEntity(dbPos *sqlc.TokenPositions) *entities.TokenPosition {
	position := &entities.TokenPosition{
		ID:           dbPos.ID,
		WalletID:     dbPos.WalletID,
		TokenAddress: dbPos.TokenAddress,
		TokenSymbol:  textToString(dbPos.TokenSymbol),
		Amount:       numericToFloat(dbPos.Amount),
		EntryPrice:   numericToFloat(dbPos.EntryPrice),
		OpenedAt:     dbPos.OpenedAt.Time,
		UpdatedAt:    dbPos.UpdatedAt.Time,
		IsActive:     dbPos.IsActive,
	}

	if dbPos.CurrentPrice.Valid {
		currentPrice := numericToFloat(dbPos.CurrentPrice)
		position.CurrentPrice = &currentPrice
	}

	if dbPos.PnlUsd.Valid {
		pnlUsd := numericToFloat(dbPos.PnlUsd)
		position.PnLUSD = &pnlUsd
	}

	return position
}

// CleanupSuspiciousPositions помечает позиции как неактивные если количество токенов подозрительно большое
func (ps *PortfolioServiceImpl) CleanupSuspiciousPositions(ctx context.Context, walletID int64) error {
	log.Printf("🧹 CLEANUP: Checking suspicious positions for wallet %d", walletID)

	// Получаем все активные позиции кошелька
	positions, err := ps.GetPositions(ctx, walletID, true)
	if err != nil {
		return fmt.Errorf("failed to get positions: %w", err)
	}

	var cleanedCount int
	for _, pos := range positions {
		if pos != nil && pos.IsActive {
			// Проверяем подозрительно большие количества токенов
			// которые могли остаться после продажи
			if pos.Amount > 1000 {
				log.Printf("🚨 SUSPICIOUS: Position %s has %.2f tokens - marking as inactive",
					pos.TokenSymbol, pos.Amount)

				// Помечаем позицию как неактивную в базе данных
				err := ps.ClosePosition(ctx, pos.ID, walletID)
				if err != nil {
					log.Printf("❌ Failed to close suspicious position %d: %v", pos.ID, err)
				} else {
					cleanedCount++
					log.Printf("✅ Closed suspicious position %s (%.2f tokens)",
						pos.TokenSymbol, pos.Amount)
				}
			}
		}
	}

	if cleanedCount > 0 {
		log.Printf("🎯 CLEANUP SUMMARY: Closed %d suspicious positions for wallet %d",
			cleanedCount, walletID)
	} else {
		log.Printf("✅ CLEANUP: No suspicious positions found for wallet %d", walletID)
	}

	return nil
}

// CreateDemoPositions удалена - позиции создаются при реальных покупках
