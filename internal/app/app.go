package app

import (
	"context"
	"fmt"
	"log"

	"cash-farmer/internal/application/services"
	"cash-farmer/internal/config"
	"cash-farmer/internal/infrastructure/api"
	"cash-farmer/internal/infrastructure/database"
	"cash-farmer/internal/infrastructure/database/repositories"
	"cash-farmer/internal/infrastructure/telegram/bot"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/redis/go-redis/v9"
)

type App struct {
	config           *config.Config
	dbConnection     *database.Connection
	redisClient      redis.UniversalClient
	serviceContainer *services.ServiceContainer
	telegramBot      *bot.TelegramBot
	ctx              context.Context
	cancel           context.CancelFunc
}

func New(cfg *config.Config) *App {
	ctx, cancel := context.WithCancel(context.Background())
	return &App{
		config: cfg,
		ctx:    ctx,
		cancel: cancel,
	}
}

func (a *App) Initialize() error {
	if err := a.initDatabase(); err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}

	if err := a.initRedis(); err != nil {
		return fmt.Errorf("failed to initialize Redis: %w", err)
	}

	if err := a.initServiceContainer(); err != nil {
		return fmt.Errorf("failed to initialize service container: %w", err)
	}

	if err := a.initTelegramBot(); err != nil {
		return fmt.Errorf("failed to initialize Telegram bot: %w", err)
	}

	return nil
}

func (a *App) Run() error {
	return a.RunWithContext(context.Background())
}

func (a *App) RunWithContext(ctx context.Context) error {
	combinedCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	go func() {
		<-combinedCtx.Done()
		a.cancel()
	}()

	botErrChan := make(chan error, 1)
	go func() {
		if err := a.telegramBot.Start(a.ctx); err != nil {
			botErrChan <- fmt.Errorf("bot error: %w", err)
		}
	}()

	select {
	case <-combinedCtx.Done():
		log.Println("Shutdown requested, stopping bot...")
		a.telegramBot.Stop()
		return nil
	case <-a.ctx.Done():
		log.Println("Context cancelled, shutting down...")
		a.telegramBot.Stop()
		return nil
	case err := <-botErrChan:
		log.Printf("Bot error occurred: %v", err)
		a.telegramBot.Stop()
		return err
	}
}

func (a *App) Shutdown() {
	if a.cancel != nil {
		a.cancel()
	}

	if a.serviceContainer != nil {
		if err := a.serviceContainer.Shutdown(a.ctx); err != nil {
			log.Printf("Error shutting down service container: %v", err)
		}
	}

	if a.redisClient != nil {
		a.redisClient.Close()
	}

	if a.dbConnection != nil {
		a.dbConnection.Close()
	}
}

func (a *App) initDatabase() error {
	dbConfig := database.Config{
		Host:     a.config.Database.Host,
		Port:     a.config.Database.Port,
		User:     a.config.Database.User,
		Password: a.config.Database.Password,
		Database: a.config.Database.Database,
		SSLMode:  a.config.Database.SSLMode,
	}

	var err error
	a.dbConnection, err = database.NewConnection(a.ctx, dbConfig)
	return err
}

func (a *App) initRedis() error {
	redisOpt, err := redis.ParseURL(a.config.Redis.URL)
	if err != nil {
		return fmt.Errorf("failed to parse Redis URL: %w", err)
	}

	a.redisClient = redis.NewClient(redisOpt)

	if err := a.redisClient.Ping(a.ctx).Err(); err != nil {
		log.Printf("Warning: Redis connection failed: %v", err)
	}

	return nil
}

func (a *App) initServiceContainer() error {
	walletRepository := repositories.NewWalletRepository(a.dbConnection.Queries)

	solanaClient := api.NewSolanaClient(a.config.Solana.RPCURL)
	jupiterClient := api.NewJupiterClient(a.config.Jupiter.APIURL)
	dexScreenerClient := api.NewDexscreenerClient(a.config.DexScreener.APIURL)

	jitoClient, err := a.initJitoClient()
	if err != nil {
		return fmt.Errorf("failed to initialize Jito client: %w", err)
	}

	a.serviceContainer = services.NewServiceContainer(
		a.dbConnection.Queries,
		walletRepository,
		a.redisClient,
		jupiterClient,
		jitoClient,
		solanaClient,
		dexScreenerClient,
	)

	log.Printf("🔍 DEBUG MAIN: ServiceContainer created")
	return nil
}

func (a *App) initJitoClient() (*api.JitoClient, error) {
	jitoClient := api.NewJitoClient(a.config.Jito.RPCURL)

	if a.config.Jito.UUID != "" {
		log.Printf("Enhanced Jito Client: UUID authentication enabled")
		jitoClient.SetUUID(a.config.Jito.UUID)
	} else {
		log.Printf("Enhanced Jito Client: Using public endpoints (no UUID)")
	}

	if a.config.Jito.Debug {
		jitoClient.SetDebug(true)
	}

	log.Printf("Initialized Enhanced Jito Client with capabilities: %+v", jitoClient.GetCapabilities())
	return jitoClient, nil
}

func (a *App) initTelegramBot() error {
	botAPI, err := tgbotapi.NewBotAPI(a.config.Telegram.BotToken)
	if err != nil {
		return fmt.Errorf("failed to create bot API: %w", err)
	}

	botAPI.Debug = a.config.Telegram.Debug
	log.Printf("Authorized on account %s", botAPI.Self.UserName)

	simpleServiceContainer := services.NewServiceContainerAdapter(a.serviceContainer)
	log.Printf("🔍 DEBUG MAIN: ServiceContainerAdapter created")

	tokenSwapService := simpleServiceContainer.GetTokenSwapService()
	log.Printf("🔍 DEBUG MAIN: TokenSwapService obtained: %v", tokenSwapService != nil)

	a.telegramBot, err = bot.NewTelegramBot(botAPI, simpleServiceContainer)
	if err != nil {
		return fmt.Errorf("failed to initialize bot: %w", err)
	}

	return nil
}
