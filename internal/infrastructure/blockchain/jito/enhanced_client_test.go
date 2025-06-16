package jito

import (
	"testing"
)

func TestNewEnhancedJitoClient(t *testing.T) {
	config := &Config{
		BaseURL: "https://amsterdam.mainnet.block-engine.jito.wtf/api/v1",
		UUID:    "",
		Debug:   false,
	}

	client := NewEnhancedJitoClient(config)
	if client == nil {
		t.Fatal("Expected client to be created, got nil")
	}

	if client.config.BaseURL != config.BaseURL {
		t.Errorf("Expected BaseURL %s, got %s", config.BaseURL, client.config.BaseURL)
	}
}

func TestNewEnhancedJitoClientFromURL(t *testing.T) {
	rpcURL := "https://amsterdam.mainnet.block-engine.jito.wtf/api/v1"
	client := NewEnhancedJitoClientFromURL(rpcURL)

	if client == nil {
		t.Fatal("Expected client to be created, got nil")
	}

	if client.config.BaseURL != rpcURL {
		t.Errorf("Expected BaseURL %s, got %s", rpcURL, client.config.BaseURL)
	}

	if client.config.UUID != "" {
		t.Errorf("Expected empty UUID for backward compatibility, got %s", client.config.UUID)
	}
}

func TestClientCapabilities(t *testing.T) {
	client := NewEnhancedJitoClientFromURL("https://test.example.com")
	capabilities := client.GetCapabilities()

	if !capabilities.SupportsUUIDAuth {
		t.Error("Expected client to support UUID authentication")
	}

	if !capabilities.SupportsBundleOnly {
		t.Error("Expected client to support bundle-only mode")
	}

	if !capabilities.SupportsRandomTips {
		t.Error("Expected client to support random tip accounts")
	}

	if capabilities.MaxBundleSize != 5 {
		t.Errorf("Expected MaxBundleSize 5, got %d", capabilities.MaxBundleSize)
	}
}

func TestValidateBundle(t *testing.T) {
	client := NewEnhancedJitoClientFromURL("https://test.example.com")

	// Test empty bundle
	err := client.ValidateBundle([]string{})
	if err == nil {
		t.Error("Expected error for empty bundle")
	}

	// Test bundle with too many transactions
	manyTxs := make([]string, 6)
	for i := range manyTxs {
		manyTxs[i] = "valid_transaction_data_that_is_long_enough_to_pass_basic_validation_but_still_fake_for_testing_purposes"
	}
	err = client.ValidateBundle(manyTxs)
	if err == nil {
		t.Error("Expected error for bundle with too many transactions")
	}

	// Test bundle with empty transaction
	err = client.ValidateBundle([]string{""})
	if err == nil {
		t.Error("Expected error for bundle with empty transaction")
	}

	// Test bundle with transaction that's too short
	err = client.ValidateBundle([]string{"short"})
	if err == nil {
		t.Error("Expected error for bundle with transaction that's too short")
	}

	// Test valid bundle
	validTx := "valid_transaction_data_that_is_long_enough_to_pass_basic_validation_but_still_fake_for_testing_purposes_and_more_data"
	err = client.ValidateBundle([]string{validTx})
	if err != nil {
		t.Errorf("Expected no error for valid bundle, got: %v", err)
	}
}

func TestSetUUID(t *testing.T) {
	client := NewEnhancedJitoClientFromURL("https://test.example.com")

	originalUUID := client.config.UUID
	if originalUUID != "" {
		t.Errorf("Expected empty UUID initially, got %s", originalUUID)
	}

	newUUID := "test-uuid-12345"
	client.SetUUID(newUUID)

	if client.config.UUID != newUUID {
		t.Errorf("Expected UUID %s, got %s", newUUID, client.config.UUID)
	}
}

func TestSetDebug(t *testing.T) {
	client := NewEnhancedJitoClientFromURL("https://test.example.com")

	if client.config.Debug {
		t.Error("Expected debug to be false initially")
	}

	client.SetDebug(true)
	if !client.config.Debug {
		t.Error("Expected debug to be true after SetDebug(true)")
	}

	client.SetDebug(false)
	if client.config.Debug {
		t.Error("Expected debug to be false after SetDebug(false)")
	}
}
