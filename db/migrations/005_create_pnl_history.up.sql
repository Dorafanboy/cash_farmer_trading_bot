-- Migration 005: Create pnl_history table for tracking profit/loss over time
CREATE TABLE IF NOT EXISTS pnl_history (
    id BIGSERIAL PRIMARY KEY,
    wallet_id BIGINT NOT NULL REFERENCES wallet(id) ON DELETE CASCADE,
    period_type VARCHAR(10) NOT NULL,
    period_start TIMESTAMP WITH TIME ZONE NOT NULL,
    period_end TIMESTAMP WITH TIME ZONE NOT NULL,
    
    -- PnL metrics
    total_pnl_usd DECIMAL(20,2) NOT NULL DEFAULT 0,
    realized_pnl_usd DECIMAL(20,2) NOT NULL DEFAULT 0,
    unrealized_pnl_usd DECIMAL(20,2) NOT NULL DEFAULT 0,
    
    -- Portfolio metrics
    total_portfolio_value_usd DECIMAL(20,2) NOT NULL DEFAULT 0,
    sol_balance DECIMAL(20,9) NOT NULL DEFAULT 0,
    active_positions_count INTEGER NOT NULL DEFAULT 0,
    
    -- Trading metrics
    trades_count INTEGER NOT NULL DEFAULT 0,
    winning_trades INTEGER NOT NULL DEFAULT 0,
    losing_trades INTEGER NOT NULL DEFAULT 0,
    
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    
    -- Constraints
    CONSTRAINT pnl_history_period_type_valid CHECK (period_type IN ('daily', 'weekly', 'monthly')),
    CONSTRAINT pnl_history_period_valid CHECK (period_end > period_start),
    CONSTRAINT pnl_history_portfolio_value_non_negative CHECK (total_portfolio_value_usd >= 0),
    CONSTRAINT pnl_history_sol_balance_non_negative CHECK (sol_balance >= 0),
    CONSTRAINT pnl_history_positions_count_non_negative CHECK (active_positions_count >= 0),
    CONSTRAINT pnl_history_trades_count_non_negative CHECK (trades_count >= 0),
    CONSTRAINT pnl_history_winning_trades_valid CHECK (winning_trades >= 0 AND winning_trades <= trades_count),
    CONSTRAINT pnl_history_losing_trades_valid CHECK (losing_trades >= 0 AND losing_trades <= trades_count),
    CONSTRAINT pnl_history_trade_counts_consistent CHECK (winning_trades + losing_trades <= trades_count),
    
    -- Unique constraint: one record per wallet/period/timeframe
    UNIQUE(wallet_id, period_type, period_start)
);

-- Index for wallet-specific PnL queries
CREATE INDEX idx_pnl_history_wallet_period ON pnl_history(wallet_id, period_type, period_start DESC);

-- Index for time-based analytics
CREATE INDEX idx_pnl_history_timeframe ON pnl_history(period_type, period_start DESC);

-- Index for performance analytics
CREATE INDEX idx_pnl_history_performance ON pnl_history(wallet_id, total_pnl_usd DESC) 
WHERE total_pnl_usd != 0;

-- Comments
COMMENT ON TABLE pnl_history IS 'Historical PnL snapshots for analytics and performance tracking';
COMMENT ON COLUMN pnl_history.period_type IS 'Time period: daily, weekly, monthly';
COMMENT ON COLUMN pnl_history.realized_pnl_usd IS 'PnL from completed trades';
COMMENT ON COLUMN pnl_history.unrealized_pnl_usd IS 'PnL from open positions';
COMMENT ON COLUMN pnl_history.winning_trades IS 'Number of profitable trades in period';
COMMENT ON COLUMN pnl_history.losing_trades IS 'Number of loss-making trades in period'; 