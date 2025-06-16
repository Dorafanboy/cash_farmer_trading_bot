package services

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"sync"
	"time"

	"cash-farmer/internal/domain/entities"
)

// SimpleWallet представляет упрощенную модель кошелька для демонстрации
type SimpleWallet struct {
	ID         string    `json:"id"`
	UserID     int64     `json:"user_id"`
	Name       string    `json:"name"`
	PublicKey  string    `json:"public_key"`
	PrivateKey string    `json:"-"` // Не экспортируем в JSON по соображениям безопасности
	IsDefault  bool      `json:"is_default"`
	CreatedAt  time.Time `json:"created_at"`
}

// SimpleWalletService интерфейс для работы с кошельками в демонстрационной версии
type SimpleWalletService interface {
	GetWallets(ctx context.Context, userID int64) ([]SimpleWallet, error)
	CreateWallet(ctx context.Context, userID int64, name string) (*SimpleWallet, error)
	GetBalance(ctx context.Context, walletID string) (float64, error)
	ExportPrivateKey(ctx context.Context, userID int64, walletID string) (string, error)
	SetPrimaryWallet(ctx context.Context, userID int64, walletID string) error
	DeleteWallet(ctx context.Context, userID int64, walletID string) error
}

// SimpleSettingsService интерфейс для работы с настройками в демонстрационной версии
type SimpleSettingsService interface {
	GetUserSettings(ctx context.Context, userID int64) (*entities.UserSettings, error)
	UpdateSettings(ctx context.Context, userID int64, settings *entities.UserSettings) error
	UpdateSlippage(ctx context.Context, userID int64, buySlippage, sellSlippage float64) error
	UpdateFees(ctx context.Context, userID int64, priorityFee, jitoTip int64) error
	UpdateDefaultSOLAmount(ctx context.Context, userID int64, amount float64) error
	UpdateSellPresets(ctx context.Context, userID int64, preset25, preset50, preset75, preset100 *float64) error
	UpdateBuyPresets(ctx context.Context, userID int64, preset1, preset2, preset3, preset4, preset5 *float64) error
	UpdateSingleBuyPreset(ctx context.Context, userID int64, presetIndex int, amount *float64) error
	UpdateSingleSellPreset(ctx context.Context, userID int64, presetIndex int, amount *float64) error
	ResetToDefaults(ctx context.Context, userID int64) error
}

// SimpleSettingsServiceImpl простая in-memory реализация для демо
type SimpleSettingsServiceImpl struct {
	settings map[int64]*entities.UserSettings
	mu       sync.RWMutex
}

func NewSimpleSettingsService() SimpleSettingsService {
	return &SimpleSettingsServiceImpl{
		settings: make(map[int64]*entities.UserSettings),
	}
}

func (s *SimpleSettingsServiceImpl) GetUserSettings(ctx context.Context, userID int64) (*entities.UserSettings, error) {
	s.mu.RLock()
	if settings, exists := s.settings[userID]; exists {
		s.mu.RUnlock()
		return settings, nil
	}
	s.mu.RUnlock()

	// Создаем дефолтные настройки если их нет
	defaultSettings := entities.NewUserSettings(userID)

	s.mu.Lock()
	// Проверяем еще раз, возможно другая горутина уже создала настройки
	if existingSettings, exists := s.settings[userID]; exists {
		s.mu.Unlock()
		return existingSettings, nil
	}
	s.settings[userID] = defaultSettings
	s.mu.Unlock()

	return defaultSettings, nil
}

func (s *SimpleSettingsServiceImpl) UpdateSettings(ctx context.Context, userID int64, settings *entities.UserSettings) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := settings.Validate(); err != nil {
		return fmt.Errorf("invalid settings: %w", err)
	}

	s.settings[userID] = settings
	return nil
}

func (s *SimpleSettingsServiceImpl) UpdateSlippage(ctx context.Context, userID int64, buySlippage, sellSlippage float64) error {
	settings, err := s.GetUserSettings(ctx, userID)
	if err != nil {
		return err
	}

	settings.UpdateSlippage(buySlippage, sellSlippage)
	return s.UpdateSettings(ctx, userID, settings)
}

func (s *SimpleSettingsServiceImpl) UpdateFees(ctx context.Context, userID int64, priorityFee, jitoTip int64) error {
	settings, err := s.GetUserSettings(ctx, userID)
	if err != nil {
		return err
	}

	settings.UpdateFees(priorityFee, jitoTip)
	return s.UpdateSettings(ctx, userID, settings)
}

