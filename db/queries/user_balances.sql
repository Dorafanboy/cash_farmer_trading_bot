-- name: UpsertUserBalance :exec
INSERT INTO user_balances (wallet_id, sol_balance, last_updated, cache_valid_until)
VALUES ($1, $2, NOW(), NOW() + INTERVAL '10 minutes')
ON CONFLICT (wallet_id) 
DO UPDATE SET 
    sol_balance = EXCLUDED.sol_balance,
    last_updated = NOW(),
    cache_valid_until = NOW() + INTERVAL '10 minutes';

-- name: GetUserBalance :one
SELECT id, wallet_id, sol_balance, last_updated, cache_valid_until
FROM user_balances
WHERE wallet_id = $1;

-- name: GetValidUserBalance :one
SELECT id, wallet_id, sol_balance, last_updated, cache_valid_until
FROM user_balances
WHERE wallet_id = $1 AND cache_valid_until > NOW();

-- name: GetAllUserBalances :many
SELECT ub.id, ub.wallet_id, ub.sol_balance, ub.last_updated, ub.cache_valid_until,
       w.user_id, w.name as wallet_name
FROM user_balances ub
JOIN wallet w ON ub.wallet_id = w.id
WHERE w.user_id = $1
ORDER BY ub.last_updated DESC;

-- name: GetExpiredBalances :many
SELECT id, wallet_id, sol_balance, last_updated, cache_valid_until
FROM user_balances
WHERE cache_valid_until <= NOW()
ORDER BY cache_valid_until ASC
LIMIT $1;

-- name: DeleteExpiredBalances :exec
DELETE FROM user_balances
WHERE cache_valid_until <= NOW() - INTERVAL '1 hour';

-- name: UpdateBalanceAmount :exec
UPDATE user_balances
SET sol_balance = $2, last_updated = NOW()
WHERE wallet_id = $1;

-- name: ExtendBalanceCache :exec
UPDATE user_balances
SET cache_valid_until = NOW() + INTERVAL '10 minutes'
WHERE wallet_id = $1;

-- name: DeleteUserBalance :exec
DELETE FROM user_balances
WHERE wallet_id = $1;

-- name: GetBalanceStats :one
SELECT 
    COUNT(*) as total_cached_balances,
    COUNT(CASE WHEN cache_valid_until > NOW() THEN 1 END) as valid_cached_balances,
    SUM(sol_balance) as total_sol_across_all_users
FROM user_balances;