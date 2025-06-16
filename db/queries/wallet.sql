-- name: SaveWallet :exec
INSERT INTO wallet (user_id, name, public_key, private_key, mnemonic, created_at, is_default)
VALUES ($1, $2, $3, $4, $5, $6, $7);

-- name: FindWalletByID :one
SELECT id, user_id, name, public_key, private_key, mnemonic, created_at, is_default
FROM wallet
WHERE id = $1;

-- name: FindWalletsByUserID :many
SELECT id, user_id, name, public_key, private_key, mnemonic, created_at, is_default
FROM wallet
WHERE user_id = $1
ORDER BY created_at DESC;

-- name: FindDefaultWalletByUserID :one
SELECT id, user_id, name, public_key, private_key, mnemonic, created_at, is_default
FROM wallet
WHERE user_id = $1 AND is_default = true;

-- name: FindWalletByName :one
SELECT id, user_id, name, public_key, private_key, mnemonic, created_at, is_default
FROM wallet
WHERE user_id = $1 AND name = $2;

-- name: UpdateWallet :exec
UPDATE wallet
SET name = $2, public_key = $3, private_key = $4, mnemonic = $5, is_default = $6
WHERE id = $1;

-- name: DeleteWallet :exec
DELETE FROM wallet
WHERE id = $1 AND user_id = $2;

-- name: SetWalletAsDefault :exec
UPDATE wallet
SET is_default = CASE
    WHEN id = $2 THEN true
    ELSE false
END
WHERE user_id = $1;

-- name: ClearDefaultWallets :exec
UPDATE wallet
SET is_default = false
WHERE user_id = $1;

-- name: SetWalletDefault :exec
UPDATE wallet
SET is_default = true
WHERE id = $1 AND user_id = $2;

-- name: CountUserWallets :one
SELECT COUNT(*)
FROM wallet
WHERE user_id = $1;

-- NEW ENCRYPTION-RELATED QUERIES

-- name: SaveEncryptedWallet :exec
INSERT INTO wallet (user_id, name, public_key, encrypted_private_key, private_key_nonce, 
                   encrypted_mnemonic, mnemonic_nonce, encryption_version, encrypted_at, 
                   created_at, is_default)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11);

-- name: FindWalletWithEncryption :one
SELECT id, user_id, name, public_key, encrypted_private_key, private_key_nonce,
       encrypted_mnemonic, mnemonic_nonce, encryption_version, encrypted_at,
       created_at, is_default
FROM wallet
WHERE id = $1;

-- name: FindWalletsWithEncryptionByUserID :many
SELECT id, user_id, name, public_key, encrypted_private_key, private_key_nonce,
       encrypted_mnemonic, mnemonic_nonce, encryption_version, encrypted_at,
       created_at, is_default
FROM wallet
WHERE user_id = $1
ORDER BY created_at DESC;

-- name: UpdateWalletEncryption :exec
UPDATE wallet
SET encrypted_private_key = $2, private_key_nonce = $3,
    encrypted_mnemonic = $4, mnemonic_nonce = $5,
    encryption_version = $6, encrypted_at = $7
WHERE id = $1;

-- name: MigrateWalletToEncryption :exec
UPDATE wallet
SET encrypted_private_key = $2, private_key_nonce = $3,
    encrypted_mnemonic = $4, mnemonic_nonce = $5,
    encryption_version = $6, encrypted_at = $7,
    private_key = '', mnemonic = ''
WHERE id = $1;

-- name: FindUnencryptedWallets :many
SELECT id, user_id, name, public_key, private_key, mnemonic, created_at, is_default
FROM wallet
WHERE encrypted_private_key IS NULL OR private_key_nonce IS NULL
ORDER BY created_at ASC;

-- name: IsWalletEncrypted :one
SELECT 
    (encrypted_private_key IS NOT NULL AND private_key_nonce IS NOT NULL) as is_encrypted,
    encryption_version
FROM wallet
WHERE id = $1;

-- name: FindDefaultWalletWithEncryption :one
SELECT id, user_id, name, public_key, encrypted_private_key, private_key_nonce,
       encrypted_mnemonic, mnemonic_nonce, encryption_version, encrypted_at,
       created_at, is_default
FROM wallet
WHERE user_id = $1 AND is_default = true; 