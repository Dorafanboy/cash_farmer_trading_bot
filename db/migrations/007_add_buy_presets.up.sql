-- Migration 007: Add customizable buy presets to user_settings table
-- Adds 5 customizable buy preset fields for user-defined trading amounts

-- Add buy preset columns with default values
ALTER TABLE user_settings 
ADD COLUMN buy_preset_1 DECIMAL(10,6) DEFAULT 0.01,
ADD COLUMN buy_preset_2 DECIMAL(10,6) DEFAULT 0.05,
ADD COLUMN buy_preset_3 DECIMAL(10,6) DEFAULT 0.1,
ADD COLUMN buy_preset_4 DECIMAL(10,6) DEFAULT 0.5,
ADD COLUMN buy_preset_5 DECIMAL(10,6) DEFAULT 1.0;

-- Add constraints for buy presets (0.001 - 100 SOL range)
ALTER TABLE user_settings 
ADD CONSTRAINT user_settings_buy_preset_1_range CHECK (buy_preset_1 IS NULL OR (buy_preset_1 >= 0.001 AND buy_preset_1 <= 100)),
ADD CONSTRAINT user_settings_buy_preset_2_range CHECK (buy_preset_2 IS NULL OR (buy_preset_2 >= 0.001 AND buy_preset_2 <= 100)),
ADD CONSTRAINT user_settings_buy_preset_3_range CHECK (buy_preset_3 IS NULL OR (buy_preset_3 >= 0.001 AND buy_preset_3 <= 100)),
ADD CONSTRAINT user_settings_buy_preset_4_range CHECK (buy_preset_4 IS NULL OR (buy_preset_4 >= 0.001 AND buy_preset_4 <= 100)),
ADD CONSTRAINT user_settings_buy_preset_5_range CHECK (buy_preset_5 IS NULL OR (buy_preset_5 >= 0.001 AND buy_preset_5 <= 100));

-- Comments for documentation
COMMENT ON COLUMN user_settings.buy_preset_1 IS 'Customizable buy preset 1 in SOL (0.001-100)';
COMMENT ON COLUMN user_settings.buy_preset_2 IS 'Customizable buy preset 2 in SOL (0.001-100)';
COMMENT ON COLUMN user_settings.buy_preset_3 IS 'Customizable buy preset 3 in SOL (0.001-100)';
COMMENT ON COLUMN user_settings.buy_preset_4 IS 'Customizable buy preset 4 in SOL (0.001-100)';
COMMENT ON COLUMN user_settings.buy_preset_5 IS 'Customizable buy preset 5 in SOL (0.001-100)';

-- Update existing users with default preset values
UPDATE user_settings SET 
    buy_preset_1 = 0.01,
    buy_preset_2 = 0.05,
    buy_preset_3 = 0.1,
    buy_preset_4 = 0.5,
    buy_preset_5 = 1.0,
    updated_at = NOW(); 