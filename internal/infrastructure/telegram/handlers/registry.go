package handlers

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"cash-farmer/internal/application/services"
	"cash-farmer/internal/infrastructure/telegram/sessions"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type HandlerRegistry struct {
	services *services.SimpleServiceContainer
	sessions *sessions.SessionManager
	api      *tgbotapi.BotAPI
	logger   *log.Logger

	// Handlers
	startHandler     *StartHandler
	walletHandler    *WalletHandler
	tradingHandler   *TradingHandler
	portfolioHandler *PortfolioHandler
	settingsHandler  *SettingsHandler
	textProcessor    *TextProcessor
}

func NewHandlerRegistry(
	serviceContainer *services.SimpleServiceContainer,
	sessionManager *sessions.SessionManager,
	api *tgbotapi.BotAPI,
	logger *log.Logger,
) (*HandlerRegistry, error) {

	// Initialize handlers
	startHandler := NewStartHandler(serviceContainer, sessionManager, api, logger)
	walletHandler := NewWalletHandler(serviceContainer, sessionManager, api, logger)
	tradingHandler := NewTradingHandler(serviceContainer, sessionManager, api, logger)
	portfolioHandler := NewPortfolioHandler(serviceContainer, sessionManager, api, logger)
	settingsHandler := NewSettingsHandler(serviceContainer, sessionManager, api, logger)
	textProcessor := NewTextProcessor(serviceContainer, sessionManager, api, logger)

	return &HandlerRegistry{
		services:         serviceContainer,
		sessions:         sessionManager,
		api:              api,
		logger:           logger,
		startHandler:     startHandler,
		walletHandler:    walletHandler,
		tradingHandler:   tradingHandler,
		portfolioHandler: portfolioHandler,
		settingsHandler:  settingsHandler,
		textProcessor:    textProcessor,
	}, nil
}

func (hr *HandlerRegistry) ProcessUpdate(ctx context.Context, update tgbotapi.Update) error {
	// Handle callback queries (button presses)
	if update.CallbackQuery != nil {
		return hr.processCallbackQuery(ctx, update.CallbackQuery)
	}

	// Handle text messages and commands
	if update.Message != nil {
		// Handle commands
		if update.Message.IsCommand() {
			return hr.processCommand(ctx, update.Message)
		}

		// Handle text messages
		return hr.processTextMessage(ctx, update.Message)
	}

	return nil
}

func (hr *HandlerRegistry) processCommand(ctx context.Context, message *tgbotapi.Message) error {
	command := message.Command()
	hr.logger.Printf("Processing command: %s from user: %d", command, message.From.ID)

	switch command {
	case "start":
		return hr.startHandler.HandleStart(ctx, message)
	case "help":
		return hr.startHandler.HandleHelp(ctx, message)
	case "buy":
		return hr.redirectToCallback(ctx, message, "buy")
	case "sell":
		return hr.redirectToCallback(ctx, message, "sell")
	case "positions":
		return hr.redirectToCallback(ctx, message, "positions")
	case "wallets":
		return hr.redirectToCallback(ctx, message, "manage_wallets")
	case "settings":
		return hr.redirectToCallback(ctx, message, "settings")
	case "netcheck":
		return hr.handleNetworkDiagnostics(ctx, message)
	default:
		// Unknown command - show help
		return hr.startHandler.HandleHelp(ctx, message)
	}
}

func (hr *HandlerRegistry) processCallbackQuery(ctx context.Context, query *tgbotapi.CallbackQuery) error {
	hr.logger.Printf("🚀🚀🚀 NEW CALLBACK CODE! Processing callback: %s from user: %d", query.Data, query.From.ID)

	// Parse callback data
	parts := strings.Split(query.Data, ":")
	action := parts[0]

	// Answer callback query to remove loading state
	if err := hr.answerCallbackQuery(query.ID); err != nil {
		hr.logger.Printf("Failed to answer callback query: %v", err)
	}

	// Route to appropriate handler
	switch action {
	case "buy":
		return hr.tradingHandler.HandleBuy(ctx, query)
	case "sell":
		return hr.tradingHandler.HandleSell(ctx, query)
	case "positions":
		return hr.portfolioHandler.HandlePositions(ctx, query)
	case "settings":
		return hr.settingsHandler.HandleSettings(ctx, query)
	// Settings submenu handlers
	case "settings_buy":
		return hr.settingsHandler.HandleBuySettings(ctx, query)
	case "settings_sell":
		return hr.settingsHandler.HandleSellSettings(ctx, query)
	case "settings_priority":
		return hr.settingsHandler.HandlePrioritySettings(ctx, query)
	case "settings_slippage":
		return hr.settingsHandler.HandleSlippageSettings(ctx, query)
	case "settings_mev":
		return hr.settingsHandler.HandleMEVSettings(ctx, query)
	case "settings_reset":
		return hr.handleSettingsReset(ctx, query)
	case "manage_wallets":
		return hr.walletHandler.HandleManageWallets(ctx, query)
	case "refresh":
		return hr.startHandler.HandleRefresh(ctx, query)
	case "wallet_generate":
		return hr.walletHandler.HandleGenerateWallet(ctx, query)
	case "wallet_import":
		return hr.walletHandler.HandleImportWallet(ctx, query)
	case "wallet_export":
		return hr.walletHandler.HandleExportWallet(ctx, query)
	case "wallet_delete":
		return hr.walletHandler.HandleDeleteWallet(ctx, query)
	case "wallet_set_primary":
		return hr.walletHandler.HandleSetPrimaryWallet(ctx, query)
	case "export_wallet":
		return hr.handleExportWalletCallback(ctx, query)
	case "set_primary":
		return hr.handleSetPrimaryCallback(ctx, query)
	case "delete_wallet":
		return hr.handleDeleteWalletCallback(ctx, query)
	case "sell_percent":
		return hr.tradingHandler.HandleSellPercent(ctx, query)
	case "custom_amount":
		return hr.tradingHandler.HandleCustomAmount(ctx, query)
	case "back":
		return hr.handleBack(ctx, query)
	case "menu":
		return hr.startHandler.HandleMainMenu(ctx, query)
	case "main_menu":
		return hr.startHandler.HandleMainMenu(ctx, query)
	case "noop":
		return nil // Do nothing for disabled buttons
	case "buy_amount_edit":
		return hr.handleBuyAmountEdit(ctx, query)
	case "buy_slippage_edit":
		return hr.handleBuySlippageEdit(ctx, query)
	case "buy_preset_edit":
		return hr.handleBuyPresetEdit(ctx, query)
	case "sell_preset_edit":
		return hr.handleSellPresetEditFlow(ctx, query)
	case "sell_slippage_edit":
		return hr.handleEditSellSlippage(ctx, query)
	case "priority_fee_edit":
		return hr.handleMEVPriorityEdit(ctx, query)
	case "jito_tip_edit":
		return hr.handleMEVJitoEdit(ctx, query)
	case "mev_toggle_on":
		return hr.handleMEVToggle(ctx, query, true)
	case "mev_toggle_off":
		return hr.handleMEVToggle(ctx, query, false)
	case "buy_preset1":
		return hr.handleBuyPreset(ctx, query, 1)
	case "buy_preset2":
		return hr.handleBuyPreset(ctx, query, 2)
	case "buy_preset3":
		return hr.handleBuyPreset(ctx, query, 3)
	case "buy_preset4":
		return hr.handleBuyPreset(ctx, query, 4)
	case "buy_preset5":
		return hr.handleBuyPreset(ctx, query, 5)
	case "edit_custom_amount":
		return hr.handleEditCustomAmountShort(ctx, query)
	case "edit_slippage":
		return hr.handleEditSlippage(ctx, query)
	case "edit_sell_slippage":
		return hr.handleEditSellSlippage(ctx, query)
	case "enter_sell_token":
		return hr.handleEnterSellToken(ctx, query)
	case "confirm_sell":
		return hr.handleConfirmSell(ctx, query)
	case "refresh_token":
		return hr.handleRefreshToken(ctx, query)
	case "confirm_buy":
		// НОВЫЙ КОД: Используем модульный TradingService!
		return hr.tradingHandler.HandleBuyPreset(ctx, query)
	case "trade_panel":
		// Возврат к торговой панели (главное меню)
		return hr.startHandler.HandleMainMenu(ctx, query)
	case "netcheck_repeat":
		return hr.handleNetworkDiagnosticsRepeat(ctx, query)
	case "sell_positions":
		return hr.tradingHandler.HandleSellPositions(ctx, query)
	case "sell_positions_refresh":
		return hr.tradingHandler.HandleSellPositions(ctx, query) // Refresh = same as positions
	case "sell_select_token":
		return hr.tradingHandler.HandleSelectTokenForSell(ctx, query)
	default:
		// Handle underscore-separated callbacks for new format
		if strings.Contains(query.Data, "_") {
			underscoreParts := strings.Split(query.Data, "_")
			switch underscoreParts[0] {
			case "buy":
				if len(underscoreParts) >= 3 && underscoreParts[1] == "token" {
					// buy_token_{address}_{amount} -> Handle token purchase
					return hr.handleTokenPurchase(ctx, query)
				} else if len(underscoreParts) >= 3 && underscoreParts[1] == "custom" {
					// buy_custom_{address} -> Handle custom amount
					return hr.handleBuyCustomAmount(ctx, query)
				} else if len(underscoreParts) >= 3 && underscoreParts[1] == "preset" {
					// buy_preset_0.01 -> HandleBuyPresetSettings
					return hr.handleBuyPresetSettings(ctx, query)
				} else if len(underscoreParts) >= 3 && underscoreParts[1] == "slippage" {
					// Old slippage format - no longer supported, use edit buttons
					hr.logger.Printf("Old slippage format not supported: %s", query.Data)
					return nil
				}
			case "edit":
				if len(underscoreParts) >= 3 && underscoreParts[1] == "custom" && underscoreParts[2] == "amount" {
					// edit_custom_amount_{address} -> Handle custom amount editing
					return hr.handleEditCustomAmount(ctx, query)
				} else if len(underscoreParts) >= 2 && underscoreParts[1] == "slippage" {
					// edit_slippage_{address} -> Handle slippage editing
					return hr.handleEditSlippage(ctx, query)
				}
			case "refresh":
				if len(underscoreParts) >= 2 && underscoreParts[1] == "token" {
					// refresh_token_{address} -> Handle token refresh
					return hr.handleRefreshToken(ctx, query)
				}
			case "sell":
				if len(underscoreParts) >= 3 && underscoreParts[1] == "select" && underscoreParts[2] == "token" {
					// sell_select_token:ADDRESS -> Handle token selection for sell
					return hr.tradingHandler.HandleSelectTokenForSell(ctx, query)
				} else if len(underscoreParts) >= 3 && underscoreParts[1] == "preset" {
					// sell_preset_25 -> HandleSellPercent (ИСПРАВЛЕНО: выполняем продажу, а не настройки!)
					return hr.tradingHandler.HandleSellPercent(ctx, query)
				} else if len(underscoreParts) >= 3 && underscoreParts[1] == "slippage" {
					// Old slippage format - no longer supported, use edit buttons
					hr.logger.Printf("Old slippage format not supported: %s", query.Data)
					return nil
				}
			case "priority":
				if len(underscoreParts) >= 3 && underscoreParts[1] == "preset" {
					// priority_preset_0.001 -> HandlePriorityPresetSettings
					return hr.handlePriorityPresetSettings(ctx, query)
				}
			default:
				// Handle old format with colons for backward compatibility
				if action == "buy_preset" && len(parts) > 1 {
					return hr.handleBuyPresetSettings(ctx, query)
				} else if action == "sell_preset" {
					return hr.handleSellPresetSettings(ctx, query)
				} else if action == "priority_preset" {
					return hr.handlePriorityPresetSettings(ctx, query)
				}
			}
		}

		hr.logger.Printf("Unknown callback action: %s", action)
		return nil
	}
}

