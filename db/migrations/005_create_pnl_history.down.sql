-- Rollback Migration 005: Drop pnl_history table
DROP INDEX IF EXISTS idx_pnl_history_performance;
DROP INDEX IF EXISTS idx_pnl_history_timeframe;
DROP INDEX IF EXISTS idx_pnl_history_wallet_period;
DROP TABLE IF EXISTS pnl_history; 