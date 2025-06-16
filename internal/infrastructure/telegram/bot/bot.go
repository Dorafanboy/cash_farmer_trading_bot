package bot

import (
	"context"
	"log"
	"time"

	"cash-farmer/internal/application/services"
	"cash-farmer/internal/infrastructure/telegram/handlers"
	"cash-farmer/internal/infrastructure/telegram/sessions"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type TelegramBot struct {
	api          *tgbotapi.BotAPI
	services     *services.SimpleServiceContainer
	handlers     *handlers.HandlerRegistry
	sessions     *sessions.SessionManager
	logger       *log.Logger
	updateConfig tgbotapi.UpdateConfig
	done         chan struct{}
}

func NewTelegramBot(api *tgbotapi.BotAPI, serviceContainer *services.SimpleServiceContainer) (*TelegramBot, error) {
	logger := log.New(log.Writer(), "[TelegramBot] ", log.LstdFlags)

	// Initialize session manager
	sessionManager := sessions.NewSessionManager()

	// Initialize handlers with services and session manager
	handlerRegistry, err := handlers.NewHandlerRegistry(serviceContainer, sessionManager, api, logger)
	if err != nil {
		return nil, err
	}

	// Configure updates
	updateConfig := tgbotapi.NewUpdate(0)
	updateConfig.Timeout = 60

	return &TelegramBot{
		api:          api,
		services:     serviceContainer,
		handlers:     handlerRegistry,
		sessions:     sessionManager,
		logger:       logger,
		updateConfig: updateConfig,
		done:         make(chan struct{}),
	}, nil
}

func (b *TelegramBot) Start(ctx context.Context) error {
	b.logger.Println("Starting Telegram bot...")

	// Set bot commands menu
	commands := []tgbotapi.BotCommand{
		{Command: "start", Description: "🚀 Main menu and wallet setup"},
		{Command: "buy", Description: "💰 Buy tokens"},
		{Command: "sell", Description: "💸 Sell tokens"},
		{Command: "positions", Description: "📊 View your positions"},
		{Command: "wallets", Description: "👛 Manage wallets"},
		{Command: "settings", Description: "⚙️ Bot settings"},
		{Command: "netcheck", Description: "🌐 Network diagnostics"},
		{Command: "help", Description: "❓ Help and support"},
	}

	setCommands := tgbotapi.NewSetMyCommands(commands...)
	if _, err := b.api.Request(setCommands); err != nil {
		b.logger.Printf("Failed to set bot commands: %v", err)
	} else {
		b.logger.Println("Bot commands menu set successfully")
	}

	updates := b.api.GetUpdatesChan(b.updateConfig)

	for {
		select {
		case <-ctx.Done():
			b.logger.Println("Context cancelled, stopping bot...")
			return ctx.Err()
		case <-b.done:
			b.logger.Println("Bot stop signal received")
			return nil
		case update := <-updates:
			// Process update in background to avoid blocking
			go b.processUpdate(ctx, update)
		}
	}
}

func (b *TelegramBot) Stop() {
	b.logger.Println("Stopping bot...")
	close(b.done)
	b.api.StopReceivingUpdates()
}

func (b *TelegramBot) processUpdate(ctx context.Context, update tgbotapi.Update) {
	defer func() {
		if r := recover(); r != nil {
			b.logger.Printf("Panic in update processing: %v", r)
		}
	}()

	// Add timeout to prevent hanging
	updateCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if err := b.handlers.ProcessUpdate(updateCtx, update); err != nil {
		b.logger.Printf("Error processing update: %v", err)
	}
}