func (hr *HandlerRegistry) processTextMessage(ctx context.Context, message *tgbotapi.Message) error {
	hr.logger.Printf("Processing text message from user: %d", message.From.ID)

	// Let text processor handle based on current user flow
	return hr.textProcessor.ProcessText(ctx, message)
}

func (hr *HandlerRegistry) handleBack(ctx context.Context, query *tgbotapi.CallbackQuery) error {
	// Clear current flow and return to main menu
	hr.sessions.SetFlow(query.From.ID, sessions.FlowNone, nil)
	return hr.startHandler.HandleMainMenuFromCallback(ctx, query)
}

func (hr *HandlerRegistry) answerCallbackQuery(callbackQueryID string) error {
	callback := tgbotapi.NewCallback(callbackQueryID, "")
	_, err := hr.api.Request(callback)
	return err
}

func (hr *HandlerRegistry) handleExportWalletCallback(ctx context.Context, query *tgbotapi.CallbackQuery) error {
	// Parse wallet ID from callback data: export_wallet:12
	parts := strings.Split(query.Data, ":")
	if len(parts) != 2 {
		hr.logger.Printf("Invalid export_wallet callback format: %s", query.Data)
		return nil
	}

	walletID := parts[1]
	userID := query.From.ID

	hr.logger.Printf("Exporting wallet %s for user %d", walletID, userID)

	// Get user wallets to find the specific one
	wallets, err := hr.services.GetWalletService().GetWallets(ctx, userID)
	if err != nil {
		return hr.editMessageWithError(ctx, query, "Failed to load wallets")
	}

	// Find the wallet to export
	var targetWallet *services.SimpleWallet
	for _, wallet := range wallets {
		if wallet.ID == walletID {
			targetWallet = &wallet
			break
		}
	}

	if targetWallet == nil {
		return hr.editMessageWithError(ctx, query, "Wallet not found")
	}

	// Show private key with security warning
	// Get the real private key from the service
	privateKey, err := hr.services.GetWalletService().ExportPrivateKey(ctx, userID, walletID)
	if err != nil {
		hr.logger.Printf("Failed to export private key for wallet %s: %v", walletID, err)
		return hr.editMessageWithError(ctx, query, "Failed to export private key")
	}

	text := fmt.Sprintf(`🔐 <b>Private Key Export</b>

💳 <b>Wallet:</b> <code>%s</code>

🔑 <b>Private Key:</b>
<code>%s</code>

⚠️ <b>CRITICAL SECURITY WARNING:</b>
• NEVER share this private key with anyone
• Store it in a secure location
• This message will auto-delete in 30 seconds
• Anyone with this key can access your funds

Tap the private key to copy it.`, targetWallet.PublicKey, privateKey)

	// Send the private key message
	edit := tgbotapi.NewEditMessageText(query.Message.Chat.ID, query.Message.MessageID, text)
	edit.ParseMode = "HTML"

	// Add a back button
	backKeyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("← Back to Wallets", "manage_wallets"),
		),
	)
	edit.ReplyMarkup = &backKeyboard

	sentMessage, err := hr.api.Send(edit)
	if err != nil {
		return err
	}

	// Запускаю горутину для автоудаления сообщения через 30 секунд
	go func() {
		time.Sleep(30 * time.Second)

		// Удаляю сообщение с приватным ключом
		deleteMsg := tgbotapi.NewDeleteMessage(sentMessage.Chat.ID, sentMessage.MessageID)
		_, deleteErr := hr.api.Request(deleteMsg)
		if deleteErr != nil {
			hr.logger.Printf("Failed to auto-delete private key message for user %d: %v", userID, deleteErr)
		} else {
			hr.logger.Printf("Auto-deleted private key message for user %d after 30 seconds", userID)
		}
	}()

	return nil
}

