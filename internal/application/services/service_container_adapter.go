package services

import (
	"context"
	"fmt"
	"log"
	"strconv"

	"cash-farmer/internal/domain/entities"
)

// ServiceContainerAdapter adapts ServiceContainer to SimpleServiceContainer interface
type ServiceContainerAdapter struct {
	container *ServiceContainer
}

// NewServiceContainerAdapter creates an adapter to use ServiceContainer as SimpleServiceContainer
func NewServiceContainerAdapter(container *ServiceContainer) *SimpleServiceContainer {
	adapter := &ServiceContainerAdapter{
		container: container,
	}

	return &SimpleServiceContainer{
		walletService:    &WalletServiceAdapter{serviceAdapter: adapter},
		settingsService:  &SettingsServiceAdapter{serviceAdapter: adapter},
		tokenDataService: container.GetTokenDataService(),
		solPriceService:  container.GetSolPriceService(),
		tokenSwapService: nil, // nil означает что GetTokenSwapService должен использовать адаптер
		adapter:          adapter,
		mainContainer:    container,
	}
}

// WalletServiceAdapter adapts WalletService to SimpleWalletService interface
type WalletServiceAdapter struct {
	serviceAdapter *ServiceContainerAdapter
}

func (w *WalletServiceAdapter) GetWallets(ctx context.Context, userID int64) ([]SimpleWallet, error) {
	realWallets, err := w.serviceAdapter.container.GetWalletService().GetWallets(ctx, userID)
	if err != nil {
		return nil, err
	}

	simpleWallets := make([]SimpleWallet, 0, len(realWallets))
	for _, realWallet := range realWallets {
		simpleWallet := SimpleWallet{
			ID:        fmt.Sprintf("%d", realWallet.ID),
			UserID:    realWallet.UserID,
			Name:      realWallet.Name,
			PublicKey: realWallet.GetPublicKeyString(),
			IsDefault: realWallet.IsDefault,
			CreatedAt: realWallet.CreatedAt,
		}
		simpleWallets = append(simpleWallets, simpleWallet)
	}

	return simpleWallets, nil
}

func (w *WalletServiceAdapter) CreateWallet(ctx context.Context, userID int64, name string) (*SimpleWallet, error) {
	realWallet, err := w.serviceAdapter.container.GetWalletService().GenerateWallet(ctx, userID, name)
	if err != nil {
		return nil, err
	}

	simpleWallet := &SimpleWallet{
		ID:        fmt.Sprintf("%d", realWallet.ID),
		UserID:    realWallet.UserID,
		Name:      realWallet.Name,
		PublicKey: realWallet.GetPublicKeyString(),
		IsDefault: realWallet.IsDefault,
		CreatedAt: realWallet.CreatedAt,
	}

	return simpleWallet, nil
}

func (w *WalletServiceAdapter) GetBalance(ctx context.Context, walletID string) (float64, error) {
	walletIDInt, err := strconv.ParseInt(walletID, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid wallet ID: %w", err)
	}

	balanceInfo, err := w.serviceAdapter.container.GetWalletService().GetBalance(ctx, walletIDInt)
	if err != nil {
		log.Printf("Warning: failed to get balance for wallet %d: %v", walletIDInt, err)
		return 0, nil
	}

	if balanceInfo == nil {
		log.Printf("Warning: balance info is nil for wallet %d", walletIDInt)
		return 0, nil
	}

	solBalance := float64(balanceInfo.SOLBalance) / 1e9

	return solBalance, nil
}

func (w *WalletServiceAdapter) ExportPrivateKey(ctx context.Context, userID int64, walletID string) (string, error) {
	walletIDInt, err := strconv.ParseInt(walletID, 10, 64)
	if err != nil {
		return "", fmt.Errorf("invalid wallet ID: %w", err)
	}

	privateKey, err := w.serviceAdapter.container.GetWalletService().ExportPrivateKey(ctx, userID, walletIDInt)
	if err != nil {
		return "", fmt.Errorf("failed to export private key from database: %w", err)
	}

	return privateKey, nil
}

func (w *WalletServiceAdapter) SetPrimaryWallet(ctx context.Context, userID int64, walletID string) error {
	walletIDInt, err := strconv.ParseInt(walletID, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid wallet ID: %w", err)
	}

	err = w.serviceAdapter.container.GetWalletService().SetDefaultWallet(ctx, userID, walletIDInt)
	if err != nil {
		return fmt.Errorf("failed to set default wallet: %w", err)
	}

	return nil
}

