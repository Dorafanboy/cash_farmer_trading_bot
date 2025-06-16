-- Rollback Migration 004: Drop user_settings table
DROP INDEX IF EXISTS idx_user_settings_updated;
DROP TABLE IF EXISTS user_settings; 