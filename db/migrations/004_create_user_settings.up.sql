-- Migration 004: Create user_settings table for trading preferences
CREATE TABLE IF NOT EXISTS user_settings (
    user_id BIGINT PRIMARY KEY,
    
    -- Slippage settings (percentage)
    buy_slippage DECIMAL(5,2) NOT NULL DEFAULT 1.0,
    sell_slippage DECIMAL(5,2) NOT NULL DEFAULT 1.0,
    
    -- Transaction fees (in lamports)
    priority_fee BIGINT NOT NULL DEFAULT 1000,
    jito_tip BIGINT NOT NULL DEFAULT 100000,
    
    -- Trading presets
    default_sol_amount DECIMAL(10,6) NOT NULL DEFAULT 0.1,
    sell_preset_25 DECIMAL(5,2) DEFAULT 25.0,
    sell_preset_50 DECIMAL(5,2) DEFAULT 50.0,
    sell_preset_75 DECIMAL(5,2) DEFAULT 75.0,
    sell_preset_100 DECIMAL(5,2) DEFAULT 100.0,
    
    -- Metadata
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    
    -- Constraints for slippage (0.1% to 50%)
    CONSTRAINT user_settings_buy_slippage_range CHECK (buy_slippage >= 0.1 AND buy_slippage <= 50.0),
    CONSTRAINT user_settings_sell_slippage_range CHECK (sell_slippage >= 0.1 AND sell_slippage <= 50.0),
    
    -- Constraints for fees (non-negative)
    CONSTRAINT user_settings_priority_fee_non_negative CHECK (priority_fee >= 0),
    CONSTRAINT user_settings_jito_tip_non_negative CHECK (jito_tip >= 0),
    
    -- Constraints for trading amounts (positive)
    CONSTRAINT user_settings_default_sol_amount_positive CHECK (default_sol_amount > 0),
    
    -- Constraints for sell presets (0-100%)
    CONSTRAINT user_settings_sell_preset_25_range CHECK (sell_preset_25 IS NULL OR (sell_preset_25 > 0 AND sell_preset_25 <= 100)),
    CONSTRAINT user_settings_sell_preset_50_range CHECK (sell_preset_50 IS NULL OR (sell_preset_50 > 0 AND sell_preset_50 <= 100)),
    CONSTRAINT user_settings_sell_preset_75_range CHECK (sell_preset_75 IS NULL OR (sell_preset_75 > 0 AND sell_preset_75 <= 100)),
    CONSTRAINT user_settings_sell_preset_100_range CHECK (sell_preset_100 IS NULL OR (sell_preset_100 > 0 AND sell_preset_100 <= 100))
);

-- Index for fast user settings lookup (primary key already covers this, but explicit for clarity)
CREATE INDEX idx_user_settings_updated ON user_settings(updated_at DESC);

-- Comments
COMMENT ON TABLE user_settings IS 'Per-user trading preferences and defaults';
COMMENT ON COLUMN user_settings.buy_slippage IS 'Buy slippage tolerance in percentage (0.1-50.0)';
COMMENT ON COLUMN user_settings.sell_slippage IS 'Sell slippage tolerance in percentage (0.1-50.0)';
COMMENT ON COLUMN user_settings.priority_fee IS 'Priority fee in lamports for transactions';
COMMENT ON COLUMN user_settings.jito_tip IS 'Jito bundle tip in lamports';
COMMENT ON COLUMN user_settings.default_sol_amount IS 'Default SOL amount for token purchases'; 