func (hr *HandlerRegistry) handleSetPrimaryCallback(ctx context.Context, query *tgbotapi.CallbackQuery) error {
	// Parse wallet ID from callback data: set_primary:12
	parts := strings.Split(query.Data, ":")
	if len(parts) != 2 {
		hr.logger.Printf("Invalid set_primary callback format: %s", query.Data)
		return nil
	}

	walletID := parts[1]
	userID := query.From.ID

	hr.logger.Printf("Setting wallet %s as primary for user %d", walletID, userID)

	// Get user wallets
	wallets, err := hr.services.GetWalletService().GetWallets(ctx, userID)
	if err != nil {
		return hr.editMessageWithError(ctx, query, "Failed to load wallets")
	}

	// Find the wallet to make primary
	var targetWallet *services.SimpleWallet
	for _, wallet := range wallets {
		if wallet.ID == walletID {
			targetWallet = &wallet
			break
		}
	}

	if targetWallet == nil {
		return hr.editMessageWithError(ctx, query, "Wallet not found")
	}

	// Use real SetPrimaryWallet method
	err = hr.services.GetWalletService().SetPrimaryWallet(ctx, userID, walletID)
	if err != nil {
		hr.logger.Printf("Failed to set primary wallet: %v", err)
		return hr.editMessageWithError(ctx, query, "Failed to update primary wallet")
	}

	text := fmt.Sprintf(`✅ <b>Primary Wallet Updated</b>

🟢 <b>New Primary Wallet:</b>
<code>%s</code>

Your default wallet has been changed successfully.
This wallet will now be used for all trading operations.`, targetWallet.PublicKey)

	// Send success message
	edit := tgbotapi.NewEditMessageText(query.Message.Chat.ID, query.Message.MessageID, text)
	edit.ParseMode = "HTML"

	// Add navigation buttons
	backKeyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("← Back to Wallets", "manage_wallets"),
			tgbotapi.NewInlineKeyboardButtonData("🏠 Main Menu", "menu"),
		),
	)
	edit.ReplyMarkup = &backKeyboard

	_, err = hr.api.Send(edit)
	return err
}

func (hr *HandlerRegistry) editMessageWithError(ctx context.Context, query *tgbotapi.CallbackQuery, errorText string) error {
	edit := tgbotapi.NewEditMessageText(query.Message.Chat.ID, query.Message.MessageID, "❌ "+errorText)

	// Add back to main menu button so user can recover
	backKeyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🏠 Main Menu", "menu"),
		),
	)
	edit.ReplyMarkup = &backKeyboard

	_, err := hr.api.Send(edit)
	return err
}

func (hr *HandlerRegistry) redirectToCallback(ctx context.Context, message *tgbotapi.Message, action string) error {
	// Create a mock callback query to reuse existing handlers
	mockQuery := &tgbotapi.CallbackQuery{
		ID:   "mock_" + action,
		From: message.From,
		Message: &tgbotapi.Message{
			Chat:      message.Chat,
			MessageID: message.MessageID,
		},
		Data: action,
	}

	// Answer the mock callback (no-op)
	// Process through existing callback handler
	return hr.processCallbackQuery(ctx, mockQuery)
}

func (hr *HandlerRegistry) handleDeleteWalletCallback(ctx context.Context, query *tgbotapi.CallbackQuery) error {
	// Parse wallet ID from callback data: delete_wallet:12
	parts := strings.Split(query.Data, ":")
	if len(parts) != 2 {
		hr.logger.Printf("Invalid delete_wallet callback format: %s", query.Data)
		return nil
	}

	walletID := parts[1]
	userID := query.From.ID

	hr.logger.Printf("Deleting wallet %s for user %d", walletID, userID)

	// Get user wallets
	wallets, err := hr.services.GetWalletService().GetWallets(ctx, userID)
	if err != nil {
		return hr.editMessageWithError(ctx, query, "Failed to load wallets")
	}

	// Find the wallet to delete
	var targetWallet *services.SimpleWallet
	for _, wallet := range wallets {
		if wallet.ID == walletID {
			targetWallet = &wallet
			break
		}
	}

	if targetWallet == nil {
		return hr.editMessageWithError(ctx, query, "Wallet not found")
	}

	// Use real DeleteWallet method
	err = hr.services.GetWalletService().DeleteWallet(ctx, userID, walletID)
	if err != nil {
		hr.logger.Printf("Failed to delete wallet: %v", err)
		return hr.editMessageWithError(ctx, query, "Failed to delete wallet")
	}

	text := fmt.Sprintf(`✅ <b>Wallet Deleted</b>

🟢 <b>Wallet:</b>
<code>%s</code>

Your wallet has been deleted successfully.
This action cannot be undone.`, targetWallet.PublicKey)

	// Send success message
	edit := tgbotapi.NewEditMessageText(query.Message.Chat.ID, query.Message.MessageID, text)
	edit.ParseMode = "HTML"

	// Add navigation buttons
	backKeyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🏠 Main Menu", "menu"),
		),
	)
	edit.ReplyMarkup = &backKeyboard

	_, err = hr.api.Send(edit)
	return err
}

// ========== SETTINGS CALLBACK HANDLERS ==========

// handleSettingsReset сбрасывает настройки к значениям по умолчанию
func (hr *HandlerRegistry) handleSettingsReset(ctx context.Context, query *tgbotapi.CallbackQuery) error {
	userID := query.From.ID
	hr.logger.Printf("Resetting settings to defaults for user %d", userID)

	err := hr.services.GetSettingsService().ResetToDefaults(ctx, userID)
	if err != nil {
		hr.logger.Printf("Failed to reset settings for user %d: %v", userID, err)
		return hr.editMessageWithError(ctx, query, "Failed to reset settings")
	}

	text := `✅ <b>Settings Reset</b>

All your settings have been reset to default values:

💰 <b>Buy Settings:</b>
• Default Amount: 0.1 SOL
• Slippage: 1.0%

💸 <b>Sell Settings:</b>  
• Slippage: 1.0%

🛡️ <b>MEV Protection:</b>
• Priority Fee: 1,000 lamports
• Jito Tip: 100,000 lamports

Your settings have been successfully reset.`

	// Return to main settings menu
	keyboard := hr.settingsHandler.uiBuilder.BuildSettingsMenuKeyboard()

	edit := tgbotapi.NewEditMessageText(query.Message.Chat.ID, query.Message.MessageID, text)
	edit.ParseMode = "HTML"
	edit.ReplyMarkup = &keyboard

	_, err = hr.api.Send(edit)
	return err
}

// handleBuyAmountEdit запускает flow для редактирования default amount
func (hr *HandlerRegistry) handleBuyAmountEdit(ctx context.Context, query *tgbotapi.CallbackQuery) error {
	userID := query.From.ID
	hr.logger.Printf("Starting buy amount edit flow for user %d", userID)

	// Set flow for text input
	hr.sessions.SetFlow(userID, sessions.FlowBuyAmountEdit, nil)

	text := `🎯 <b>Edit Default Buy Amount</b>

Please send the new default amount in SOL.

<b>Examples:</b>
• <code>0.01</code> - for 0.01 SOL
• <code>0.5</code> - for 0.5 SOL
• <code>2.5</code> - for 2.5 SOL

💡 <b>Current:</b> Send any valid SOL amount now:`

	keyboard := hr.settingsHandler.uiBuilder.BuildBackMenuKeyboard()

	edit := tgbotapi.NewEditMessageText(query.Message.Chat.ID, query.Message.MessageID, text)
	edit.ParseMode = "HTML"
	edit.ReplyMarkup = &keyboard

	_, err := hr.api.Send(edit)
	return err
}

