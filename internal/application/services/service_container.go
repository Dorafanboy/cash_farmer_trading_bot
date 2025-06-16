package services

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"cash-farmer/internal/application/repositories"
	"cash-farmer/internal/infrastructure/api"
	"cash-farmer/internal/infrastructure/blockchain/config"
	"cash-farmer/internal/infrastructure/blockchain/jito"
	"cash-farmer/internal/infrastructure/blockchain/jupiter"
	"cash-farmer/internal/infrastructure/blockchain/solana"
	"cash-farmer/internal/infrastructure/blockchain/trading"
	"cash-farmer/internal/infrastructure/cache"
	"cash-farmer/internal/infrastructure/database/sqlc"
	infraServices "cash-farmer/internal/infrastructure/services"
)

// ServiceContainer holds all application services with dependency injection
type ServiceContainer struct {
	// Infrastructure
	db                 *sqlc.Queries
	walletRepo         repositories.WalletRepository
	cacheRepo          repositories.CacheRepository
	redisClient        redis.UniversalClient
	transactionManager *TransactionManager

	// Core Services
	solPriceService  SolPriceService
	tokenDataService TokenDataService
	priceService     PriceService
	tokenSwapService *TokenSwapService

	// Application Services
	walletService    WalletService
	portfolioService PortfolioService
	settingsService  SettingsService

	// Modular Blockchain Services (NEW)
	tradingService trading.TradingService
	jupiterService jupiter.JupiterService
	jitoService    jito.JitoService
	solanaService  solana.SolanaService
}

// NewServiceContainer creates a new service container with all dependencies
func NewServiceContainer(
	db *sqlc.Queries,
	walletRepo repositories.WalletRepository,
	redisClient redis.UniversalClient,
	jupiterClient *api.JupiterClient,
	jitoClient *api.JitoClient,
	solanaClient *api.SolanaClient,
	dexscreenerClient *api.DexscreenerClient,
) *ServiceContainer {

	// Create cache repository
	cacheRepo := cache.NewRedisCache(redisClient)

	// Create key manager
	keyManager, err := infraServices.NewKeyManager()
	if err != nil {
		fmt.Printf("Warning: Failed to create KeyManager: %v. Wallet encryption will be disabled.\n", err)
		// Create a nil keyManager - WalletService will handle this gracefully
		keyManager = nil
	}

	// Create core services - using DexScreener for SOL price
	solPriceService := infraServices.NewSolPriceServiceImpl(
		dexscreenerClient,
		cacheRepo,
		60*time.Second, // 60 second TTL
	)

	tokenDataService := infraServices.NewTokenDataServiceImpl(dexscreenerClient)

	// Create price service adapter
	priceService := NewPriceServiceAdapter(tokenDataService, solPriceService)

	// Create wallet service first (without TransactionManager)
	walletService := NewWalletService(walletRepo, nil, keyManager)

	// Create transaction manager with WalletService
	transactionManager := NewTransactionManager(
		jupiterClient,
		jitoClient,
		solanaClient,
		DefaultTransactionConfig(),
		walletService,
	)

	fmt.Printf("🔍 SERVICE CONTAINER: TransactionManager config: UseJitoPrimary=%t, JitoFallbackEnabled=%t\n",
		transactionManager.GetConfig().UseJitoPrimary,
		transactionManager.GetConfig().JitoFallbackEnabled)

	// Update wallet service with TransactionManager
	walletService.(*WalletServiceImpl).SetTransactionManager(transactionManager)

	// Create modular blockchain services with proper configuration
	blockchainConfig := config.DefaultBlockchainConfig()

	// Create Jupiter service with config
	jupiterService := jupiter.NewJupiterService(blockchainConfig)

	// Create Jito service with config
	jitoService := jito.NewJitoService(blockchainConfig)

	// Create Solana service with config
	solanaService, err := solana.NewSolanaService(blockchainConfig)
	if err != nil {
		fmt.Printf("Warning: Failed to create SolanaService: %v\n", err)
	}

	// Create Trading service that orchestrates all modular services
	tradingService, err := trading.NewTradingService(blockchainConfig)
	if err != nil {
		fmt.Printf("Warning: Failed to create TradingService: %v\n", err)
	}

	// Create token swap service (existing)
	tokenSwapService := NewTokenSwapService(transactionManager, jitoClient.GetEnhancedClient())
	portfolioService := NewPortfolioService(db, walletRepo, priceService)
	settingsService := NewSettingsService(db)

	return &ServiceContainer{
		db:                 db,
		walletRepo:         walletRepo,
		cacheRepo:          cacheRepo,
		redisClient:        redisClient,
		transactionManager: transactionManager,
		solPriceService:    solPriceService,
		tokenDataService:   tokenDataService,
		priceService:       priceService,
		walletService:      walletService,
		portfolioService:   portfolioService,
		settingsService:    settingsService,
		tokenSwapService:   tokenSwapService,
		tradingService:     tradingService,
		jupiterService:     jupiterService,
		jitoService:        jitoService,
		solanaService:      solanaService,
	}
}

// GetWalletService returns the wallet service
func (sc *ServiceContainer) GetWalletService() WalletService {
	return sc.walletService
}

// GetPortfolioService returns the portfolio service
func (sc *ServiceContainer) GetPortfolioService() PortfolioService {
	return sc.portfolioService
}

// GetSettingsService returns the settings service
func (sc *ServiceContainer) GetSettingsService() SettingsService {
	return sc.settingsService
}

// GetTransactionManager returns the transaction manager
func (sc *ServiceContainer) GetTransactionManager() *TransactionManager {
	return sc.transactionManager
}

// GetSolPriceService returns the SOL price service
func (sc *ServiceContainer) GetSolPriceService() SolPriceService {
	return sc.solPriceService
}

// GetTokenDataService returns the token data service
func (sc *ServiceContainer) GetTokenDataService() TokenDataService {
	return sc.tokenDataService
}

// GetPriceService returns the unified price service
func (sc *ServiceContainer) GetPriceService() PriceService {
	return sc.priceService
}

// GetTokenSwapService returns the token swap service
func (sc *ServiceContainer) GetTokenSwapService() *TokenSwapService {
	return sc.tokenSwapService
}

// GetTradingService returns the modular trading service
func (sc *ServiceContainer) GetTradingService() trading.TradingService {
	return sc.tradingService
}

// GetJupiterService returns the modular Jupiter service
func (sc *ServiceContainer) GetJupiterService() jupiter.JupiterService {
	return sc.jupiterService
}

// GetJitoService returns the modular Jito service
func (sc *ServiceContainer) GetJitoService() jito.JitoService {
	return sc.jitoService
}

// GetSolanaService returns the modular Solana service
func (sc *ServiceContainer) GetSolanaService() solana.SolanaService {
	return sc.solanaService
}

// Shutdown gracefully shuts down all services
func (sc *ServiceContainer) Shutdown(ctx context.Context) error {
	// Close Redis connection
	if sc.redisClient != nil {
		if err := sc.redisClient.Close(); err != nil {
			fmt.Printf("Warning: failed to close Redis client: %v\n", err)
		}
	}

	fmt.Println("ServiceContainer: All services shut down gracefully")
	return nil
}
