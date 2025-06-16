-- Migration 002: Create user_balances table for SOL balance caching
CREATE TABLE IF NOT EXISTS user_balances (
    id BIGSERIAL PRIMARY KEY,
    wallet_id BIGINT NOT NULL REFERENCES wallet(id) ON DELETE CASCADE,
    sol_balance DECIMAL(20,9) NOT NULL DEFAULT 0,
    last_updated TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    cache_valid_until TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW() + INTERVAL '10 minutes',
    
    -- Constraints
    CONSTRAINT user_balances_sol_balance_non_negative CHECK (sol_balance >= 0),
    
    -- Unique constraint: one balance record per wallet
    UNIQUE(wallet_id)
);

-- Index for cache validation queries
CREATE INDEX idx_user_balances_wallet_cache ON user_balances(wallet_id, cache_valid_until);

-- Index for cache cleanup queries
CREATE INDEX idx_user_balances_cache_expired ON user_balances(cache_valid_until);

-- Comment
COMMENT ON TABLE user_balances IS 'Cached SOL balances for user wallets with TTL mechanism';
COMMENT ON COLUMN user_balances.cache_valid_until IS 'TTL for balance cache, typically 5-10 minutes'; 