func (s *SimpleSettingsServiceImpl) UpdateDefaultSOLAmount(ctx context.Context, userID int64, amount float64) error {
	settings, err := s.GetUserSettings(ctx, userID)
	if err != nil {
		return err
	}

	settings.UpdateDefaultSOLAmount(amount)
	return s.UpdateSettings(ctx, userID, settings)
}

func (s *SimpleSettingsServiceImpl) UpdateSellPresets(ctx context.Context, userID int64, preset25, preset50, preset75, preset100 *float64) error {
	settings, err := s.GetUserSettings(ctx, userID)
	if err != nil {
		return err
	}

	settings.UpdateSellPresets(preset25, preset50, preset75, preset100)
	return s.UpdateSettings(ctx, userID, settings)
}

func (s *SimpleSettingsServiceImpl) UpdateBuyPresets(ctx context.Context, userID int64, preset1, preset2, preset3, preset4, preset5 *float64) error {
	settings, err := s.GetUserSettings(ctx, userID)
	if err != nil {
		return err
	}

	settings.UpdateBuyPresets(preset1, preset2, preset3, preset4, preset5)
	return s.UpdateSettings(ctx, userID, settings)
}

func (s *SimpleSettingsServiceImpl) UpdateSingleBuyPreset(ctx context.Context, userID int64, presetIndex int, amount *float64) error {
	settings, err := s.GetUserSettings(ctx, userID)
	if err != nil {
		return err
	}

	settings.UpdateSingleBuyPreset(presetIndex, amount)
	return s.UpdateSettings(ctx, userID, settings)
}

func (s *SimpleSettingsServiceImpl) UpdateSingleSellPreset(ctx context.Context, userID int64, presetIndex int, amount *float64) error {
	settings, err := s.GetUserSettings(ctx, userID)
	if err != nil {
		return err
	}

	settings.UpdateSingleSellPreset(presetIndex, amount)
	return s.UpdateSettings(ctx, userID, settings)
}

func (s *SimpleSettingsServiceImpl) ResetToDefaults(ctx context.Context, userID int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	defaultSettings := entities.NewUserSettings(userID)
	s.settings[userID] = defaultSettings
	return nil
}

// SimpleWalletServiceImpl реализует базовый функционал кошелька для демонстрации
type SimpleWalletServiceImpl struct {
	wallets map[int64][]SimpleWallet
	nextID  int64
	mu      sync.RWMutex
}

func NewSimpleWalletService() SimpleWalletService {
	return &SimpleWalletServiceImpl{
		wallets: make(map[int64][]SimpleWallet),
		nextID:  1,
	}
}

func (s *SimpleWalletServiceImpl) GetWallets(ctx context.Context, userID int64) ([]SimpleWallet, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if wallets, exists := s.wallets[userID]; exists {
		return wallets, nil
	}

	// Return empty slice if no wallets - don't auto-create here
	return []SimpleWallet{}, nil
}

func (s *SimpleWalletServiceImpl) CreateWallet(ctx context.Context, userID int64, name string) (*SimpleWallet, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Генерируем случайный публичный ключ для демонстрации
	publicKeys := []string{
		"7xKXtg2CW87d97TXJSDpbD5jBkheTqA83TZRuJosgAsU",
		"9WzDXwBbmkg8ZTbNMqUxvQRAyrZzDsGYdLVL9zYtAWWM",
		"ATokenGPvbdGVxr1b2hvZbsiqW5xWH25efTNsLJA8knL",
		"EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
		"So11111111111111111111111111111111111111112",
	}

	// Генерируем соответствующие приватные ключи для демонстрации
	privateKeys := []string{
		"3YSWjQGfUd4eDUV2FtHKYgwb3JzCX7Jg2m6NZrBbZuM7kQ3PsJXhTr4nC8Vx5KLwMgE9DpFqZ1BaH6RyNvU2SfW8",
		"5KqM9Hf7QgBv2YnXs4Lp8Tu1J3Dx6ZrF2CvNmW5BgP8kA7SfMt9EqLv3DjG1UzRyHx2QzJ6Kf4NcP5RsY9LwMgE",
		"4HgF3Qx8Lv2Kz7Rw1Pj9Mx5Jy6Nt4Cz8Rw1Pj9Mx5Jy6Nt4Cz8Rw1Pj9Mx5Jy6Nt4Cz8Rw1Pj9Mx5Jy6Nt",
		"2Nv7Dx5Kz3Rw8Pj1Mx9Jy4Nt6Cz5Rw2Pj8Mx7Jy3Nt9Cz4Rw1Pj5Mx6Jy2Nt8Cz3Rw7Pj4Mx1Jy",
		"1So3Mp8Qx5Lv2Kz7Rw1Pj9Mx5Jy6Nt4Cz8Rw1Pj9Mx5Jy6Nt4Cz8Rw1Pj9Mx5Jy6Nt4Cz8Rw",
	}

	randomIndex := rand.Intn(len(publicKeys))
	randomPublicKey := publicKeys[randomIndex]
	randomPrivateKey := privateKeys[randomIndex]

	// Check if this is the first wallet for the user
	isFirstWallet := len(s.wallets[userID]) == 0

	newWallet := SimpleWallet{
		ID:         fmt.Sprintf("%d", s.nextID),
		UserID:     userID,
		Name:       name,
		PublicKey:  randomPublicKey,
		PrivateKey: randomPrivateKey,
		IsDefault:  isFirstWallet, // First wallet becomes default
		CreatedAt:  time.Now(),
	}
	s.nextID++

	if s.wallets[userID] == nil {
		s.wallets[userID] = []SimpleWallet{}
	}
	s.wallets[userID] = append(s.wallets[userID], newWallet)

	return &newWallet, nil
}