// handleBuySlippageEdit запускает flow для редактирования buy slippage
func (hr *HandlerRegistry) handleBuySlippageEdit(ctx context.Context, query *tgbotapi.CallbackQuery) error {
	userID := query.From.ID
	hr.logger.Printf("Starting buy slippage edit flow for user %d", userID)

	// Set flow for text input
	hr.sessions.SetFlow(userID, sessions.FlowBuySlippageEdit, nil)

	text := `📊 <b>Edit Buy Slippage</b>

Please send the new buy slippage percentage.

<b>Examples:</b>
• <code>0.5</code> - for 0.5% slippage
• <code>1</code> - for 1% slippage
• <code>3</code> - for 3% slippage

💡 <b>Recommended:</b> 0.5-3% for most tokens
⚠️ <b>Higher slippage:</b> Use carefully, may result in worse prices

Send buy slippage percentage now:`

	keyboard := hr.settingsHandler.uiBuilder.BuildBackMenuKeyboard()

	edit := tgbotapi.NewEditMessageText(query.Message.Chat.ID, query.Message.MessageID, text)
	edit.ParseMode = "HTML"
	edit.ReplyMarkup = &keyboard

	_, err := hr.api.Send(edit)
	return err
}

// handleBuyPresetEdit запускает flow для редактирования конкретного buy preset
func (hr *HandlerRegistry) handleBuyPresetEdit(ctx context.Context, query *tgbotapi.CallbackQuery) error {
	userID := query.From.ID

	// Parse preset index from callback data: buy_preset_edit:1
	parts := strings.Split(query.Data, ":")
	if len(parts) != 2 {
		hr.logger.Printf("Invalid buy_preset_edit callback format: %s", query.Data)
		return nil
	}

	presetIndex, err := strconv.Atoi(parts[1])
	if err != nil || presetIndex < 1 || presetIndex > 5 {
		hr.logger.Printf("Invalid buy preset index: %s", parts[1])
		return nil
	}

	hr.logger.Printf("Starting buy preset %d edit flow for user %d", presetIndex, userID)

	// Set flow for text input with preset index
	flowData := map[string]interface{}{
		"preset_index": presetIndex,
	}
	hr.sessions.SetFlow(userID, sessions.FlowBuyPresetEdit, flowData)

	text := fmt.Sprintf(`💰 <b>Edit Buy Preset %d</b>

Please send the new amount in SOL for preset %d.

<b>Examples:</b>
• <code>0.01</code> - for 0.01 SOL
• <code>0.05</code> - for 0.05 SOL
• <code>0.1</code> - for 0.1 SOL
• <code>0.5</code> - for 0.5 SOL
• <code>1</code> - for 1 SOL

💡 <b>Range:</b> 0.001 to 100 SOL
🎯 <b>Tips:</b> Choose amounts you use frequently

Send the SOL amount for preset %d now:`, presetIndex, presetIndex, presetIndex)

	keyboard := hr.settingsHandler.uiBuilder.BuildBackMenuKeyboard()

	edit := tgbotapi.NewEditMessageText(query.Message.Chat.ID, query.Message.MessageID, text)
	edit.ParseMode = "HTML"
	edit.ReplyMarkup = &keyboard

	_, err = hr.api.Send(edit)
	return err
}

// handleSellSlippageEdit запускает flow для редактирования sell slippage
func (hr *HandlerRegistry) handleEditSellSlippage(ctx context.Context, query *tgbotapi.CallbackQuery) error {
	userID := query.From.ID
	hr.logger.Printf("🔍 EDIT SELL SLIPPAGE: User %d wants to edit sell slippage", userID)

	text := `✏️ <b>Edit Sell Slippage</b>

📊 Current slippage will be used for token sales.

💡 <b>Enter new slippage percentage (1-50):</b>

⚠️ <b>Examples:</b>
• <code>5</code> - 5% slippage (recommended)
• <code>10</code> - 10% slippage (higher tolerance)
• <code>1</code> - 1% slippage (may fail on volatile tokens)

🔙 Send any message to cancel and return to sell menu.`

	edit := tgbotapi.NewEditMessageText(query.Message.Chat.ID, query.Message.MessageID, text)
	edit.ParseMode = "HTML"
	_, err := hr.api.Send(edit)
	if err != nil {
		return err
	}

	// Set flow for sell slippage editing
	hr.sessions.SetFlow(userID, sessions.FlowSellSlippageEdit, nil)
	return nil
}

// handleMEVPriorityEdit запускает flow для редактирования priority fee
func (hr *HandlerRegistry) handleMEVPriorityEdit(ctx context.Context, query *tgbotapi.CallbackQuery) error {
	userID := query.From.ID
	hr.logger.Printf("Starting priority fee edit flow for user %d", userID)

	// Set flow for text input
	hr.sessions.SetFlow(userID, sessions.FlowPriorityFeeEdit, nil)

	text := `⚡ <b>Edit Priority Fee</b>

Please send the new priority fee in SOL.

<b>Examples:</b>
• <code>0.001</code> - for 0.001 SOL (low priority)
• <code>0.01</code> - for 0.01 SOL (medium priority)
• <code>0.05</code> - for 0.05 SOL (high priority)

💡 <b>What is it?</b> Network fee for transaction priority
📊 <b>Higher fee = faster processing</b>

Send priority fee in SOL now:`

	keyboard := hr.settingsHandler.uiBuilder.BuildBackMenuKeyboard()

	edit := tgbotapi.NewEditMessageText(query.Message.Chat.ID, query.Message.MessageID, text)
	edit.ParseMode = "HTML"
	edit.ReplyMarkup = &keyboard

	_, err := hr.api.Send(edit)
	return err
}

// handleMEVJitoEdit запускает flow для редактирования Jito tip
func (hr *HandlerRegistry) handleMEVJitoEdit(ctx context.Context, query *tgbotapi.CallbackQuery) error {
	userID := query.From.ID
	hr.logger.Printf("Starting Jito tip edit flow for user %d", userID)

	// Set flow for text input
	hr.sessions.SetFlow(userID, sessions.FlowJitoTipEdit, nil)

	text := `🎯 <b>Edit Jito Tip</b>

Please send the new Jito tip in SOL.

<b>Examples:</b>
• <code>0.0001</code> - for 0.0001 SOL (small trades)
• <code>0.0005</code> - for 0.0005 SOL (medium trades)
• <code>0.001</code> - for 0.001 SOL (large trades)

💡 <b>Recommendations:</b>
• Small trades (&lt;0.1 SOL): 0.0001 SOL
• Medium trades (0.1-1 SOL): 0.0005 SOL  
• Large trades (&gt;1 SOL): 0.001+ SOL

Send Jito tip in SOL now:`

	keyboard := hr.settingsHandler.uiBuilder.BuildBackMenuKeyboard()

	edit := tgbotapi.NewEditMessageText(query.Message.Chat.ID, query.Message.MessageID, text)
	edit.ParseMode = "HTML"
	edit.ReplyMarkup = &keyboard

	_, err := hr.api.Send(edit)
	return err
}

// handleMEVToggle переключает MEV защиту вкл/выкл
func (hr *HandlerRegistry) handleMEVToggle(ctx context.Context, query *tgbotapi.CallbackQuery, enable bool) error {
	userID := query.From.ID
	hr.logger.Printf("Toggling MEV protection to %v for user %d", enable, userID)

	// Get current settings
	settings, err := hr.services.GetSettingsService().GetUserSettings(ctx, userID)
	if err != nil {
		hr.logger.Printf("Failed to get settings for user %d: %v", userID, err)
		return hr.editMessageWithError(ctx, query, "Failed to load settings")
	}

	// Toggle MEV by setting/unsetting Jito tip
	var newJitoTip int64
	if enable {
		// If enabling and Jito tip is 0, set to default
		if settings.JitoTip == 0 {
			newJitoTip = 100000 // Default 100k lamports
		} else {
			newJitoTip = settings.JitoTip // Keep current value
		}
	} else {
		// If disabling, set to 0
		newJitoTip = 0
	}

	// Update settings
	err = hr.services.GetSettingsService().UpdateFees(ctx, userID, settings.PriorityFee, newJitoTip)
	if err != nil {
		hr.logger.Printf("Failed to update MEV settings for user %d: %v", userID, err)
		return hr.editMessageWithError(ctx, query, "Failed to update MEV settings")
	}

	// Return to MEV settings page (not main settings)
	return hr.settingsHandler.HandleMEVSettings(ctx, query)
}

