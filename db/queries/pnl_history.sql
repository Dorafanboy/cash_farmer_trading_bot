-- name: CreatePnLRecord :exec
INSERT INTO pnl_history (wallet_id, period_type, period_start, period_end, total_pnl_usd, realized_pnl_usd, 
                        unrealized_pnl_usd, total_portfolio_value_usd, sol_balance, active_positions_count,
                        trades_count, winning_trades, losing_trades, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, NOW());

-- name: UpsertPnLRecord :exec
INSERT INTO pnl_history (wallet_id, period_type, period_start, period_end, total_pnl_usd, realized_pnl_usd, 
                        unrealized_pnl_usd, total_portfolio_value_usd, sol_balance, active_positions_count,
                        trades_count, winning_trades, losing_trades, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, NOW())
ON CONFLICT (wallet_id, period_type, period_start)
DO UPDATE SET
    period_end = EXCLUDED.period_end,
    total_pnl_usd = EXCLUDED.total_pnl_usd,
    realized_pnl_usd = EXCLUDED.realized_pnl_usd,
    unrealized_pnl_usd = EXCLUDED.unrealized_pnl_usd,
    total_portfolio_value_usd = EXCLUDED.total_portfolio_value_usd,
    sol_balance = EXCLUDED.sol_balance,
    active_positions_count = EXCLUDED.active_positions_count,
    trades_count = EXCLUDED.trades_count,
    winning_trades = EXCLUDED.winning_trades,
    losing_trades = EXCLUDED.losing_trades,
    created_at = NOW();

-- name: GetPnLHistory :many
SELECT id, wallet_id, period_type, period_start, period_end, total_pnl_usd, realized_pnl_usd, 
       unrealized_pnl_usd, total_portfolio_value_usd, sol_balance, active_positions_count,
       trades_count, winning_trades, losing_trades, created_at
FROM pnl_history
WHERE wallet_id = $1 AND period_type = $2
ORDER BY period_start DESC
LIMIT $3;

-- name: GetPnLByPeriod :one
SELECT id, wallet_id, period_type, period_start, period_end, total_pnl_usd, realized_pnl_usd, 
       unrealized_pnl_usd, total_portfolio_value_usd, sol_balance, active_positions_count,
       trades_count, winning_trades, losing_trades, created_at
FROM pnl_history
WHERE wallet_id = $1 AND period_type = $2 AND period_start = $3;

-- name: GetDailyPnL :many
SELECT id, wallet_id, period_type, period_start, period_end, total_pnl_usd, realized_pnl_usd, 
       unrealized_pnl_usd, total_portfolio_value_usd, sol_balance, active_positions_count,
       trades_count, winning_trades, losing_trades, created_at
FROM pnl_history
WHERE wallet_id = $1 AND period_type = 'daily'
ORDER BY period_start DESC
LIMIT $2;

-- name: GetWeeklyPnL :many
SELECT id, wallet_id, period_type, period_start, period_end, total_pnl_usd, realized_pnl_usd, 
       unrealized_pnl_usd, total_portfolio_value_usd, sol_balance, active_positions_count,
       trades_count, winning_trades, losing_trades, created_at
FROM pnl_history
WHERE wallet_id = $1 AND period_type = 'weekly'
ORDER BY period_start DESC
LIMIT $2;

-- name: GetMonthlyPnL :many
SELECT id, wallet_id, period_type, period_start, period_end, total_pnl_usd, realized_pnl_usd, 
       unrealized_pnl_usd, total_portfolio_value_usd, sol_balance, active_positions_count,
       trades_count, winning_trades, losing_trades, created_at
FROM pnl_history
WHERE wallet_id = $1 AND period_type = 'monthly'
ORDER BY period_start DESC
LIMIT $2;

-- name: GetPnLSummaryStats :one
SELECT 
    COUNT(*) as total_periods,
    SUM(total_pnl_usd) as cumulative_pnl_usd,
    AVG(total_pnl_usd) as avg_pnl_per_period,
    SUM(trades_count) as total_trades,
    SUM(winning_trades) as total_winning_trades,
    SUM(losing_trades) as total_losing_trades,
    MAX(total_pnl_usd) as best_period_pnl,
    MIN(total_pnl_usd) as worst_period_pnl,
    AVG(total_portfolio_value_usd) as avg_portfolio_value
FROM pnl_history
WHERE wallet_id = $1 AND period_type = $2;

-- name: GetRecentPerformance :many
SELECT id, wallet_id, period_type, period_start, period_end, total_pnl_usd, realized_pnl_usd, 
       unrealized_pnl_usd, total_portfolio_value_usd, sol_balance, active_positions_count,
       trades_count, winning_trades, losing_trades, created_at
FROM pnl_history
WHERE wallet_id = $1 AND period_start >= $2
ORDER BY period_start DESC;

-- name: GetTopPerformingPeriods :many
SELECT id, wallet_id, period_type, period_start, period_end, total_pnl_usd, realized_pnl_usd, 
       unrealized_pnl_usd, total_portfolio_value_usd, sol_balance, active_positions_count,
       trades_count, winning_trades, losing_trades, created_at
FROM pnl_history
WHERE wallet_id = $1 AND period_type = $2 AND total_pnl_usd > 0
ORDER BY total_pnl_usd DESC
LIMIT $3;

-- name: GetWorstPerformingPeriods :many
SELECT id, wallet_id, period_type, period_start, period_end, total_pnl_usd, realized_pnl_usd, 
       unrealized_pnl_usd, total_portfolio_value_usd, sol_balance, active_positions_count,
       trades_count, winning_trades, losing_trades, created_at
FROM pnl_history
WHERE wallet_id = $1 AND period_type = $2 AND total_pnl_usd < 0
ORDER BY total_pnl_usd ASC
LIMIT $3;

-- name: DeleteOldPnLRecords :exec
DELETE FROM pnl_history
WHERE wallet_id = $1 AND period_start < $2;

-- name: GetAllWalletsPnLForPeriod :many
SELECT ph.id, ph.wallet_id, ph.period_type, ph.period_start, ph.period_end, ph.total_pnl_usd, ph.realized_pnl_usd, 
       ph.unrealized_pnl_usd, ph.total_portfolio_value_usd, ph.sol_balance, ph.active_positions_count,
       ph.trades_count, ph.winning_trades, ph.losing_trades, ph.created_at,
       w.user_id, w.name as wallet_name
FROM pnl_history ph
JOIN wallet w ON ph.wallet_id = w.id
WHERE ph.period_type = $1 AND ph.period_start = $2
ORDER BY ph.total_pnl_usd DESC; 