func (s *SimpleWalletServiceImpl) GetBalance(ctx context.Context, walletID string) (float64, error) {
	// Генерируем случайный баланс для демонстрации работы refresh
	// Базовое значение + небольшое случайное отклонение
	baseBalance := 0.5
	randomVariation := (rand.Float64() - 0.5) * 0.2 // ±0.1 SOL вариация

	balance := baseBalance + randomVariation
	if balance < 0 {
		balance = 0.01 // Минимальный баланс
	}

	return balance, nil
}

func (s *SimpleWalletServiceImpl) ExportPrivateKey(ctx context.Context, userID int64, walletID string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Find the wallet by ID across all users
	for _, userWallets := range s.wallets {
		for _, wallet := range userWallets {
			if wallet.ID == walletID {
				return wallet.PrivateKey, nil
			}
		}
	}

	return "", fmt.Errorf("wallet with ID %s not found", walletID)
}

// SimpleServiceContainer holds simplified services for Phase 1A demonstration
type SimpleServiceContainer struct {
	walletService    SimpleWalletService
	settingsService  SimpleSettingsService
	portfolioService PortfolioService         // Added for positions management
	tokenDataService TokenDataService         // Added for DexScreener integration
	solPriceService  SolPriceService          // Added for SOL price fetching
	tokenSwapService *TokenSwapService        // Added for TDD token swap functionality
	adapter          *ServiceContainerAdapter // For accessing real PnL data
	pnlTracker       PnLTracker               // NEW: PnL Tracking system

	// NEW: Modular Blockchain Services Integration
	mainContainer *ServiceContainer // Доступ к полной ServiceContainer с модульными сервисами
}

func (s *SimpleServiceContainer) GetWalletService() SimpleWalletService {
	return s.walletService
}

func (s *SimpleServiceContainer) GetSettingsService() SimpleSettingsService {
	return s.settingsService
}

// GetTokenDataService returns the token data service for DexScreener integration
func (s *SimpleServiceContainer) GetTokenDataService() TokenDataService {
	return s.tokenDataService
}

// GetSolPriceService returns the SOL price service
func (s *SimpleServiceContainer) GetSolPriceService() SolPriceService {
	return s.solPriceService
}

// GetWalletPnL возвращает PnL для кошелька (только для адаптера с реальной БД)
func (s *SimpleServiceContainer) GetWalletPnL(ctx context.Context, walletID int64) (float64, error) {
	// Если есть адаптер, используем реальные данные
	if s.adapter != nil {
		return s.adapter.GetWalletPnL(ctx, walletID)
	}
	// Заглушка для простого контейнера - возвращает 0
	return 0, nil
}

// GetPortfolioService returns the portfolio service (через adapter или простую реализацию)
func (s *SimpleServiceContainer) GetPortfolioService() PortfolioService {
	if s.adapter != nil {
		return s.adapter.GetPortfolioService()
	}
	return s.portfolioService
}

func NewSimpleServiceContainer() (*SimpleServiceContainer, error) {
	// Создаем базовый TokenSwapService для demo
	// В реальной системе он будет создаваться с реальными зависимостями
	tokenSwapService := NewTokenSwapService(nil, nil)

	// Создаем логгер для PnL Tracker
	logger := log.New(log.Writer(), "[PnL] ", log.LstdFlags|log.Lshortfile)

	// Создаем PnL Tracker с заглушками для demo режима
	// В реальном режиме эти зависимости будут установлены через adapter
	pnlTracker := NewPnLTracker(nil, nil, nil, logger)

	return &SimpleServiceContainer{
		walletService:    NewSimpleWalletService(),
		settingsService:  NewSimpleSettingsService(),
		portfolioService: NewSimplePortfolioService(),
		tokenSwapService: tokenSwapService,
		pnlTracker:       pnlTracker,
	}, nil
}