// handleMEVLearn показывает образовательную информацию о MEV
func (hr *HandlerRegistry) handleMEVLearn(ctx context.Context, query *tgbotapi.CallbackQuery) error {
	userID := query.From.ID
	hr.logger.Printf("Showing MEV education for user %d", userID)

	text := `❓ <b>About MEV Protection</b>

<b>🎯 What are MEV attacks?</b>
MEV bots monitor pending transactions and can:
• <b>Front-run</b> your buy orders (you pay higher price)
• <b>Sandwich</b> your trades (steal profits)
• <b>Copy</b> successful strategies

<b>🛡️ How Jito protects you:</b>
• Bundles transactions privately
• Executes them atomically  
• Prevents front-running
• 99%+ success rate

<b>💰 Cost breakdown:</b>
• <b>Priority Fee:</b> Network priority
• <b>Jito Tip:</b> Bundle inclusion fee
• <b>Total:</b> Usually $0.005-0.02 per trade

<b>📊 When to use:</b>
✅ Trading volatile tokens
✅ Large transactions (>0.1 SOL)
✅ Popular/trending tokens
❌ Very small test transactions

<b>💡 Bottom line:</b>
MEV protection costs a few cents but can save you dollars or more per trade by preventing attacks.`

	keyboard := hr.settingsHandler.uiBuilder.BuildMEVEducationKeyboard()

	edit := tgbotapi.NewEditMessageText(query.Message.Chat.ID, query.Message.MessageID, text)
	edit.ParseMode = "HTML"
	edit.ReplyMarkup = &keyboard

	_, err := hr.api.Send(edit)
	return err
}

// handleBuyPresetSettings обрабатывает нажатие кнопки preset для настроек покупки
func (hr *HandlerRegistry) handleBuyPresetSettings(ctx context.Context, query *tgbotapi.CallbackQuery) error {
	userID := query.From.ID

	// Parse amount from callback data: buy_preset_0.05 or buy_preset:0.05
	var amountStr string
	if strings.Contains(query.Data, "_") {
		// New format: buy_preset_0.05
		parts := strings.Split(query.Data, "_")
		if len(parts) != 3 {
			hr.logger.Printf("Invalid buy_preset callback format: %s", query.Data)
			return nil
		}
		amountStr = parts[2]
	} else {
		// Old format: buy_preset:0.05
		parts := strings.Split(query.Data, ":")
		if len(parts) != 2 {
			hr.logger.Printf("Invalid buy_preset callback format: %s", query.Data)
			return nil
		}
		amountStr = parts[1]
	}

	hr.logger.Printf("Setting buy preset %s for user %d", amountStr, userID)

	// Parse amount as float
	amount, err := strconv.ParseFloat(amountStr, 64)
	if err != nil {
		hr.logger.Printf("Invalid buy preset amount: %s", amountStr)
		return hr.editMessageWithError(ctx, query, "Invalid preset amount")
	}

	// Update default SOL amount
	err = hr.services.GetSettingsService().UpdateDefaultSOLAmount(ctx, userID, amount)
	if err != nil {
		hr.logger.Printf("Failed to update default SOL amount for user %d: %v", userID, err)
		return hr.editMessageWithError(ctx, query, "Failed to update default amount")
	}

	// Show success message instead of redirecting back
	text := fmt.Sprintf(`✅ <b>Buy Amount Updated</b>

New default buy amount: <code>%.3f SOL</code>

This amount will be used as default for buying tokens.`, amount)

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("⬅️ Back to Buy Settings", "settings_buy"),
			tgbotapi.NewInlineKeyboardButtonData("🏠 Main Menu", "menu"),
		),
	)

	edit := tgbotapi.NewEditMessageText(query.Message.Chat.ID, query.Message.MessageID, text)
	edit.ParseMode = "HTML"
	edit.ReplyMarkup = &keyboard

	_, err = hr.api.Send(edit)
	return err
}

// handleSellPresetSettings обрабатывает нажатие кнопки preset для настроек продажи
func (hr *HandlerRegistry) handleSellPresetSettings(ctx context.Context, query *tgbotapi.CallbackQuery) error {
	userID := query.From.ID

	// Parse percentage from callback data: sell_preset_25 or sell_preset:25
	var percentStr string
	if strings.Contains(query.Data, "_") {
		// New format: sell_preset_25
		parts := strings.Split(query.Data, "_")
		if len(parts) != 3 {
			hr.logger.Printf("Invalid sell_preset callback format: %s", query.Data)
			return nil
		}
		percentStr = parts[2]
	} else {
		// Old format: sell_preset:25
		parts := strings.Split(query.Data, ":")
		if len(parts) != 2 {
			hr.logger.Printf("Invalid sell_preset callback format: %s", query.Data)
			return nil
		}
		percentStr = parts[1]
	}

	hr.logger.Printf("Setting sell preset %s%% for user %d", percentStr, userID)

	// Parse percentage
	percent, err := strconv.ParseFloat(percentStr, 64)
	if err != nil {
		hr.logger.Printf("Invalid sell preset percentage: %s", percentStr)
		return hr.editMessageWithError(ctx, query, "Invalid preset percentage")
	}

	// Update sell presets - for simplicity, update the specific one
	var preset25, preset50, preset75, preset100 *float64
	switch percentStr {
	case "25":
		preset25 = &percent
	case "50":
		preset50 = &percent
	case "75":
		preset75 = &percent
	case "100":
		preset100 = &percent
	}

	err = hr.services.GetSettingsService().UpdateSellPresets(ctx, userID, preset25, preset50, preset75, preset100)
	if err != nil {
		hr.logger.Printf("Failed to update sell presets for user %d: %v", userID, err)
		return hr.editMessageWithError(ctx, query, "Failed to update sell preset")
	}

	// Show success message instead of redirecting back
	text := fmt.Sprintf(`✅ <b>Sell Preset Updated</b>

Preset %s%% has been configured.

This preset will be available for quick selling.`, percentStr)

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("⬅️ Back to Sell Settings", "settings_sell"),
			tgbotapi.NewInlineKeyboardButtonData("🏠 Main Menu", "menu"),
		),
	)

	edit := tgbotapi.NewEditMessageText(query.Message.Chat.ID, query.Message.MessageID, text)
	edit.ParseMode = "HTML"
	edit.ReplyMarkup = &keyboard

	_, err = hr.api.Send(edit)
	return err
}

// handlePriorityPresetSettings обрабатывает нажатие кнопки preset для настроек priority fee
func (hr *HandlerRegistry) handlePriorityPresetSettings(ctx context.Context, query *tgbotapi.CallbackQuery) error {
	userID := query.From.ID

	// Parse SOL amount from callback data: priority_preset_0.001 or priority_preset:0.001
	var solAmountStr string
	if strings.Contains(query.Data, "_") {
		// New format: priority_preset_0.001
		parts := strings.Split(query.Data, "_")
		if len(parts) != 3 {
			hr.logger.Printf("Invalid priority_preset callback format: %s", query.Data)
			return nil
		}
		solAmountStr = parts[2]
	} else {
		// Old format: priority_preset:0.001
		parts := strings.Split(query.Data, ":")
		if len(parts) != 2 {
			hr.logger.Printf("Invalid priority_preset callback format: %s", query.Data)
			return nil
		}
		solAmountStr = parts[1]
	}

	hr.logger.Printf("Setting priority fee preset %s SOL for user %d", solAmountStr, userID)

	// Parse SOL amount
	solAmount, err := strconv.ParseFloat(solAmountStr, 64)
	if err != nil {
		hr.logger.Printf("Invalid priority fee preset SOL amount: %s", solAmountStr)
		return hr.editMessageWithError(ctx, query, "Invalid preset SOL amount")
	}

	// Convert SOL to lamports (1 SOL = 1e9 lamports)
	lamports := int64(solAmount * 1e9)

	// Get current settings to preserve Jito tip
	settings, err := hr.services.GetSettingsService().GetUserSettings(ctx, userID)
	if err != nil {
		hr.logger.Printf("Failed to get settings for user %d: %v", userID, err)
		return hr.editMessageWithError(ctx, query, "Failed to load settings")
	}

	// Update priority fee while keeping jito tip
	err = hr.services.GetSettingsService().UpdateFees(ctx, userID, lamports, settings.JitoTip)
	if err != nil {
		hr.logger.Printf("Failed to update priority fee for user %d: %v", userID, err)
		return hr.editMessageWithError(ctx, query, "Failed to update priority fee")
	}

	// Show success message instead of redirecting back
	text := fmt.Sprintf(`✅ <b>Priority Fee Updated</b>

New priority fee: <code>%.6f SOL</code>

This fee will be used for transaction priority.`, solAmount)

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("⬅️ Back to Priority Settings", "settings_priority"),
			tgbotapi.NewInlineKeyboardButtonData("🏠 Main Menu", "menu"),
		),
	)

	edit := tgbotapi.NewEditMessageText(query.Message.Chat.ID, query.Message.MessageID, text)
	edit.ParseMode = "HTML"
	edit.ReplyMarkup = &keyboard

	_, err = hr.api.Send(edit)
	return err
}

