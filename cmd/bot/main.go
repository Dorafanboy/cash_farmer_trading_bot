package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"cash-farmer/internal/application/services"
	"cash-farmer/internal/infrastructure/api"
	"cash-farmer/internal/infrastructure/blockchain/jito"
	"cash-farmer/internal/infrastructure/database"
	"cash-farmer/internal/infrastructure/database/repositories"
	"cash-farmer/internal/infrastructure/telegram/bot"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Printf("Warning: .env file not found or couldn't be loaded: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	botToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	if botToken == "" {
		log.Fatal("TELEGRAM_BOT_TOKEN environment variable is required")
	}

	port, _ := strconv.Atoi(getEnvOrDefault("DB_PORT", "5432"))
	dbConfig := database.Config{
		Host:     getEnvOrDefault("DB_HOST", "localhost"),
		Port:     port,
		User:     getEnvOrDefault("DB_USER", "postgres"),
		Password: getEnvOrDefault("DB_PASSWORD", "password"),
		Database: getEnvOrDefault("DB_NAME", "cash_farmer"),
		SSLMode:  getEnvOrDefault("DB_SSL_MODE", "disable"),
	}

	dbConnection, err := database.NewConnection(ctx, dbConfig)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer dbConnection.Close()

	redisURL := getEnvOrDefault("REDIS_URL", "redis://localhost:6379")
	redisOpt, err := redis.ParseURL(redisURL)
	if err != nil {
		log.Fatalf("Failed to parse Redis URL: %v", err)
	}

	var redisClient redis.UniversalClient = redis.NewClient(redisOpt)
	defer redisClient.Close()

	if err := redisClient.Ping(ctx).Err(); err != nil {
		log.Printf("Warning: Redis connection failed: %v", err)
	}

	walletRepository := repositories.NewWalletRepository(dbConnection.Queries)

	solanaRPCURL := getEnvOrDefault("SOLANA_RPC_URL", "https://mainnet.helius-rpc.com/?api-key=97f4303e-3cbd-4417-8c0f-bfdcf31e3cec")
	solanaClient := api.NewSolanaClient(solanaRPCURL)

	jupiterAPIURL := getEnvOrDefault("JUPITER_API_URL", "https://quote-api.jup.ag/v6")
	jupiterClient := api.NewJupiterClient(jupiterAPIURL)

	jitoRPCURL := getEnvOrDefault("JITO_RPC_URL", "https://amsterdam.mainnet.block-engine.jito.wtf/api/v1")
	jitoUUID := getEnvOrDefault("JITO_UUID", "")
	jitoDebug := getEnvOrDefault("JITO_DEBUG", "false") == "true"

	jitoClient := jito.NewJitoClient(jitoRPCURL)

	if jitoUUID != "" {
		log.Printf("Enhanced Jito Client: UUID authentication enabled")
		jitoClient.SetUUID(jitoUUID)
	} else {
		log.Printf("Enhanced Jito Client: Using public endpoints (no UUID)")
	}

	if jitoDebug {
		jitoClient.SetDebug(true)
	}

	log.Printf("Initialized Enhanced Jito Client with capabilities: %+v", jitoClient.GetCapabilities())

	dexScreenerAPIURL := getEnvOrDefault("DEXSCREENER_API_URL", "https://api.dexscreener.com")
	dexScreenerClient := api.NewDexscreenerClient(dexScreenerAPIURL)

	serviceContainer := services.NewServiceContainer(
		dbConnection.Queries,
		walletRepository,
		redisClient,
		jupiterClient,
		jitoClient,
		solanaClient,
		dexScreenerClient,
	)

	log.Printf("🔍 DEBUG MAIN: ServiceContainer created")

	defer func() {
		if err := serviceContainer.Shutdown(ctx); err != nil {
			log.Printf("Error shutting down service container: %v", err)
		}
	}()

	simpleServiceContainer := services.NewServiceContainerAdapter(serviceContainer)

	log.Printf("🔍 DEBUG MAIN: ServiceContainerAdapter created")

	tokenSwapService := simpleServiceContainer.GetTokenSwapService()
	log.Printf("🔍 DEBUG MAIN: TokenSwapService obtained: %v", tokenSwapService != nil)

	botAPI, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		log.Fatalf("Failed to create bot API: %v", err)
	}

	if os.Getenv("BOT_DEBUG") == "true" {
		botAPI.Debug = true
	}

	log.Printf("Authorized on account %s", botAPI.Self.UserName)

	telegramBot, err := bot.NewTelegramBot(botAPI, simpleServiceContainer)
	if err != nil {
		log.Fatalf("Failed to initialize bot: %v", err)
	}

	go func() {
		if err := telegramBot.Start(ctx); err != nil {
			log.Printf("Bot error: %v", err)
			cancel()
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-sigChan:
		log.Println("Received interrupt signal, shutting down...")
	case <-ctx.Done():
		log.Println("Context cancelled, shutting down...")
	}

	telegramBot.Stop()
	log.Println("Bot stopped")
}

// getEnvOrDefault returns environment variable value or default if not set
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
