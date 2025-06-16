-- Rollback Migration 006: Remove encryption columns from wallet table
ALTER TABLE wallet 
DROP CONSTRAINT IF EXISTS wallet_encryption_version_valid;

ALTER TABLE wallet 
DROP COLUMN IF EXISTS encrypted_at,
DROP COLUMN IF EXISTS encryption_version,
DROP COLUMN IF EXISTS mnemonic_nonce,
DROP COLUMN IF EXISTS encrypted_mnemonic,
DROP COLUMN IF EXISTS private_key_nonce,
DROP COLUMN IF EXISTS encrypted_private_key; 