// handleSellPresetEditFlow запускает flow для редактирования конкретного sell preset
func (hr *HandlerRegistry) handleSellPresetEditFlow(ctx context.Context, query *tgbotapi.CallbackQuery) error {
	userID := query.From.ID

	// Parse preset percentage from callback data: sell_preset_edit:25
	parts := strings.Split(query.Data, ":")
	if len(parts) != 2 {
		hr.logger.Printf("Invalid sell_preset_edit callback format: %s", query.Data)
		return nil
	}

	presetPercent := parts[1]
	hr.logger.Printf("Starting sell preset %s%% edit flow for user %d", presetPercent, userID)

	// Map percentage to index (25%=1, 50%=2, 75%=3, 100%=4)
	var presetIndex int
	switch presetPercent {
	case "25":
		presetIndex = 1
	case "50":
		presetIndex = 2
	case "75":
		presetIndex = 3
	case "100":
		presetIndex = 4
	default:
		hr.logger.Printf("Invalid sell preset percentage: %s", presetPercent)
		return nil
	}

	// Store preset info in session data
	sessionData := map[string]interface{}{
		"preset_percent": presetPercent,
		"preset_index":   presetIndex,
	}

	// Set flow for text input
	hr.sessions.SetFlow(userID, sessions.FlowSellPresetEdit, sessionData)

	text := fmt.Sprintf(`💸 <b>Edit %s%% Sell Preset</b>

Please send the new percentage for the %s%% sell preset.

<b>Examples:</b>
• <code>25</code> - for 25%%
• <code>33</code> - for 33%%
• <code>50</code> - for 50%%
• <code>100</code> - for 100%%

💡 <b>Range:</b> 1 to 100 percent
🎯 <b>Tips:</b> Choose percentages you use frequently for selling

Send the percentage for %s%% preset now:`, presetPercent, presetPercent, presetPercent)

	keyboard := hr.settingsHandler.uiBuilder.BuildBackMenuKeyboard()

	edit := tgbotapi.NewEditMessageText(query.Message.Chat.ID, query.Message.MessageID, text)
	edit.ParseMode = "HTML"
	edit.ReplyMarkup = &keyboard

	_, err := hr.api.Send(edit)
	return err
}

// handleTokenPurchase handles token purchase with specified amount
func (hr *HandlerRegistry) handleTokenPurchase(ctx context.Context, query *tgbotapi.CallbackQuery) error {
	userID := query.From.ID

	// Parse callback data: buy_token_{address}_{amount}
	parts := strings.Split(query.Data, "_")
	if len(parts) < 4 {
		hr.logger.Printf("Invalid token purchase callback format: %s", query.Data)
		return nil
	}

	tokenAddress := parts[2]
	amountStr := parts[3]

	hr.logger.Printf("Token purchase request: %s SOL of %s for user %d", amountStr, tokenAddress, userID)

	// TODO: Implement actual token purchase logic
	// For now, show a placeholder message
	text := fmt.Sprintf(`🚧 <b>Token Purchase</b>

Token: <code>%s</code>
Amount: <code>%s SOL</code>

⚠️ Token purchase functionality is in development.

Please check back later for trading features.`, tokenAddress, amountStr)

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔄 Refresh Token", fmt.Sprintf("refresh_token_%s", tokenAddress)),
			tgbotapi.NewInlineKeyboardButtonData("🔙 Back", "menu"),
		),
	)

	edit := tgbotapi.NewEditMessageText(query.Message.Chat.ID, query.Message.MessageID, text)
	edit.ParseMode = "HTML"
	edit.ReplyMarkup = &keyboard

	_, err := hr.api.Send(edit)
	return err
}

// handleEditCustomAmount handles custom amount editing for token purchase
func (hr *HandlerRegistry) handleEditCustomAmount(ctx context.Context, query *tgbotapi.CallbackQuery) error {
	userID := query.From.ID

	// Parse callback data: edit_custom_amount_{address}
	parts := strings.Split(query.Data, "_")
	if len(parts) < 4 {
		hr.logger.Printf("Invalid edit custom amount callback format: %s", query.Data)
		return nil
	}

	tokenAddress := parts[3]

	hr.logger.Printf("Starting custom amount edit flow for token %s, user %d", tokenAddress, userID)

	// Store message ID and chat ID in session so we can edit it later
	sessionData := map[string]interface{}{
		"token_address":   tokenAddress,
		"edit_message_id": query.Message.MessageID,
		"edit_chat_id":    query.Message.Chat.ID,
	}

	// Set flow for text input
	hr.sessions.SetFlow(userID, sessions.FlowTokenBuy, sessionData)

	text := `💰 <b>Edit Custom Amount</b>

Please send the new SOL amount for token purchases.

<b>Examples:</b>
• <code>0.1</code> - for 0.1 SOL
• <code>0.5</code> - for 0.5 SOL
• <code>1.0</code> - for 1.0 SOL
• <code>2.5</code> - for 2.5 SOL

💡 <b>Range:</b> 0.001 to 1000 SOL
🎯 <b>Tips:</b> This will be your default amount for quick purchases

Send the SOL amount now:`

	keyboard := hr.settingsHandler.uiBuilder.BuildBackMenuKeyboard()

	edit := tgbotapi.NewEditMessageText(query.Message.Chat.ID, query.Message.MessageID, text)
	edit.ParseMode = "HTML"
	edit.ReplyMarkup = &keyboard

	_, err := hr.api.Send(edit)
	return err
}

// handleEditSlippage handles slippage editing for token purchase
func (hr *HandlerRegistry) handleEditSlippage(ctx context.Context, query *tgbotapi.CallbackQuery) error {
	userID := query.From.ID

	// Handle both formats: edit_slippage and edit_slippage_{address}
	var tokenAddress string
	parts := strings.Split(query.Data, "_")

	if len(parts) >= 3 {
		// Old format: edit_slippage_{address}
		tokenAddress = parts[2]
		hr.logger.Printf("Starting slippage edit flow for token %s, user %d (old format)", tokenAddress, userID)
	} else {
		// New format: edit_slippage (get token from session)
		var ok bool
		tokenAddress, ok = hr.sessions.GetData(userID, "token_address").(string)
		if !ok || tokenAddress == "" {
			hr.logger.Printf("No token address found in session for user %d", userID)
			return hr.editMessageWithError(ctx, query, "No token address found. Please enter a token address first.")
		}
		hr.logger.Printf("Starting slippage edit flow for token %s, user %d (new format)", tokenAddress, userID)
	}

	// Store message ID and chat ID in session so we can edit it later
	sessionData := map[string]interface{}{
		"token_address":   tokenAddress,
		"edit_message_id": query.Message.MessageID,
		"edit_chat_id":    query.Message.Chat.ID,
	}

	// Set flow for text input
	hr.sessions.SetFlow(userID, sessions.FlowBuySlippageEdit, sessionData)

	text := `🎛️ <b>Edit Buy Slippage</b>

Please send the new slippage percentage for token purchases.

<b>Examples:</b>
• <code>1</code> - for 1%
• <code>5</code> - for 5%
• <code>10</code> - for 10%
• <code>15</code> - for 15%

💡 <b>Range:</b> 0.1% to 50%
🎯 <b>Tips:</b> Higher slippage = faster execution but worse price

Send the slippage percentage now:`

	keyboard := hr.settingsHandler.uiBuilder.BuildBackMenuKeyboard()

	edit := tgbotapi.NewEditMessageText(query.Message.Chat.ID, query.Message.MessageID, text)
	edit.ParseMode = "HTML"
	edit.ReplyMarkup = &keyboard

	_, err := hr.api.Send(edit)
	return err
}

