-- Migration 006: Extend wallet table with encryption columns
-- Add new encrypted columns for secure storage

-- Add encrypted private key with nonce
ALTER TABLE wallet 
ADD COLUMN encrypted_private_key BYTEA,
ADD COLUMN private_key_nonce BYTEA;

-- Add encrypted mnemonic with nonce
ALTER TABLE wallet 
ADD COLUMN encrypted_mnemonic BYTEA,
ADD COLUMN mnemonic_nonce BYTEA;

-- Add encryption metadata
ALTER TABLE wallet 
ADD COLUMN encryption_version INTEGER NOT NULL DEFAULT 1,
ADD COLUMN encrypted_at TIMESTAMP WITH TIME ZONE;

-- Constraints for encryption fields
ALTER TABLE wallet 
ADD CONSTRAINT wallet_encryption_version_valid CHECK (encryption_version > 0);

-- Comment on new columns
COMMENT ON COLUMN wallet.encrypted_private_key IS 'AES-256-GCM encrypted private key';
COMMENT ON COLUMN wallet.private_key_nonce IS 'Nonce for private key encryption';
COMMENT ON COLUMN wallet.encrypted_mnemonic IS 'AES-256-GCM encrypted mnemonic phrase';
COMMENT ON COLUMN wallet.mnemonic_nonce IS 'Nonce for mnemonic encryption';
COMMENT ON COLUMN wallet.encryption_version IS 'Version of encryption scheme used';
COMMENT ON COLUMN wallet.encrypted_at IS 'Timestamp when encryption was applied';

-- Note: Original private_key and mnemonic columns will be removed in a future migration
-- after data migration is complete to ensure no data loss 