package jito

import (
	"context"
	"time"
)

// LegacyJitoClient полностью имитирует api.JitoClient но использует EnhancedJitoClient внутри
// Это обеспечивает полную backward compatibility
type LegacyJitoClient struct {
	enhanced JitoClientInterface
	rpcURL   string // Совместимость с api.JitoClient
}

// NewJitoClient создает новый legacy compatible Jito клиент
// Эта функция заменяет api.NewJitoClient с enhanced функциональностью
func NewJitoClient(rpcURL string) *LegacyJitoClient {
	config := &Config{
		BaseURL: rpcURL,
		UUID:    "",
		Debug:   false,
	}
	enhanced := NewEnhancedJitoClient(config)

	return &LegacyJitoClient{
		enhanced: enhanced,
		rpcURL:   rpcURL,
	}
}

// NewJitoClientFromEnhanced создает legacy клиент из enhanced клиента
func NewJitoClientFromEnhanced(enhanced JitoClientInterface, rpcURL string) *LegacyJitoClient {
	return &LegacyJitoClient{
		enhanced: enhanced,
		rpcURL:   rpcURL,
	}
}

// SendBundle отправляет bundle транзакций в Jito
func (c *LegacyJitoClient) SendBundle(ctx context.Context, transactions []string) (*JitoBundleResult, error) {
	result, err := c.enhanced.SendBundle(ctx, transactions)
	if err != nil {
		return nil, err
	}

	return &JitoBundleResult{
		BundleID: result.BundleID,
	}, nil
}

// GetBundleStatuses получает статусы нескольких bundles
func (c *LegacyJitoClient) GetBundleStatuses(ctx context.Context, bundleIDs []string) ([]JitoBundleStatus, error) {
	statuses, err := c.enhanced.GetBundleStatuses(ctx, bundleIDs)
	if err != nil {
		return nil, err
	}

	result := make([]JitoBundleStatus, len(statuses))
	for i, status := range statuses {
		result[i] = JitoBundleStatus{
			BundleID:     status.BundleID,
			Status:       status.Status,
			LandedSlot:   status.LandedSlot,
			Transactions: status.Transactions,
			Error:        status.Error,
		}
	}

	return result, nil
}

// GetBundleStatus получает статус одного bundle
func (c *LegacyJitoClient) GetBundleStatus(ctx context.Context, bundleID string) (*JitoBundleStatus, error) {
	status, err := c.enhanced.GetBundleStatus(ctx, bundleID)
	if err != nil {
		return nil, err
	}

	return &JitoBundleStatus{
		BundleID:     status.BundleID,
		Status:       status.Status,
		LandedSlot:   status.LandedSlot,
		Transactions: status.Transactions,
		Error:        status.Error,
	}, nil
}

// GetInflightBundleStatuses получает все inflight bundles
func (c *LegacyJitoClient) GetInflightBundleStatuses(ctx context.Context) ([]JitoInflightBundle, error) {
	bundles, err := c.enhanced.GetInflightBundleStatuses(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]JitoInflightBundle, len(bundles))
	for i, bundle := range bundles {
		result[i] = JitoInflightBundle{
			BundleID:     bundle.BundleID,
			Transactions: bundle.Transactions,
			Slot:         bundle.Slot,
		}
	}

	return result, nil
}

// GetTipAccounts получает Jito tip аккаунты для priority fees
func (c *LegacyJitoClient) GetTipAccounts(ctx context.Context) ([]string, error) {
	return c.enhanced.GetTipAccounts(ctx)
}

// WaitForBundleConfirmation ждет подтверждения bundle с timeout
func (c *LegacyJitoClient) WaitForBundleConfirmation(ctx context.Context, bundleID string, timeout time.Duration) (*JitoBundleStatus, error) {
	status, err := c.enhanced.WaitForBundleConfirmation(ctx, bundleID, timeout)
	if err != nil {
		return nil, err
	}

	return &JitoBundleStatus{
		BundleID:     status.BundleID,
		Status:       status.Status,
		LandedSlot:   status.LandedSlot,
		Transactions: status.Transactions,
		Error:        status.Error,
	}, nil
}

// ValidateBundle выполняет базовую валидацию bundle перед отправкой
func (c *LegacyJitoClient) ValidateBundle(transactions []string) error {
	return c.enhanced.ValidateBundle(transactions)
}

// ENHANCED МЕТОДЫ - дополнительная функциональность

// GetEnhancedClient возвращает underlying enhanced клиент для продвинутых фич
func (c *LegacyJitoClient) GetEnhancedClient() JitoClientInterface {
	return c.enhanced
}

// SetUUID включает UUID аутентификацию (enhanced фича)
func (c *LegacyJitoClient) SetUUID(uuid string) {
	c.enhanced.SetUUID(uuid)
}

// SetDebug включает debug режим (enhanced фича)
func (c *LegacyJitoClient) SetDebug(enabled bool) {
	c.enhanced.SetDebug(enabled)
}

// GetRandomTipAccount возвращает random tip аккаунт (enhanced фича)
func (c *LegacyJitoClient) GetRandomTipAccount(ctx context.Context) (*TipAccount, error) {
	return c.enhanced.GetRandomTipAccount(ctx)
}

// SendBundleOnly отправляет bundle в bundle-only режиме (enhanced фича)
func (c *LegacyJitoClient) SendBundleOnly(ctx context.Context, transactions []string) (*JitoBundleResult, error) {
	result, err := c.enhanced.SendBundleOnly(ctx, transactions)
	if err != nil {
		return nil, err
	}

	return &JitoBundleResult{
		BundleID: result.BundleID,
	}, nil
}

// SendTransaction отправляет одну транзакцию с bundle-only опцией (enhanced фича)
func (c *LegacyJitoClient) SendTransaction(ctx context.Context, txData string, bundleOnly bool) (string, error) {
	return c.enhanced.SendTransaction(ctx, txData, bundleOnly)
}

// GetCapabilities возвращает возможности клиента (enhanced фича)
func (c *LegacyJitoClient) GetCapabilities() ClientCapabilities {
	if enhancedClient, ok := c.enhanced.(*EnhancedJitoClient); ok {
		return enhancedClient.GetCapabilities()
	}

	return ClientCapabilities{
		SupportsUUIDAuth:   true,
		SupportsBundleOnly: true,
		SupportsRandomTips: true,
		MaxBundleSize:      5,
		DefaultMode:        ModeStandard,
	}
}