func (s *SimpleServiceContainer) Close() error {
	// Cleanup if needed
	return nil
}

func (s *SimpleWalletServiceImpl) SetPrimaryWallet(ctx context.Context, userID int64, walletID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	userWallets, exists := s.wallets[userID]
	if !exists {
		return fmt.Errorf("no wallets found for user %d", userID)
	}

	// Find target wallet and reset all primary flags
	var targetFound bool
	for i := range userWallets {
		if userWallets[i].ID == walletID {
			userWallets[i].IsDefault = true
			targetFound = true
		} else {
			userWallets[i].IsDefault = false
		}
	}

	if !targetFound {
		return fmt.Errorf("wallet with ID %s not found for user %d", walletID, userID)
	}

	// Update the wallets map
	s.wallets[userID] = userWallets
	return nil
}

func (s *SimpleWalletServiceImpl) DeleteWallet(ctx context.Context, userID int64, walletID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	userWallets, exists := s.wallets[userID]
	if !exists {
		return fmt.Errorf("no wallets found for user %d", userID)
	}

	// Cannot delete if only one wallet exists
	if len(userWallets) == 1 {
		return fmt.Errorf("cannot delete the last wallet")
	}

	// Find and remove the target wallet
	var updatedWallets []SimpleWallet
	var targetFound bool
	var deletedWasPrimary bool

	for _, wallet := range userWallets {
		if wallet.ID == walletID {
			targetFound = true
			deletedWasPrimary = wallet.IsDefault
		} else {
			updatedWallets = append(updatedWallets, wallet)
		}
	}

	if !targetFound {
		return fmt.Errorf("wallet with ID %s not found for user %d", walletID, userID)
	}

	// If deleted wallet was primary, make the first remaining wallet primary
	if deletedWasPrimary && len(updatedWallets) > 0 {
		updatedWallets[0].IsDefault = true
	}

	// Update the wallets map
	s.wallets[userID] = updatedWallets
	return nil
}

// GetTokenSwapService returns the token swap service for executing token swaps
func (s *SimpleServiceContainer) GetTokenSwapService() *TokenSwapService {
	// DEBUG: Добавляем логи для отладки
	log.Printf("🔍 DEBUG GetTokenSwapService: adapter=%v, tokenSwapService=%v", s.adapter != nil, s.tokenSwapService != nil)

	// Если есть адаптер, используем реальный сервис
	if s.adapter != nil {
		log.Printf("🔍 DEBUG: Using adapter.GetTokenSwapService()")
		return s.adapter.GetTokenSwapService()
	}
	// Возвращаем собственный TokenSwapService
	log.Printf("🔍 DEBUG: Using own tokenSwapService (DEMO mode)")
	return s.tokenSwapService
}

// NEW: Modular Blockchain Services Access
func (s *SimpleServiceContainer) GetTradingService() interface{} {
	if s.mainContainer != nil {
		return s.mainContainer.GetTradingService()
	}
	return nil
}

func (s *SimpleServiceContainer) GetJupiterService() interface{} {
	if s.mainContainer != nil {
		return s.mainContainer.GetJupiterService()
	}
	return nil
}

func (s *SimpleServiceContainer) GetJitoService() interface{} {
	if s.mainContainer != nil {
		return s.mainContainer.GetJitoService()
	}
	return nil
}

func (s *SimpleServiceContainer) GetSolanaService() interface{} {
	if s.mainContainer != nil {
		return s.mainContainer.GetSolanaService()
	}
	return nil
}

// SetMainContainer устанавливает основной ServiceContainer с модульными сервисами
func (s *SimpleServiceContainer) SetMainContainer(container *ServiceContainer) {
	s.mainContainer = container
}

// GetPnLTracker возвращает PnL Tracker для отслеживания прибыли/убытков
func (s *SimpleServiceContainer) GetPnLTracker() PnLTracker {
	return s.pnlTracker
}

// SimplePortfolioService - простая in-memory реализация для демо режима
type SimplePortfolioService struct {
	positions map[int64][]*entities.TokenPosition // walletID -> positions
	nextID    int64
	mu        sync.RWMutex
}

func NewSimplePortfolioService() PortfolioService {
	return &SimplePortfolioService{
		positions: make(map[int64][]*entities.TokenPosition),
		nextID:    1,
	}
}

