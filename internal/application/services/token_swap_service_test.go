package services

import (
	"cash-farmer/internal/infrastructure/api"
	"context"
	"testing"
	"time"
)

func TestTokenSwapService_GetDefaultPresets(t *testing.T) {
	// Create a mock transaction manager and jito client for testing
	mockTxManager := &TransactionManager{}
	mockJitoClient := &MockJitoClient{}

	service := NewTokenSwapService(mockTxManager, mockJitoClient)
	presets := service.GetDefaultPresets()

	// Test that all expected presets exist
	expectedPresets := []string{"conservative", "balanced", "aggressive", "mev_protected", "fast_execution"}

	for _, expectedID := range expectedPresets {
		preset, exists := presets[expectedID]
		if !exists {
			t.Errorf("Expected preset '%s' not found", expectedID)
			continue
		}

		if preset.ID != expectedID {
			t.Errorf("Preset ID mismatch: expected '%s', got '%s'", expectedID, preset.ID)
		}

		if preset.Name == "" {
			t.Errorf("Preset '%s' has empty name", expectedID)
		}

		if preset.Description == "" {
			t.Errorf("Preset '%s' has empty description", expectedID)
		}

		if preset.DefaultSlippageBps <= 0 {
			t.Errorf("Preset '%s' has invalid default slippage: %d", expectedID, preset.DefaultSlippageBps)
		}

		if preset.MaxSlippageBps <= preset.DefaultSlippageBps {
			t.Errorf("Preset '%s' max slippage (%d) should be greater than default (%d)",
				expectedID, preset.MaxSlippageBps, preset.DefaultSlippageBps)
		}

		if preset.TimeoutSeconds <= 0 {
			t.Errorf("Preset '%s' has invalid timeout: %d", expectedID, preset.TimeoutSeconds)
		}

		if preset.MaxRetries <= 0 {
			t.Errorf("Preset '%s' has invalid max retries: %d", expectedID, preset.MaxRetries)
		}
	}
}

func TestTokenSwapService_DetermineSwapPreset(t *testing.T) {
	mockTxManager := &TransactionManager{}
	mockJitoClient := &MockJitoClient{}
	service := NewTokenSwapService(mockTxManager, mockJitoClient)

	// Test with specific preset ID in request
	req := SwapRequest{
		PresetID: "aggressive",
	}

	preset, err := service.determineSwapPreset(req, nil)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if preset.ID != "aggressive" {
		t.Errorf("Expected preset 'aggressive', got '%s'", preset.ID)
	}

	// Test with user preferred preset
	userSettings := &UserSwapSettings{
		PreferredPresetID: "conservative",
	}

	req2 := SwapRequest{} // No preset specified
	preset2, err := service.determineSwapPreset(req2, userSettings)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if preset2.ID != "conservative" {
		t.Errorf("Expected preset 'conservative', got '%s'", preset2.ID)
	}

	// Test default behavior (should return "balanced")
	req3 := SwapRequest{} // No preset specified
	preset3, err := service.determineSwapPreset(req3, nil)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if preset3.ID != "balanced" {
		t.Errorf("Expected default preset 'balanced', got '%s'", preset3.ID)
	}
}

func TestTokenSwapService_ValidateSwapRequest(t *testing.T) {
	mockTxManager := &TransactionManager{}
	mockJitoClient := &MockJitoClient{}
	service := NewTokenSwapService(mockTxManager, mockJitoClient)

	preset := &SwapPreset{
		MaxSlippageBps: 500,
	}

	// Test amount validation
	userSettings := &UserSwapSettings{
		MaxAmountPerSwap: func() *int64 { v := int64(1000000); return &v }(), // 1M lamports limit
	}

	req := SwapRequest{
		AmountLamports: 2000000, // 2M lamports - exceeds limit
	}

	err := service.validateSwapRequest(req, userSettings, preset)
	if err == nil {
		t.Error("Expected error for amount exceeding limit")
	}

	// Test valid amount
	req2 := SwapRequest{
		AmountLamports: 500000, // 0.5M lamports - within limit
	}

	err2 := service.validateSwapRequest(req2, userSettings, preset)
	if err2 != nil {
		t.Errorf("Expected no error for valid amount, got: %v", err2)
	}

	// Test token whitelist validation
	userSettings2 := &UserSwapSettings{
		WhitelistedTokens: []string{"EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v"}, // USDC
	}

	req3 := SwapRequest{
		OutputMint: "So11111111111111111111111111111111111111112", // SOL - not in whitelist
	}

	err3 := service.validateSwapRequest(req3, userSettings2, preset)
	if err3 == nil {
		t.Error("Expected error for token not in whitelist")
	}

	// Test valid token in whitelist
	req4 := SwapRequest{
		OutputMint: "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v", // USDC - in whitelist
	}

	err4 := service.validateSwapRequest(req4, userSettings2, preset)
	if err4 != nil {
		t.Errorf("Expected no error for whitelisted token, got: %v", err4)
	}
}

// MockJitoClient for testing
type MockJitoClient struct{}

// Implement required interface methods
func (m *MockJitoClient) SendBundle(ctx context.Context, transactions []string) (*api.JitoBundleResult, error) {
	return &api.JitoBundleResult{BundleID: "mock-bundle-id"}, nil
}

func (m *MockJitoClient) GetBundleStatuses(ctx context.Context, bundleIDs []string) ([]api.JitoBundleStatus, error) {
	return []api.JitoBundleStatus{}, nil
}

func (m *MockJitoClient) GetBundleStatus(ctx context.Context, bundleID string) (*api.JitoBundleStatus, error) {
	return &api.JitoBundleStatus{BundleID: bundleID, Status: "confirmed"}, nil
}

func (m *MockJitoClient) GetInflightBundleStatuses(ctx context.Context) ([]api.JitoInflightBundle, error) {
	return []api.JitoInflightBundle{}, nil
}

func (m *MockJitoClient) GetTipAccounts(ctx context.Context) ([]string, error) {
	return []string{"mock-tip-account"}, nil
}

func (m *MockJitoClient) WaitForBundleConfirmation(ctx context.Context, bundleID string, timeout time.Duration) (*api.JitoBundleStatus, error) {
	return &api.JitoBundleStatus{BundleID: bundleID, Status: "confirmed"}, nil
}

func (m *MockJitoClient) ValidateBundle(transactions []string) error {
	return nil
}
