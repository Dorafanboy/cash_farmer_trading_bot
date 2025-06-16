-- name: CreateUserSettings :exec
INSERT INTO user_settings (user_id, buy_slippage, sell_slippage, priority_fee, jito_tip, default_sol_amount, 
                          sell_preset_25, sell_preset_50, sell_preset_75, sell_preset_100,
                          buy_preset_1, buy_preset_2, buy_preset_3, buy_preset_4, buy_preset_5, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, NOW(), NOW());

-- name: GetUserSettings :one
SELECT user_id, buy_slippage, sell_slippage, priority_fee, jito_tip, default_sol_amount,
       sell_preset_25, sell_preset_50, sell_preset_75, sell_preset_100,
       buy_preset_1, buy_preset_2, buy_preset_3, buy_preset_4, buy_preset_5, created_at, updated_at
FROM user_settings
WHERE user_id = $1;

-- name: UpsertUserSettings :exec
INSERT INTO user_settings (user_id, buy_slippage, sell_slippage, priority_fee, jito_tip, default_sol_amount, 
                          sell_preset_25, sell_preset_50, sell_preset_75, sell_preset_100,
                          buy_preset_1, buy_preset_2, buy_preset_3, buy_preset_4, buy_preset_5, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, NOW(), NOW())
ON CONFLICT (user_id)
DO UPDATE SET
    buy_slippage = EXCLUDED.buy_slippage,
    sell_slippage = EXCLUDED.sell_slippage,
    priority_fee = EXCLUDED.priority_fee,
    jito_tip = EXCLUDED.jito_tip,
    default_sol_amount = EXCLUDED.default_sol_amount,
    sell_preset_25 = EXCLUDED.sell_preset_25,
    sell_preset_50 = EXCLUDED.sell_preset_50,
    sell_preset_75 = EXCLUDED.sell_preset_75,
    sell_preset_100 = EXCLUDED.sell_preset_100,
    buy_preset_1 = EXCLUDED.buy_preset_1,
    buy_preset_2 = EXCLUDED.buy_preset_2,
    buy_preset_3 = EXCLUDED.buy_preset_3,
    buy_preset_4 = EXCLUDED.buy_preset_4,
    buy_preset_5 = EXCLUDED.buy_preset_5,
    updated_at = NOW();

-- name: UpdateSlippageSettings :exec
UPDATE user_settings
SET buy_slippage = $2, sell_slippage = $3, updated_at = NOW()
WHERE user_id = $1;

-- name: UpdateFeeSettings :exec
UPDATE user_settings
SET priority_fee = $2, jito_tip = $3, updated_at = NOW()
WHERE user_id = $1;

-- name: UpdateDefaultSolAmount :exec
UPDATE user_settings
SET default_sol_amount = $2, updated_at = NOW()
WHERE user_id = $1;

-- name: UpdateSellPresets :exec
UPDATE user_settings
SET sell_preset_25 = $2, sell_preset_50 = $3, sell_preset_75 = $4, sell_preset_100 = $5, updated_at = NOW()
WHERE user_id = $1;

-- name: UpdateBuyPresets :exec
UPDATE user_settings
SET buy_preset_1 = $2, buy_preset_2 = $3, buy_preset_3 = $4, buy_preset_4 = $5, buy_preset_5 = $6, updated_at = NOW()
WHERE user_id = $1;

-- name: UpdateSingleBuyPreset1 :exec
UPDATE user_settings
SET buy_preset_1 = $2, updated_at = NOW()
WHERE user_id = $1;

-- name: UpdateSingleBuyPreset2 :exec
UPDATE user_settings
SET buy_preset_2 = $2, updated_at = NOW()
WHERE user_id = $1;

-- name: UpdateSingleBuyPreset3 :exec
UPDATE user_settings
SET buy_preset_3 = $2, updated_at = NOW()
WHERE user_id = $1;

-- name: UpdateSingleBuyPreset4 :exec
UPDATE user_settings
SET buy_preset_4 = $2, updated_at = NOW()
WHERE user_id = $1;

-- name: UpdateSingleBuyPreset5 :exec
UPDATE user_settings
SET buy_preset_5 = $2, updated_at = NOW()
WHERE user_id = $1;

-- name: UpdateSingleSellPreset1 :exec
UPDATE user_settings
SET sell_preset_25 = $2, updated_at = NOW()
WHERE user_id = $1;

-- name: UpdateSingleSellPreset2 :exec
UPDATE user_settings
SET sell_preset_50 = $2, updated_at = NOW()
WHERE user_id = $1;

-- name: UpdateSingleSellPreset3 :exec
UPDATE user_settings
SET sell_preset_75 = $2, updated_at = NOW()
WHERE user_id = $1;

-- name: UpdateSingleSellPreset4 :exec
UPDATE user_settings
SET sell_preset_100 = $2, updated_at = NOW()
WHERE user_id = $1;

-- name: UpdatePriorityFee :exec
UPDATE user_settings
SET priority_fee = $2, updated_at = NOW()
WHERE user_id = $1;

-- name: UpdateJitoTip :exec
UPDATE user_settings
SET jito_tip = $2, updated_at = NOW()
WHERE user_id = $1;

-- name: GetUserSlippageSettings :one
SELECT user_id, buy_slippage, sell_slippage
FROM user_settings
WHERE user_id = $1;

-- name: GetUserFeeSettings :one
SELECT user_id, priority_fee, jito_tip
FROM user_settings
WHERE user_id = $1;

-- name: GetUserTradingPresets :one
SELECT user_id, default_sol_amount, sell_preset_25, sell_preset_50, sell_preset_75, sell_preset_100
FROM user_settings
WHERE user_id = $1;

-- name: GetUserBuyPresets :one
SELECT user_id, buy_preset_1, buy_preset_2, buy_preset_3, buy_preset_4, buy_preset_5
FROM user_settings
WHERE user_id = $1;

-- name: DeleteUserSettings :exec
DELETE FROM user_settings
WHERE user_id = $1;

-- name: GetSettingsWithDefaults :one
SELECT 
    COALESCE(us.buy_slippage, 15.0) as buy_slippage,
    COALESCE(us.sell_slippage, 1.0) as sell_slippage,
    COALESCE(us.priority_fee, 1000) as priority_fee,
    COALESCE(us.jito_tip, 100000) as jito_tip,
    COALESCE(us.default_sol_amount, 0.1) as default_sol_amount,
    COALESCE(us.sell_preset_25, 25.0) as sell_preset_25,
    COALESCE(us.sell_preset_50, 50.0) as sell_preset_50,
    COALESCE(us.sell_preset_75, 75.0) as sell_preset_75,
    COALESCE(us.sell_preset_100, 100.0) as sell_preset_100,
    COALESCE(us.buy_preset_1, 0.01) as buy_preset_1,
    COALESCE(us.buy_preset_2, 0.05) as buy_preset_2,
    COALESCE(us.buy_preset_3, 0.1) as buy_preset_3,
    COALESCE(us.buy_preset_4, 0.5) as buy_preset_4,
    COALESCE(us.buy_preset_5, 1.0) as buy_preset_5
FROM (SELECT $1::bigint as user_id) AS u
LEFT JOIN user_settings us ON us.user_id = u.user_id; 