CREATE TABLE IF NOT EXISTS wallet (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL,
    name VARCHAR(50) NOT NULL,
    public_key VARCHAR(44) NOT NULL,
    private_key TEXT NOT NULL,
    mnemonic TEXT DEFAULT '',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    is_default BOOLEAN NOT NULL DEFAULT FALSE,
    
    CONSTRAINT wallet_name_length CHECK (LENGTH(TRIM(name)) >= 2),
    CONSTRAINT wallet_public_key_length CHECK (LENGTH(public_key) >= 32 AND LENGTH(public_key) <= 44),
    CONSTRAINT wallet_private_key_not_empty CHECK (LENGTH(TRIM(private_key)) > 0)
);

CREATE INDEX idx_wallet_user_id ON wallet(user_id);
CREATE INDEX idx_wallet_user_id_name ON wallet(user_id, name);
CREATE INDEX idx_wallet_user_id_default ON wallet(user_id, is_default);

CREATE UNIQUE INDEX idx_wallet_user_default ON wallet(user_id) WHERE is_default = true; 