-- Migration 003: Create token_positions table for portfolio tracking
CREATE TABLE IF NOT EXISTS token_positions (
    id BIGSERIAL PRIMARY KEY,
    wallet_id BIGINT NOT NULL REFERENCES wallet(id) ON DELETE CASCADE,
    token_address VARCHAR(44) NOT NULL,
    token_symbol VARCHAR(20),
    amount DECIMAL(20,9) NOT NULL,
    entry_price DECIMAL(20,9) NOT NULL,
    current_price DECIMAL(20,9),
    pnl_usd DECIMAL(20,2),
    opened_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    
    -- Constraints
    CONSTRAINT token_positions_amount_positive CHECK (amount > 0),
    CONSTRAINT token_positions_entry_price_positive CHECK (entry_price > 0),
    CONSTRAINT token_positions_current_price_positive CHECK (current_price IS NULL OR current_price > 0),
    CONSTRAINT token_positions_address_length CHECK (LENGTH(token_address) >= 32 AND LENGTH(token_address) <= 44),
    
    -- Unique constraint: one active position per wallet-token pair
    UNIQUE(wallet_id, token_address, is_active) DEFERRABLE INITIALLY DEFERRED
);

-- Partial index for active positions only (most queries)
CREATE INDEX idx_token_positions_active_only ON token_positions(wallet_id, updated_at) 
WHERE is_active = TRUE;

-- Index for token-wide queries
CREATE INDEX idx_token_positions_token ON token_positions(token_address, is_active);

-- Index for PnL calculations
CREATE INDEX idx_token_positions_pnl ON token_positions(wallet_id, pnl_usd DESC) 
WHERE is_active = TRUE AND pnl_usd IS NOT NULL;

-- Index for time-based queries
CREATE INDEX idx_token_positions_time ON token_positions(wallet_id, opened_at DESC);

-- Comments
COMMENT ON TABLE token_positions IS 'Active and historical token positions for portfolio tracking';
COMMENT ON COLUMN token_positions.is_active IS 'FALSE when position is closed/sold';
COMMENT ON COLUMN token_positions.pnl_usd IS 'Calculated PnL in USD, updated periodically'; 