func (s *SimplePortfolioService) OpenPosition(ctx context.Context, req OpenPositionRequest) (*entities.TokenPosition, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	position := entities.NewTokenPosition(req.WalletID, req.TokenAddress, req.TokenSymbol, req.Amount, req.EntryPrice)
	position.ID = s.nextID
	s.nextID++

	if s.positions[req.WalletID] == nil {
		s.positions[req.WalletID] = make([]*entities.TokenPosition, 0)
	}
	s.positions[req.WalletID] = append(s.positions[req.WalletID], position)

	return position, nil
}

func (s *SimplePortfolioService) ClosePosition(ctx context.Context, positionID int64, walletID int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	positions := s.positions[walletID]
	for i, pos := range positions {
		if pos.ID == positionID {
			pos.IsActive = false
			s.positions[walletID][i] = pos
			return nil
		}
	}
	return fmt.Errorf("position not found")
}

func (s *SimplePortfolioService) UpdatePositionAmount(ctx context.Context, positionID int64, newAmount float64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for walletID, positions := range s.positions {
		for i, pos := range positions {
			if pos.ID == positionID {
				pos.Amount = newAmount
				s.positions[walletID][i] = pos
				return nil
			}
		}
	}
	return fmt.Errorf("position not found")
}

func (s *SimplePortfolioService) GetPosition(ctx context.Context, positionID int64) (*entities.TokenPosition, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, positions := range s.positions {
		for _, pos := range positions {
			if pos.ID == positionID {
				return pos, nil
			}
		}
	}
	return nil, fmt.Errorf("position not found")
}

func (s *SimplePortfolioService) GetPositions(ctx context.Context, walletID int64, activeOnly bool) ([]*entities.TokenPosition, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	positions := s.positions[walletID]
	if positions == nil {
		return []*entities.TokenPosition{}, nil
	}

	if !activeOnly {
		return positions, nil
	}

	// Фильтруем только активные позиции
	var activePositions []*entities.TokenPosition
	for _, pos := range positions {
		if pos.IsActive {
			activePositions = append(activePositions, pos)
		}
	}

	return activePositions, nil
}

func (s *SimplePortfolioService) GetPositionsByUser(ctx context.Context, userID int64) ([]*entities.TokenPosition, error) {
	// В демо режиме не реализовано - нужна связь user -> wallets
	return []*entities.TokenPosition{}, nil
}

func (s *SimplePortfolioService) GetActivePositionByToken(ctx context.Context, walletID int64, tokenAddress string) (*entities.TokenPosition, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	positions := s.positions[walletID]
	for _, pos := range positions {
		if pos.IsActive && pos.TokenAddress == tokenAddress {
			return pos, nil
		}
	}
	return nil, fmt.Errorf("active position not found for token %s", tokenAddress)
}

func (s *SimplePortfolioService) UpdatePositionPrice(ctx context.Context, positionID int64, newPrice float64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for walletID, positions := range s.positions {
		for i, pos := range positions {
			if pos.ID == positionID {
				pos.CurrentPrice = &newPrice
				s.positions[walletID][i] = pos
				return nil
			}
		}
	}
	return fmt.Errorf("position not found")
}

func (s *SimplePortfolioService) BulkUpdatePrices(ctx context.Context, priceUpdates []PositionPriceUpdate) error {
	// В демо режиме не реализовано
	return nil
}

func (s *SimplePortfolioService) RefreshStalePositions(ctx context.Context, limit int) error {
	// В демо режиме не реализовано
	return nil
}

func (s *SimplePortfolioService) CalculatePnL(ctx context.Context, walletID int64) (*PnLSummary, error) {
	// В демо режиме возвращаем заглушку
	return &PnLSummary{
		WalletID:             walletID,
		TotalActivePositions: 0,
		TotalPnLUSD:          0,
	}, nil
}

func (s *SimplePortfolioService) GetTopPositions(ctx context.Context, walletID int64, limit int) ([]*entities.TokenPosition, error) {
	positions, err := s.GetPositions(ctx, walletID, true)
	if err != nil {
		return nil, err
	}

	if len(positions) <= limit {
		return positions, nil
	}

	return positions[:limit], nil
}

func (s *SimplePortfolioService) GetPortfolioSummary(ctx context.Context, userID int64) (*PortfolioSummary, error) {
	// В демо режиме возвращаем заглушку
	return &PortfolioSummary{
		UserID:         userID,
		TotalWallets:   1,
		TotalPositions: 0,
		TotalValue:     0,
		TotalPnL:       0,
	}, nil
}
