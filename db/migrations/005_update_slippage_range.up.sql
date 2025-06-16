-- Migration 005: Update slippage range from 0.1%-50% to 1%-90%

-- Drop old constraints
ALTER TABLE user_settings DROP CONSTRAINT user_settings_buy_slippage_range;
ALTER TABLE user_settings DROP CONSTRAINT user_settings_sell_slippage_range;

-- Add new constraints with updated range (1% to 90%)
ALTER TABLE user_settings ADD CONSTRAINT user_settings_buy_slippage_range CHECK (buy_slippage >= 1.0 AND buy_slippage <= 90.0);
ALTER TABLE user_settings ADD CONSTRAINT user_settings_sell_slippage_range CHECK (sell_slippage >= 1.0 AND sell_slippage <= 90.0);

-- Update comments
COMMENT ON COLUMN user_settings.buy_slippage IS 'Buy slippage tolerance in percentage (1.0-90.0)';
COMMENT ON COLUMN user_settings.sell_slippage IS 'Sell slippage tolerance in percentage (1.0-90.0)'; 