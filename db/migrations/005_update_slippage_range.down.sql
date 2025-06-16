-- Migration 005 Down: Revert slippage range from 1%-90% back to 0.1%-50%

-- Drop new constraints
ALTER TABLE user_settings DROP CONSTRAINT user_settings_buy_slippage_range;
ALTER TABLE user_settings DROP CONSTRAINT user_settings_sell_slippage_range;

-- Add old constraints back (0.1% to 50%)
ALTER TABLE user_settings ADD CONSTRAINT user_settings_buy_slippage_range CHECK (buy_slippage >= 0.1 AND buy_slippage <= 50.0);
ALTER TABLE user_settings ADD CONSTRAINT user_settings_sell_slippage_range CHECK (sell_slippage >= 0.1 AND sell_slippage <= 50.0);

-- Revert comments
COMMENT ON COLUMN user_settings.buy_slippage IS 'Buy slippage tolerance in percentage (0.1-50.0)';
COMMENT ON COLUMN user_settings.sell_slippage IS 'Sell slippage tolerance in percentage (0.1-50.0)'; 