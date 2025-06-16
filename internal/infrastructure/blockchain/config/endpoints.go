package config

import (
	"fmt"
	"time"
)

// BlockchainConfig содержит всю конфигурацию для blockchain services
type BlockchainConfig struct {
	// === JITO CONFIGURATION ===
	JitoEndpoints  []string `json:"jito_endpoints"`
	DefaultJitoTip uint64   `json:"default_jito_tip"`
	MaxJitoTip     uint64   `json:"max_jito_tip"`

	// === SOLANA CONFIGURATION ===
	SolanaRPCURLs    []string      `json:"solana_rpc_urls"`
	BlockhashRefresh time.Duration `json:"blockhash_refresh"`

	// === JUPITER CONFIGURATION ===
	JupiterBaseURL  string `json:"jupiter_base_url"`
	DefaultSlippage int    `json:"default_slippage"`

	// === RETRY CONFIGURATION ===
	MaxRetries     int           `json:"max_retries"`
	RetryDelay     time.Duration `json:"retry_delay"`
	RequestTimeout time.Duration `json:"request_timeout"`
}

// DefaultBlockchainConfig возвращает конфигурацию по умолчанию из main.go
func DefaultBlockchainConfig() *BlockchainConfig {
	return &BlockchainConfig{
		// Jito endpoints из main.go строки 26-36
		JitoEndpoints: []string{
			"https://mainnet.block-engine.jito.wtf/api/v1",
			"https://amsterdam.mainnet.block-engine.jito.wtf/api/v1",
			"https://frankfurt.mainnet.block-engine.jito.wtf/api/v1",
			"https://london.mainnet.block-engine.jito.wtf/api/v1",
			"https://singapore.mainnet.block-engine.jito.wtf",
			"https://ny.mainnet.block-engine.jito.wtf/api/v1",
			"https://slc.mainnet.block-engine.jito.wtf/api/v1",
			"https://tokyo.mainnet.block-engine.jito.wtf/api/v1",
		},

		// Solana RPC URLs из main.go строки 15-19
		SolanaRPCURLs: []string{
			"https://mainnet.helius-rpc.com/?api-key=97f4303e-3cbd-4417-8c0f-bfdcf31e3cec",
			//"https://rpc.hellomoon.io",
			//"https://api.mainnet-beta.solana.com",
			//"https://solana-api.projectserum.com",
		},

		// Jupiter configuration
		JupiterBaseURL:  "https://quote-api.jup.ag/v6",
		DefaultSlippage: 1500, // 1.5%

		// Jito tips configuration
		DefaultJitoTip: 10000,  // 0.00001 SOL
		MaxJitoTip:     500000, // 0.0005 SOL

		// Retry configuration из main.go
		MaxRetries:     3,
		RetryDelay:     2 * time.Second,
		RequestTimeout: 30 * time.Second,

		// Blockhash refresh из main.go строка 172 - увеличили интервал
		BlockhashRefresh: 2 * time.Second, // Изменили с 400ms на 2 секунды
	}
}

// GetJitoEndpoints возвращает список Jito endpoints
func (c *BlockchainConfig) GetJitoEndpoints() []string {
	return c.JitoEndpoints
}

// GetSolanaRPCURLs возвращает список Solana RPC URLs
func (c *BlockchainConfig) GetSolanaRPCURLs() []string {
	return c.SolanaRPCURLs
}

// Validate проверяет корректность конфигурации
func (c *BlockchainConfig) Validate() error {
	if len(c.JitoEndpoints) == 0 {
		return fmt.Errorf("jito endpoints не могут быть пустыми")
	}

	if len(c.SolanaRPCURLs) == 0 {
		return fmt.Errorf("solana RPC URLs не могут быть пустыми")
	}

	if c.JupiterBaseURL == "" {
		return fmt.Errorf("jupiter base URL не может быть пустым")
	}

	return nil
}
