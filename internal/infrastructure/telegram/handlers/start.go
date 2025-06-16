package handlers

import (
	"context"
	"fmt"
	"log"
	"time"

	"cash-farmer/internal/application/services"
	"cash-farmer/internal/infrastructure/telegram/sessions"
	"cash-farmer/internal/infrastructure/telegram/ui"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type StartHandler struct {
	services    *services.SimpleServiceContainer // ВРЕМЕННО: вернул обратно
	sessions    *sessions.SessionManager
	api         *tgbotapi.BotAPI
	logger      *log.Logger
	uiBuilder   *ui.Builder
	cacheHelper *CachedDataHelper // НОВОЕ: кэш помощник
}

func NewStartHandler(
	serviceContainer *services.SimpleServiceContainer, // ВРЕМЕННО: вернул обратно
	sessionManager *sessions.SessionManager,
	api *tgbotapi.BotAPI,
	logger *log.Logger,
) *StartHandler {
	return &StartHandler{
		services:    serviceContainer,
		sessions:    sessionManager,
		api:         api,
		logger:      logger,
		uiBuilder:   ui.NewBuilder(),
		cacheHelper: NewCachedDataHelper(serviceContainer), // НОВОЕ: инициализируем кэш
	}
}

func (h *StartHandler) HandleStart(ctx context.Context, message *tgbotapi.Message) error {
	userID := message.From.ID
	h.logger.Printf("Processing /start command for user: %d", userID)

	// Ensure user has a wallet first
	wallet, err := h.ensureUserWallet(ctx, userID)
	if err != nil {
		h.logger.Printf("Failed to ensure wallet for user %d: %v", userID, err)

		// Send error message
		msg := tgbotapi.NewMessage(userID, "❌ Failed to initialize wallet")
		_, err = h.api.Send(msg)
		return err
	}

	// Get wallet balance
	balance, err := h.services.GetWalletService().GetBalance(ctx, wallet.ID)
	if err != nil {
		h.logger.Printf("Failed to get balance for user %d: %v", userID, err)
		balance = 0 // Continue with 0 balance if unable to fetch
	}

	// Build welcome message using the same function as refresh
	text := h.buildWelcomeText(userID, wallet.PublicKey, balance)

	// Directly show main menu
	keyboard := h.uiBuilder.BuildMainMenuKeyboard()

	msg := tgbotapi.NewMessage(userID, text)
	msg.ParseMode = "HTML"
	msg.ReplyMarkup = keyboard

	_, err = h.api.Send(msg)
	return err
}

func (h *StartHandler) HandleHelp(ctx context.Context, message *tgbotapi.Message) error {
	userID := message.From.ID
	helpText := `
🤖 *Solana Cash Farmer Trading Bot*

Commands:
/start - Main menu and wallet setup
/help - Show this help message

Features:
• 💰 Buy/Sell tokens instantly
• 📊 Track your portfolio
• 👛 Manage multiple wallets
• ⚙️ Configure trading settings

To get started, use /start command!
`

	msg := tgbotapi.NewMessage(userID, helpText)
	msg.ParseMode = "Markdown"

	_, err := h.api.Send(msg)
	return err
}

func (h *StartHandler) HandleRefresh(ctx context.Context, query *tgbotapi.CallbackQuery) error {
	userID := query.From.ID
	h.logger.Printf("Handling refresh for user: %d", userID)

	// Get user's primary wallet
	wallet, err := h.getUserPrimaryWallet(ctx, userID)
	if err != nil {
		return h.editMessageWithError(ctx, query, "Failed to get wallet information")
	}

	// Get UPDATED balance - with proper refresh randomization
	newBalance, err := h.services.GetWalletService().GetBalance(ctx, wallet.ID)
	if err != nil {
		h.logger.Printf("Failed to get balance for user %d: %v", userID, err)
		newBalance = 0
	}

	// Always update message on refresh to show that something happened
	return h.editWelcomeMessage(ctx, query, wallet.PublicKey, newBalance)
}

func (h *StartHandler) HandleMainMenu(ctx context.Context, query *tgbotapi.CallbackQuery) error {
	userID := query.From.ID

	// Get user's primary wallet
	wallet, err := h.getUserPrimaryWallet(ctx, userID)
	if err != nil {
		return h.editMessageWithError(ctx, query, "Failed to get wallet information")
	}

	// Get wallet balance
	balance, err := h.services.GetWalletService().GetBalance(ctx, wallet.ID)
	if err != nil {
		balance = 0
	}

	return h.editWelcomeMessage(ctx, query, wallet.PublicKey, balance)
}

func (h *StartHandler) HandleMainMenuFromCallback(ctx context.Context, query *tgbotapi.CallbackQuery) error {
	return h.HandleMainMenu(ctx, query)
}

func (h *StartHandler) ensureUserWallet(ctx context.Context, userID int64) (*services.SimpleWallet, error) {
	// Try to get existing primary wallet
	wallets, err := h.services.GetWalletService().GetWallets(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get wallets: %w", err)
	}

	h.logger.Printf("User %d has %d wallets", userID, len(wallets))

	// Find primary wallet
	for _, wallet := range wallets {
		if wallet.IsDefault {
			h.logger.Printf("Found primary wallet for user %d: %s", userID, wallet.PublicKey)
			return &wallet, nil
		}
	}

	// If user has existing wallets but none are primary, use the first one as primary
	if len(wallets) > 0 {
		firstWallet := wallets[0]
		h.logger.Printf("User %d has %d wallets but none are primary. Using first wallet %s as primary",
			userID, len(wallets), firstWallet.PublicKey)

		// TODO: In real implementation, we'd update the database to mark this wallet as primary
		// For now, just return it and treat it as primary

		// Create a copy with IsDefault=true for this session
		primaryWallet := firstWallet
		primaryWallet.IsDefault = true

		return &primaryWallet, nil
	}

	h.logger.Printf("No wallets found for user %d, creating new one", userID)

	// No wallets at all, create new one
	newWallet, err := h.services.GetWalletService().CreateWallet(ctx, userID, "Main Wallet")
	if err != nil {
		return nil, err
	}

	h.logger.Printf("Created new wallet for user %d: %s", userID, newWallet.PublicKey)
	return newWallet, nil
}

func (h *StartHandler) getUserPrimaryWallet(ctx context.Context, userID int64) (*services.SimpleWallet, error) {
	wallets, err := h.services.GetWalletService().GetWallets(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Look for primary wallet
	for _, wallet := range wallets {
		if wallet.IsDefault {
			return &wallet, nil
		}
	}

	// If no primary wallet found, ensure user has a wallet
	return h.ensureUserWallet(ctx, userID)
}

func (h *StartHandler) sendWelcomeMessage(ctx context.Context, userID int64, walletAddress string, balance float64) error {
	// Delete previous message if exists
	if lastMsg := h.sessions.GetLastMessage(userID); lastMsg != nil {
		deleteMsg := tgbotapi.NewDeleteMessage(lastMsg.Chat.ID, lastMsg.MessageID)
		h.api.Send(deleteMsg) // Ignore errors for message deletion
	}

	text := h.buildWelcomeText(userID, walletAddress, balance)
	keyboard := h.uiBuilder.BuildMainMenuKeyboard()

	msg := tgbotapi.NewMessage(userID, text)
	msg.ParseMode = "HTML"
	msg.ReplyMarkup = keyboard

	sent, err := h.api.Send(msg)
	if err != nil {
		return err
	}

	// Update session with last message
	h.sessions.UpdateLastMessage(userID, &sent)
	return nil
}

func (h *StartHandler) editWelcomeMessage(ctx context.Context, query *tgbotapi.CallbackQuery, walletAddress string, balance float64) error {
	userID := query.From.ID
	text := h.buildWelcomeText(userID, walletAddress, balance)
	keyboard := h.uiBuilder.BuildMainMenuKeyboard()

	edit := tgbotapi.NewEditMessageText(query.Message.Chat.ID, query.Message.MessageID, text)
	edit.ParseMode = "HTML"
	edit.ReplyMarkup = &keyboard

	_, err := h.api.Send(edit)
	return err
}

func (h *StartHandler) buildWelcomeText(userID int64, walletAddress string, balance float64) string {
	// НОВАЯ ЛОГИКА: Используем кэшированную версию
	ctx := context.Background()

	// Пытаемся получить оптимизированный текст
	optimizedText, err := h.cacheHelper.BuildOptimizedWelcomeText(ctx, userID)
	if err == nil && optimizedText != "" {
		h.logger.Printf("✅ CACHE: Using optimized welcome text for user %d", userID)
		return optimizedText
	}

	h.logger.Printf("⚠️ CACHE: Fallback to legacy welcome text for user %d: %v", userID, err)

	// LEGACY FALLBACK: старая логика для совместимости
	usdValue := balance * 100 // Fallback: 1 SOL = $100
	pnlString := "🚀"

	// Get real SOL price from cache first
	if solPrice, err := h.cacheHelper.GetSOLPrice(ctx); err == nil {
		usdValue = balance * solPrice
	}

	// Get PnL from cache
	if cachedPnL, err := h.cacheHelper.GetPnLData(ctx, userID); err == nil {
		pnlString = cachedPnL
	}

	// Add deposit warning for zero balance
	var depositWarning string
	if balance == 0 {
		depositWarning = "\n\n❗️❗️<b>No SOL balance. To start trading, transfer SOL to wallet address</b>"
	}

	// Add timestamp to ensure message content is always different on refresh
	currentTime := time.Now().Format("15:04:05")

	return fmt.Sprintf(`🚀 <b>Welcome to Solana Cash Farmer Trading Bot!</b>

💳 <b>Wallet:</b>
<code>%s</code> (Tap to copy)

💰 <b>Balance:</b> %.4f SOL / $%.2f (PnL %s)

🕒 <b>Last Updated:</b> %s

👉🏻 <b>Start To Use:</b>
  ·  Start Trading: Send token contract address%s

Click the Refresh button to update your current balance.`,
		walletAddress, balance, usdValue, pnlString, currentTime, depositWarning)
}

func (h *StartHandler) sendErrorMessage(ctx context.Context, userID int64, errorText string) error {
	msg := tgbotapi.NewMessage(userID, "❌ "+errorText)
	_, err := h.api.Send(msg)
	return err
}

func (h *StartHandler) editMessageWithError(ctx context.Context, query *tgbotapi.CallbackQuery, errorText string) error {
	edit := tgbotapi.NewEditMessageText(query.Message.Chat.ID, query.Message.MessageID, "❌ "+errorText)
	_, err := h.api.Send(edit)
	return err
}