// handleRefreshToken handles token data refresh
func (hr *HandlerRegistry) handleRefreshToken(ctx context.Context, query *tgbotapi.CallbackQuery) error {
	userID := query.From.ID

	// Parse callback data: refresh_token_{address}
	parts := strings.Split(query.Data, "_")
	if len(parts) < 3 {
		hr.logger.Printf("Invalid refresh token callback format: %s", query.Data)
		return nil
	}

	tokenAddress := parts[2]

	hr.logger.Printf("Refreshing token data for %s, user %d", tokenAddress, userID)

	// Simulate the user sending the token address again to trigger handleTokenAddress
	// Create a fake message to process through text processor
	fakeMessage := &tgbotapi.Message{
		From: &tgbotapi.User{ID: userID},
		Text: tokenAddress,
		Chat: &tgbotapi.Chat{ID: userID},
	}

	// Process through text processor to get fresh token data
	return hr.textProcessor.ProcessText(ctx, fakeMessage)
}

// handleBuyCustomAmount handles custom amount selection for token purchase
func (hr *HandlerRegistry) handleBuyCustomAmount(ctx context.Context, query *tgbotapi.CallbackQuery) error {
	userID := query.From.ID

	// Parse callback data: buy_custom_{address}
	parts := strings.Split(query.Data, "_")
	if len(parts) < 3 {
		hr.logger.Printf("Invalid buy custom callback format: %s", query.Data)
		return nil
	}

	tokenAddress := parts[2]

	hr.logger.Printf("Custom amount selection for token %s, user %d", tokenAddress, userID)

	// Redirect to edit custom amount handler
	newQuery := *query
	newQuery.Data = fmt.Sprintf("edit_custom_amount_%s", tokenAddress)

	return hr.handleEditCustomAmount(ctx, &newQuery)
}

// handleBuyPreset handles buy preset selection for token purchase
func (hr *HandlerRegistry) handleBuyPreset(ctx context.Context, query *tgbotapi.CallbackQuery, presetIndex int) error {
	userID := query.From.ID

	hr.logger.Printf("Selecting buy preset %d for user %d", presetIndex, userID)

	// Get user settings to load database presets instead of hardcoded
	settings, err := hr.services.GetSettingsService().GetUserSettings(ctx, userID)
	if err != nil {
		hr.logger.Printf("Failed to get user settings for user %d: %v", userID, err)
		return hr.editMessageWithError(ctx, query, "Failed to load user settings")
	}

	// Define preset amounts from database
	presetAmounts := []float64{}
	if settings.BuyPreset1 != nil {
		presetAmounts = append(presetAmounts, *settings.BuyPreset1)
	}
	if settings.BuyPreset2 != nil {
		presetAmounts = append(presetAmounts, *settings.BuyPreset2)
	}
	if settings.BuyPreset3 != nil {
		presetAmounts = append(presetAmounts, *settings.BuyPreset3)
	}
	if settings.BuyPreset4 != nil {
		presetAmounts = append(presetAmounts, *settings.BuyPreset4)
	}
	if settings.BuyPreset5 != nil {
		presetAmounts = append(presetAmounts, *settings.BuyPreset5)
	}

	// Fallback to default presets if no database presets configured
	if len(presetAmounts) == 0 {
		presetAmounts = []float64{0.001, 0.05, 0.1, 0.02, 0.2} // fallback defaults
	}

	if presetIndex < 1 || presetIndex > len(presetAmounts) {
		hr.logger.Printf("Invalid preset index %d", presetIndex)
		return hr.editMessageWithError(ctx, query, "Invalid preset selected")
	}

	selectedAmount := presetAmounts[presetIndex-1]

	// Save selected preset amount in session
	hr.sessions.SetData(userID, "selected_amount", selectedAmount)
	hr.sessions.SetData(userID, "selected_amount_type", "preset")

	hr.logger.Printf("User %d selected preset %d with amount %.3f SOL", userID, presetIndex, selectedAmount)

	// Get token address from session to return to token menu
	tokenAddress, ok := hr.sessions.GetData(userID, "token_address").(string)
	if !ok || tokenAddress == "" {
		hr.logger.Printf("No token address found in session for user %d", userID)
		return hr.editMessageWithError(ctx, query, "No token address found. Please enter a token address first.")
	}

	// Use the new HandleTokenAddressEdit function to edit existing message instead of sending new one
	return hr.textProcessor.HandleTokenAddressEdit(ctx, query, tokenAddress)
}

// handleEditCustomAmountShort handles short custom amount editing
func (hr *HandlerRegistry) handleEditCustomAmountShort(ctx context.Context, query *tgbotapi.CallbackQuery) error {
	userID := query.From.ID

	// Parse callback data: edit_custom_amount
	hr.logger.Printf("Starting custom amount edit flow for user %d", userID)

	// Get token address from session data
	tokenAddress, ok := hr.sessions.GetData(userID, "token_address").(string)
	if !ok || tokenAddress == "" {
		hr.logger.Printf("No token address found in session for user %d", userID)
		return hr.editMessageWithError(ctx, query, "No token address found. Please enter a token address first.")
	}

	// Basic validation of token address to prevent edit context issues
	if len(tokenAddress) < 32 || len(tokenAddress) > 44 {
		hr.logger.Printf("Invalid token address in session for user %d: %s", userID, tokenAddress)
		// Clear invalid token data from session
		hr.sessions.SetData(userID, "token_address", "")
		return hr.editMessageWithError(ctx, query, "Invalid token address in session. Please enter a valid token address first.")
	}

	// Store message ID and chat ID in session so we can edit it later
	sessionData := map[string]interface{}{
		"token_address":   tokenAddress,
		"edit_message_id": query.Message.MessageID,
		"edit_chat_id":    query.Message.Chat.ID,
	}

	// Set flow for text input - use FlowBuyAmountEdit for custom amount editing
	hr.sessions.SetFlow(userID, sessions.FlowBuyAmountEdit, sessionData)

	text := `💰 <b>Edit Custom Amount</b>

Please send the new SOL amount for token purchases.

<b>Examples:</b>
• <code>0.1</code> - for 0.1 SOL
• <code>0.5</code> - for 0.5 SOL
• <code>1.0</code> - for 1.0 SOL
• <code>2.5</code> - for 2.5 SOL

💡 <b>Range:</b> 0.001 to 1000 SOL
🎯 <b>Tips:</b> This will be your default amount for quick purchases

Send the SOL amount now:`

	keyboard := hr.settingsHandler.uiBuilder.BuildBackMenuKeyboard()

	edit := tgbotapi.NewEditMessageText(query.Message.Chat.ID, query.Message.MessageID, text)
	edit.ParseMode = "HTML"
	edit.ReplyMarkup = &keyboard

	_, err := hr.api.Send(edit)
	return err
}