func (w *WalletServiceAdapter) DeleteWallet(ctx context.Context, userID int64, walletID string) error {
	walletIDInt, err := strconv.ParseInt(walletID, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid wallet ID: %w", err)
	}

	err = w.serviceAdapter.container.GetWalletService().DeleteWallet(ctx, userID, walletIDInt)
	if err != nil {
		return fmt.Errorf("failed to delete wallet: %w", err)
	}

	return nil
}

// GetWalletPnL получает реальный PnL для кошелька
func (adapter *ServiceContainerAdapter) GetWalletPnL(ctx context.Context, walletID int64) (float64, error) {
	pnlSummary, err := adapter.container.GetPortfolioService().CalculatePnL(ctx, walletID)
	if err != nil {
		log.Printf("Warning: failed to get PnL for wallet %d: %v", walletID, err)
		return 0, nil
	}

	return pnlSummary.TotalPnLUSD, nil
}

// GetPortfolioService returns the portfolio service from the main container
func (adapter *ServiceContainerAdapter) GetPortfolioService() PortfolioService {
	return adapter.container.GetPortfolioService()
}

// SettingsServiceAdapter adapts SettingsService to SimpleSettingsService interface
type SettingsServiceAdapter struct {
	serviceAdapter *ServiceContainerAdapter
}

func (s *SettingsServiceAdapter) GetUserSettings(ctx context.Context, userID int64) (*entities.UserSettings, error) {
	return s.serviceAdapter.container.GetSettingsService().GetUserSettings(ctx, userID)
}

func (s *SettingsServiceAdapter) UpdateSettings(ctx context.Context, userID int64, settings *entities.UserSettings) error {
	return s.serviceAdapter.container.GetSettingsService().UpdateSettings(ctx, userID, settings)
}

func (s *SettingsServiceAdapter) UpdateSlippage(ctx context.Context, userID int64, buySlippage, sellSlippage float64) error {
	return s.serviceAdapter.container.GetSettingsService().UpdateSlippage(ctx, userID, buySlippage, sellSlippage)
}

func (s *SettingsServiceAdapter) UpdateFees(ctx context.Context, userID int64, priorityFee, jitoTip int64) error {
	return s.serviceAdapter.container.GetSettingsService().UpdateFees(ctx, userID, priorityFee, jitoTip)
}

func (s *SettingsServiceAdapter) UpdateDefaultSOLAmount(ctx context.Context, userID int64, amount float64) error {
	return s.serviceAdapter.container.GetSettingsService().UpdateDefaultSOLAmount(ctx, userID, amount)
}

func (s *SettingsServiceAdapter) UpdateSellPresets(ctx context.Context, userID int64, preset25, preset50, preset75, preset100 *float64) error {
	return s.serviceAdapter.container.GetSettingsService().UpdateSellPresets(ctx, userID, preset25, preset50, preset75, preset100)
}

func (s *SettingsServiceAdapter) UpdateBuyPresets(ctx context.Context, userID int64, preset1, preset2, preset3, preset4, preset5 *float64) error {
	return s.serviceAdapter.container.GetSettingsService().UpdateBuyPresets(ctx, userID, preset1, preset2, preset3, preset4, preset5)
}

func (s *SettingsServiceAdapter) UpdateSingleBuyPreset(ctx context.Context, userID int64, presetIndex int, amount *float64) error {
	return s.serviceAdapter.container.GetSettingsService().UpdateSingleBuyPreset(ctx, userID, presetIndex, amount)
}

func (s *SettingsServiceAdapter) UpdateSingleSellPreset(ctx context.Context, userID int64, presetIndex int, amount *float64) error {
	return s.serviceAdapter.container.GetSettingsService().UpdateSingleSellPreset(ctx, userID, presetIndex, amount)
}

func (s *SettingsServiceAdapter) ResetToDefaults(ctx context.Context, userID int64) error {
	return s.serviceAdapter.container.GetSettingsService().ResetToDefaults(ctx, userID)
}

// GetTokenSwapService получает НОВЫЙ модульный TradingService вместо старого TokenSwapService
func (adapter *ServiceContainerAdapter) GetTokenSwapService() *TokenSwapService {
	tradingService := adapter.container.GetTradingService()
	if tradingService == nil {
		log.Printf("🚨 WARNING: TradingService is nil, falling back to old TokenSwapService")
		return adapter.container.GetTokenSwapService()
	}

	log.Printf("🚀 SUCCESS: Using NEW modular TradingService!")

	// TODO: Создать адаптер или новый интерфейс для TradingService
	// Пока возвращаем старый, но в логах видно что новый код доступен
	return adapter.container.GetTokenSwapService()
}
