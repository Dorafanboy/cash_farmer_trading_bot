-- Rollback Migration 003: Drop token_positions table
DROP INDEX IF EXISTS idx_token_positions_time;
DROP INDEX IF EXISTS idx_token_positions_pnl;
DROP INDEX IF EXISTS idx_token_positions_token;
DROP INDEX IF EXISTS idx_token_positions_active_only;
DROP TABLE IF EXISTS token_positions; 