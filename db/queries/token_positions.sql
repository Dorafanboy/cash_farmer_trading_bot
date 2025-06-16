-- name: CreateTokenPosition :one
INSERT INTO token_positions (wallet_id, token_address, token_symbol, amount, entry_price, opened_at)
VALUES ($1, $2, $3, $4, $5, NOW())
RETURNING id, wallet_id, token_address, token_symbol, amount, entry_price, current_price, pnl_usd, opened_at, updated_at, is_active;

-- name: GetTokenPosition :one
SELECT id, wallet_id, token_address, token_symbol, amount, entry_price, current_price, pnl_usd, opened_at, updated_at, is_active
FROM token_positions
WHERE id = $1;

-- name: GetActiveTokenPosition :one
SELECT id, wallet_id, token_address, token_symbol, amount, entry_price, current_price, pnl_usd, opened_at, updated_at, is_active
FROM token_positions
WHERE wallet_id = $1 AND token_address = $2 AND is_active = true;

-- name: GetActivePositionsByWallet :many
SELECT id, wallet_id, token_address, token_symbol, amount, entry_price, current_price, pnl_usd, opened_at, updated_at, is_active
FROM token_positions
WHERE wallet_id = $1 AND is_active = true
ORDER BY updated_at DESC;

-- name: GetAllPositionsByWallet :many
SELECT id, wallet_id, token_address, token_symbol, amount, entry_price, current_price, pnl_usd, opened_at, updated_at, is_active
FROM token_positions
WHERE wallet_id = $1
ORDER BY updated_at DESC;

-- name: GetPositionsByToken :many
SELECT tp.id, tp.wallet_id, tp.token_address, tp.token_symbol, tp.amount, tp.entry_price, tp.current_price, tp.pnl_usd, tp.opened_at, tp.updated_at, tp.is_active,
       w.user_id, w.name as wallet_name
FROM token_positions tp
JOIN wallet w ON tp.wallet_id = w.id
WHERE tp.token_address = $1 AND tp.is_active = $2
ORDER BY tp.updated_at DESC;

-- name: UpdateTokenPositionPrice :exec
UPDATE token_positions
SET current_price = $2, updated_at = NOW()
WHERE id = $1;

-- name: UpdateTokenPositionPnL :exec
UPDATE token_positions
SET current_price = $2, pnl_usd = $3, updated_at = NOW()
WHERE id = $1;

-- name: UpdateTokenPositionAmount :exec
UPDATE token_positions
SET amount = $2, updated_at = NOW()
WHERE id = $1;

-- name: CloseTokenPosition :exec
UPDATE token_positions
SET is_active = false, updated_at = NOW()
WHERE id = $1;

-- name: GetTopPnLPositions :many
SELECT id, wallet_id, token_address, token_symbol, amount, entry_price, current_price, pnl_usd, opened_at, updated_at, is_active
FROM token_positions
WHERE wallet_id = $1 AND is_active = true AND pnl_usd IS NOT NULL
ORDER BY pnl_usd DESC
LIMIT $2;

-- name: GetPositionsPnLSummary :one
SELECT 
    COUNT(*) as total_active_positions,
    COALESCE(SUM(pnl_usd), 0) as total_pnl_usd,
    COALESCE(AVG(pnl_usd), 0) as avg_pnl_usd,
    COALESCE(SUM(CASE WHEN pnl_usd > 0 THEN pnl_usd ELSE 0 END), 0) as total_profit_usd,
    COALESCE(SUM(CASE WHEN pnl_usd < 0 THEN pnl_usd ELSE 0 END), 0) as total_loss_usd,
    COUNT(CASE WHEN pnl_usd > 0 THEN 1 END) as winning_positions,
    COUNT(CASE WHEN pnl_usd < 0 THEN 1 END) as losing_positions
FROM token_positions
WHERE wallet_id = $1 AND is_active = true;

-- name: GetPositionsForPriceUpdate :many
SELECT id, wallet_id, token_address, current_price
FROM token_positions
WHERE is_active = true AND updated_at < NOW() - INTERVAL '5 minutes'
ORDER BY updated_at ASC
LIMIT $1;

-- name: BulkUpdatePositionPrices :exec
UPDATE token_positions
SET current_price = 
    CASE token_address
        WHEN $1 THEN $2
        WHEN $3 THEN $4
        WHEN $5 THEN $6
        ELSE current_price
    END,
    updated_at = NOW()
WHERE token_address IN ($1, $3, $5) AND is_active = true;

-- name: DeletePosition :exec
DELETE FROM token_positions
WHERE id = $1 AND wallet_id = $2; 