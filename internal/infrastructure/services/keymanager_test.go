package services

import (
	"encoding/hex"
	"os"
	"testing"
)

func TestKeyManager_EncryptDecrypt(t *testing.T) {
	// Setup test master key
	testMasterKey := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	os.Setenv("CASH_FARMER_MASTER_KEY", testMasterKey)
	defer os.Unsetenv("CASH_FARMER_MASTER_KEY")

	km, err := NewKeyManager()
	if err != nil {
		t.Fatalf("Failed to create KeyManager: %v", err)
	}

	userID := int64(12345)
	testData := "sensitive_private_key_data"

	// Test encryption
	encrypted, err := km.Encrypt(userID, []byte(testData))
	if err != nil {
		t.Fatalf("Encryption failed: %v", err)
	}

	// Verify encrypted data is not empty
	if len(encrypted.Data) == 0 {
		t.Fatal("Encrypted data is empty")
	}
	if len(encrypted.Nonce) == 0 {
		t.Fatal("Nonce is empty")
	}

	// Test decryption
	decrypted, err := km.Decrypt(userID, encrypted)
	if err != nil {
		t.Fatalf("Decryption failed: %v", err)
	}

	// Verify decrypted data matches original
	if string(decrypted) != testData {
		t.Fatalf("Decrypted data doesn't match: expected %s, got %s", testData, string(decrypted))
	}
}

func TestKeyManager_PrivateKeyEncryption(t *testing.T) {
	// Setup test master key
	testMasterKey := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	os.Setenv("CASH_FARMER_MASTER_KEY", testMasterKey)
	defer os.Unsetenv("CASH_FARMER_MASTER_KEY")

	km, err := NewKeyManager()
	if err != nil {
		t.Fatalf("Failed to create KeyManager: %v", err)
	}

	userID := int64(67890)
	privateKey := "5JrjVJkHdHnNmF9LVx8C2GBGHZJGNjYSY5J8pXvGFWPBqJ8Fmm3"

	// Test private key encryption
	encrypted, err := km.EncryptPrivateKey(userID, privateKey)
	if err != nil {
		t.Fatalf("Private key encryption failed: %v", err)
	}

	// Test private key decryption
	decrypted, err := km.DecryptPrivateKey(userID, encrypted)
	if err != nil {
		t.Fatalf("Private key decryption failed: %v", err)
	}

	if decrypted != privateKey {
		t.Fatalf("Decrypted private key doesn't match: expected %s, got %s", privateKey, decrypted)
	}
}

func TestKeyManager_MnemonicEncryption(t *testing.T) {
	// Setup test master key
	testMasterKey := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	os.Setenv("CASH_FARMER_MASTER_KEY", testMasterKey)
	defer os.Unsetenv("CASH_FARMER_MASTER_KEY")

	km, err := NewKeyManager()
	if err != nil {
		t.Fatalf("Failed to create KeyManager: %v", err)
	}

	userID := int64(11111)
	mnemonic := "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about"

	// Test mnemonic encryption
	encrypted, err := km.EncryptMnemonic(userID, mnemonic)
	if err != nil {
		t.Fatalf("Mnemonic encryption failed: %v", err)
	}

	// Test mnemonic decryption
	decrypted, err := km.DecryptMnemonic(userID, encrypted)
	if err != nil {
		t.Fatalf("Mnemonic decryption failed: %v", err)
	}

	if decrypted != mnemonic {
		t.Fatalf("Decrypted mnemonic doesn't match: expected %s, got %s", mnemonic, decrypted)
	}
}

func TestKeyManager_UserIsolation(t *testing.T) {
	// Setup test master key
	testMasterKey := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	os.Setenv("CASH_FARMER_MASTER_KEY", testMasterKey)
	defer os.Unsetenv("CASH_FARMER_MASTER_KEY")

	km, err := NewKeyManager()
	if err != nil {
		t.Fatalf("Failed to create KeyManager: %v", err)
	}

	user1ID := int64(1)
	user2ID := int64(2)
	testData := "same_data_for_both_users"

	// Encrypt same data for different users
	encrypted1, err := km.Encrypt(user1ID, []byte(testData))
	if err != nil {
		t.Fatalf("Encryption for user1 failed: %v", err)
	}

	encrypted2, err := km.Encrypt(user2ID, []byte(testData))
	if err != nil {
		t.Fatalf("Encryption for user2 failed: %v", err)
	}

	// Verify encrypted data is different for different users
	if hex.EncodeToString(encrypted1.Data) == hex.EncodeToString(encrypted2.Data) {
		t.Fatal("Encrypted data should be different for different users")
	}

	// Verify user1 cannot decrypt user2's data (should fail)
	_, err = km.Decrypt(user1ID, encrypted2)
	if err == nil {
		t.Fatal("User1 should not be able to decrypt user2's data")
	}

	// Verify each user can decrypt their own data
	decrypted1, err := km.Decrypt(user1ID, encrypted1)
	if err != nil {
		t.Fatalf("User1 decryption failed: %v", err)
	}

	decrypted2, err := km.Decrypt(user2ID, encrypted2)
	if err != nil {
		t.Fatalf("User2 decryption failed: %v", err)
	}

	if string(decrypted1) != testData || string(decrypted2) != testData {
		t.Fatal("Decrypted data doesn't match original for one or both users")
	}
}

func TestKeyManager_InvalidMasterKey(t *testing.T) {
	// Test with missing master key
	os.Unsetenv("CASH_FARMER_MASTER_KEY")
	_, err := NewKeyManager()
	if err == nil {
		t.Fatal("Should fail with missing master key")
	}

	// Test with invalid hex master key
	os.Setenv("CASH_FARMER_MASTER_KEY", "invalid_hex")
	defer os.Unsetenv("CASH_FARMER_MASTER_KEY")
	_, err = NewKeyManager()
	if err == nil {
		t.Fatal("Should fail with invalid hex master key")
	}

	// Test with wrong length master key
	os.Setenv("CASH_FARMER_MASTER_KEY", "0123456789abcdef") // Only 16 bytes
	_, err = NewKeyManager()
	if err == nil {
		t.Fatal("Should fail with wrong length master key")
	}
}

func TestGenerateMasterKey(t *testing.T) {
	key, err := GenerateMasterKey()
	if err != nil {
		t.Fatalf("Failed to generate master key: %v", err)
	}

	// Verify key length (64 hex characters = 32 bytes)
	if len(key) != 64 {
		t.Fatalf("Generated key should be 64 hex characters, got %d", len(key))
	}

	// Verify it's valid hex
	_, err = hex.DecodeString(key)
	if err != nil {
		t.Fatalf("Generated key should be valid hex: %v", err)
	}

	// Generate another key and verify they're different
	key2, err := GenerateMasterKey()
	if err != nil {
		t.Fatalf("Failed to generate second master key: %v", err)
	}

	if key == key2 {
		t.Fatal("Generated keys should be different")
	}
}