// handleEnterSellToken обрабатывает ввод адреса токена для продажи
func (hr *HandlerRegistry) handleEnterSellToken(ctx context.Context, query *tgbotapi.CallbackQuery) error {
	userID := query.From.ID
	hr.logger.Printf("🔍 ENTER SELL TOKEN: User %d wants to enter token address", userID)

	// Получаем выбранный процент
	selectedPercentage, hasPercentage := hr.sessions.GetData(userID, "selected_sell_percentage").(float64)
	if !hasPercentage {
		selectedPercentage = 50.0 // default
	}

	text := fmt.Sprintf(`📝 <b>Enter Token Address</b>

💸 <b>Sell Percentage:</b> %.0f%%

📋 Please enter the token address you want to sell:

💡 <b>Example:</b>
<code>EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v</code>

⚠️ <b>Note:</b> We will sell %.0f%% of your token balance.

🔙 Send any message to cancel and return to sell menu.`, selectedPercentage, selectedPercentage)

	edit := tgbotapi.NewEditMessageText(query.Message.Chat.ID, query.Message.MessageID, text)
	edit.ParseMode = "HTML"
	_, err := hr.api.Send(edit)
	if err != nil {
		return err
	}

	// Set flow for token address input
	flowData := map[string]interface{}{
		"sell_percentage": selectedPercentage,
	}
	hr.sessions.SetFlow(userID, sessions.FlowTokenSell, flowData)
	return nil
}

// handleConfirmSell обрабатывает подтверждение продажи
func (hr *HandlerRegistry) handleConfirmSell(ctx context.Context, query *tgbotapi.CallbackQuery) error {
	userID := query.From.ID
	hr.logger.Printf("🔍 CONFIRM SELL: User %d wants to confirm sell", userID)

	// Получаем данные из сессии
	tokenAddress, hasToken := hr.sessions.GetData(userID, "token_address").(string)
	selectedPercentage, hasPercentage := hr.sessions.GetData(userID, "selected_sell_percentage").(float64)

	if !hasToken || tokenAddress == "" {
		return hr.editMessageWithError(ctx, query, "Token address not found. Please start over.")
	}

	if !hasPercentage {
		return hr.editMessageWithError(ctx, query, "Sell percentage not found. Please start over.")
	}

	hr.logger.Printf("🚀 CONFIRM SELL: User %d, Token: %s, Percentage: %.0f%%", userID, tokenAddress, selectedPercentage)

	// Показываем процесс исполнения
	processingText := fmt.Sprintf(`⏳ <b>Executing Sell Order</b>

🎯 <b>Token:</b> <code>%s</code>
💸 <b>Percentage:</b> %.0f%%
🔄 <b>Status:</b> Processing transaction...

⚠️ Please wait, this may take 10-30 seconds...`, tokenAddress, selectedPercentage)

	edit := tgbotapi.NewEditMessageText(query.Message.Chat.ID, query.Message.MessageID, processingText)
	edit.ParseMode = "HTML"
	hr.api.Send(edit)

	// Выполняем продажу через executeSimpleSell (РАБОЧАЯ ЛОГИКА!)
	go hr.tradingHandler.executeSimpleSell(context.Background(), userID, query.Message.Chat.ID, query.Message.MessageID, tokenAddress, selectedPercentage)

	return nil
}

func (hr *HandlerRegistry) handleNetworkDiagnostics(ctx context.Context, message *tgbotapi.Message) error {
	hr.logger.Printf("Starting network diagnostics for user %d", message.From.ID)

	// Send initial message
	initialMsg := tgbotapi.NewMessage(message.Chat.ID, "🔍 Запускаем диагностику сети...")
	sentMsg, err := hr.api.Send(initialMsg)
	if err != nil {
		return err
	}

	// Get services
	walletService := hr.services.GetWalletService()

	// Create a test wallet to test basic connectivity
	diagText := "🌐 <b>Диагностика сети Solana</b>\n\n"

	// Test basic network connectivity through a simple balance request
	testCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	// Create a fake wallet ID for testing - try to get any user's wallet for testing
	userID := message.From.ID
	userWallets, walletErr := walletService.GetWallets(testCtx, userID)

	startTime := time.Now()
	if walletErr != nil || len(userWallets) == 0 {
		// No wallet available for testing - this tells us about wallet service status
		duration := time.Since(startTime)
		diagText += "⚠️ <b>Wallet Service тест:</b> НЕТ КОШЕЛЬКОВ\n"
		diagText += fmt.Sprintf("   Нет доступных кошельков для тестирования\n")
		diagText += fmt.Sprintf("   Время проверки: %v\n\n", duration)

		// Add instructions to create wallet first
		diagText += "📝 <b>Действие:</b> Создайте кошелёк через /wallets для полной диагностики\n\n"
	} else {
		// Test with user's first wallet
		testWallet := userWallets[0]
		_, balanceErr := walletService.GetBalance(testCtx, testWallet.ID)
		duration := time.Since(startTime)

		if balanceErr != nil {
			diagText += "❌ <b>Основной RPC тест:</b> НЕУСПЕХ\n"
			diagText += fmt.Sprintf("   Ошибка: %v\n", balanceErr)
			diagText += fmt.Sprintf("   Время ответа: %v\n\n", duration)

			// Check if it's a timeout error
			if strings.Contains(balanceErr.Error(), "context deadline exceeded") {
				diagText += "🚨 <b>Обнаружена проблема с таймаутами!</b>\n"
				diagText += "• RPC серверы не отвечают вовремя\n"
				diagText += "• Возможны проблемы с интернет соединением\n"
				diagText += "• Рекомендуется повторить попытку позже\n\n"
			}

			if strings.Contains(balanceErr.Error(), "wsarecv") {
				diagText += "🔴 <b>Обнаружена проблема Windows сети!</b>\n"
				diagText += "• Проблемы с TCP соединением\n"
				diagText += "• Возможные причины:\n"
				diagText += "  - Нестабильное интернет соединение\n"
				diagText += "  - Блокировка файрволлом\n"
				diagText += "  - Проблемы с провайдером\n\n"
			}
		} else {
			diagText += "✅ <b>Основной RPC тест:</b> УСПЕХ\n"
			diagText += fmt.Sprintf("   Кошелёк: %s...\n", testWallet.PublicKey[:8])
			diagText += fmt.Sprintf("   Время ответа: %v\n\n", duration)
		}
	}

	// Check current time and add timestamp
	diagText += fmt.Sprintf("🕐 <b>Время диагностики:</b> %s\n", time.Now().Format("15:04:05"))

	// Add recommendations based on the logs
	diagText += "\n📋 <b>Рекомендации при ошибках blockhash:</b>\n"
	diagText += "1. Перезапустите бота (помогает при накоплении ошибок)\n"
	diagText += "2. Проверьте интернет соединение\n"
	diagText += "3. Попробуйте через несколько минут\n"
	diagText += "4. При Windows - проверьте антивирус/файрвол\n"

	// Add technical info
	diagText += "\n🔧 <b>Техническая информация:</b>\n"
	diagText += "• Таймаут обновления blockhash: 30 сек\n"
	diagText += "• Интервал обновления: 2 сек\n"
	diagText += "• Используется 4 RPC endpoint\n"
	diagText += "• Параллельные запросы с fallback\n"

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔄 Повторить тест", "netcheck_repeat"),
			tgbotapi.NewInlineKeyboardButtonData("🏠 Главное меню", "menu"),
		),
	)

	edit := tgbotapi.NewEditMessageText(sentMsg.Chat.ID, sentMsg.MessageID, diagText)
	edit.ParseMode = "HTML"
	edit.ReplyMarkup = &keyboard

	_, err = hr.api.Send(edit)
	return err
}

func (hr *HandlerRegistry) handleNetworkDiagnosticsRepeat(ctx context.Context, query *tgbotapi.CallbackQuery) error {
	userID := query.From.ID

	hr.logger.Printf("Starting network diagnostics repeat for user %d", userID)

	// Call the existing handleNetworkDiagnostics function
	return hr.handleNetworkDiagnostics(ctx, query.Message)
}
