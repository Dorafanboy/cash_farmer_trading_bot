-- Rollback Migration 007: Remove customizable buy presets from user_settings table
-- Removes the 5 buy preset fields and their constraints

-- Drop constraints first
ALTER TABLE user_settings 
DROP CONSTRAINT IF EXISTS user_settings_buy_preset_1_range,
DROP CONSTRAINT IF EXISTS user_settings_buy_preset_2_range,
DROP CONSTRAINT IF EXISTS user_settings_buy_preset_3_range,
DROP CONSTRAINT IF EXISTS user_settings_buy_preset_4_range,
DROP CONSTRAINT IF EXISTS user_settings_buy_preset_5_range;

-- Drop columns
ALTER TABLE user_settings 
DROP COLUMN IF EXISTS buy_preset_1,
DROP COLUMN IF EXISTS buy_preset_2,
DROP COLUMN IF EXISTS buy_preset_3,
DROP COLUMN IF EXISTS buy_preset_4,
DROP COLUMN IF EXISTS buy_preset_5; 