-- Rollback Migration 002: Drop user_balances table
DROP INDEX IF EXISTS idx_user_balances_cache_expired;
DROP INDEX IF EXISTS idx_user_balances_wallet_cache;
DROP TABLE IF EXISTS user_balances; 