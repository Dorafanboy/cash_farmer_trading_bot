package config

type DefaultValues struct {
	Database    DatabaseDefaults
	Redis       RedisDefaults
	Solana      SolanaDefaults
	Jupiter     JupiterDefaults
	Jito        JitoDefaults
	DexScreener DexScreenerDefaults
}

type DatabaseDefaults struct {
	Host     string
	Port     string
	User     string
	Database string
	SSLMode  string
}

type RedisDefaults struct {
	URL string
}

type SolanaDefaults struct {
	PublicRPCURL string
}

type JupiterDefaults struct {
	APIURL string
}

type JitoDefaults struct {
	RPCURL string
}

type DexScreenerDefaults struct {
	APIURL string
}

func GetDefaults() DefaultValues {
	return DefaultValues{
		Database: DatabaseDefaults{
			Host:     "localhost",
			Port:     "5432",
			User:     "postgres",
			Database: "cash_farmer",
			SSLMode:  "disable",
		},
		Redis: RedisDefaults{
			URL: "redis://localhost:6379",
		},
		Solana: SolanaDefaults{
			PublicRPCURL: "https://api.mainnet-beta.solana.com",
		},
		Jupiter: JupiterDefaults{
			APIURL: "https://quote-api.jup.ag/v6",
		},
		Jito: JitoDefaults{
			RPCURL: "https://amsterdam.mainnet.block-engine.jito.wtf/api/v1",
		},
		DexScreener: DexScreenerDefaults{
			APIURL: "https://api.dexscreener.com",
		},
	}
}

var ProductionRequiredEnvVars = []string{
	"TELEGRAM_BOT_TOKEN",
	"DB_PASSWORD",
	"SOLANA_RPC_URL",
}

// DevelopmentOnlyDefaults содержит значения ТОЛЬКО для разработки
type DevelopmentOnlyDefaults struct {
	DatabasePassword string
}

func GetDevelopmentDefaults() DevelopmentOnlyDefaults {
	return DevelopmentOnlyDefaults{
		DatabasePassword: "password",
	}
}
