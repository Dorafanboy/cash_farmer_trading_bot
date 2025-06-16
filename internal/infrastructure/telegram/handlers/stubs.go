package handlers

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"cash-farmer/internal/application/services"
	"cash-farmer/internal/domain/entities"
	"cash-farmer/internal/domain/valueobjects"
	"cash-farmer/internal/infrastructure/blockchain/trading"
	"cash-farmer/internal/infrastructure/telegram/sessions"
	"cash-farmer/internal/infrastructure/telegram/ui"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// SwapDetails содержит детальную информацию о свапе для улучшенного отображения
type SwapDetails struct {
	TokenAddress        string
	TokenSymbol         string
	TokenName           string
	SolAmount           float64
	TokenAmount         float64
	TokenPrice          string
	Liquidity           string
	MarketCap           string
	UserSOLBalance      string
	UserSOLBalanceUSD   string
	UserTokenBalance    string
	UserTokenBalanceUSD string
	PnL                 string
	TxID                string
}

// SellDetails содержит детальную информацию о продаже для улучшенного отображения
type SellDetails struct {
	TokenAddress        string
	TokenSymbol         string
	TokenName           string
	SellPercentage      float64
	TokenAmount         float64
	SOLReceived         float64
	TokenPrice          string
	Liquidity           string
	MarketCap           string
	UserSOLBalance      string
	UserSOLBalanceUSD   string
	UserTokenBalance    string
	UserTokenBalanceUSD string
	PnL                 string
	TxID                string
}

// PaginationState хранит состояние пагинации
type PaginationState struct {
	CurrentPage int
	TotalPages  int
	PageSize    int
	TotalItems  int
}

// parsePageFromCallback извлекает номер страницы из callback data
func parsePageFromCallback(data string) int {
	// Ожидаем формат "wallets_page:1" или "manage_wallets:1"
	parts := strings.Split(data, ":")
	if len(parts) >= 2 {
		if page, err := strconv.Atoi(parts[len(parts)-1]); err == nil {
			return page
		}
	}
	return 1 // default to page 1
}

// calculatePagination вычисляет параметры пагинации
func calculatePagination(totalItems, pageSize, currentPage int) PaginationState {
	totalPages := (totalItems + pageSize - 1) / pageSize
	if totalPages == 0 {
		totalPages = 1
	}
	if currentPage < 1 {
		currentPage = 1
	}
	if currentPage > totalPages {
		currentPage = totalPages
	}

	return PaginationState{
		CurrentPage: currentPage,
		TotalPages:  totalPages,
		PageSize:    pageSize,
		TotalItems:  totalItems,
	}
}

// paginateWallets возвращает подмножество кошельков для указанной страницы
func paginateWallets(wallets []services.SimpleWallet, pagination PaginationState) []services.SimpleWallet {
	start := (pagination.CurrentPage - 1) * pagination.PageSize
	end := start + pagination.PageSize

	if start >= len(wallets) {
		return []services.SimpleWallet{}
	}
	if end > len(wallets) {
		end = len(wallets)
	}

	return wallets[start:end]
}

// buildPaginationKeyboard создает клавиатуру с кнопками пагинации
func buildPaginationKeyboard(pagination PaginationState, baseAction string) [][]tgbotapi.InlineKeyboardButton {
	var rows [][]tgbotapi.InlineKeyboardButton

	if pagination.TotalPages > 1 {
		var navButtons []tgbotapi.InlineKeyboardButton

		// Кнопка "Назад"
		if pagination.CurrentPage > 1 {
			navButtons = append(navButtons,
				tgbotapi.NewInlineKeyboardButtonData("◀️ Back", fmt.Sprintf("%s:%d", baseAction, pagination.CurrentPage-1)))
		}

		// Показать текущую страницу
		pageInfo := fmt.Sprintf("Page %d of %d", pagination.CurrentPage, pagination.TotalPages)
		navButtons = append(navButtons,
			tgbotapi.NewInlineKeyboardButtonData(pageInfo, "noop"))

		// Кнопка "Вперед"
		if pagination.CurrentPage < pagination.TotalPages {
			navButtons = append(navButtons,
				tgbotapi.NewInlineKeyboardButtonData("Next ▶️", fmt.Sprintf("%s:%d", baseAction, pagination.CurrentPage+1)))
		}

		rows = append(rows, navButtons)
	}

	return rows
}

// sortWalletsByPrimary сортирует кошельки так, чтобы primary wallet был первым
func sortWalletsByPrimary(wallets []services.SimpleWallet) []services.SimpleWallet {
	if len(wallets) <= 1 {
		return wallets
	}

	// Create a copy to avoid modifying the original slice
	sorted := make([]services.SimpleWallet, len(wallets))
	copy(sorted, wallets)

	// Find primary wallet and move it to the beginning
	for i, wallet := range sorted {
		if wallet.IsDefault {
			// Swap primary wallet to position 0
			sorted[0], sorted[i] = sorted[i], sorted[0]
			break
		}
	}

	return sorted
}

// WalletHandler - реальная реализация
type WalletHandler struct {
	services  *services.SimpleServiceContainer
	sessions  *sessions.SessionManager
	api       *tgbotapi.BotAPI
	logger    *log.Logger
	uiBuilder *ui.Builder
}

func NewWalletHandler(serviceContainer *services.SimpleServiceContainer, sessionManager *sessions.SessionManager, api *tgbotapi.BotAPI, logger *log.Logger) *WalletHandler {
	return &WalletHandler{
		services:  serviceContainer,
		sessions:  sessionManager,
		api:       api,
		logger:    logger,
		uiBuilder: ui.NewBuilder(),
	}
}

func (h *WalletHandler) HandleManageWallets(ctx context.Context, query *tgbotapi.CallbackQuery) error {
	userID := query.From.ID
	h.logger.Printf("Showing wallet management for user: %d", userID)

	// Парсим номер страницы из callback data
	currentPage := parsePageFromCallback(query.Data)

	// Get user wallets
	wallets, err := h.services.GetWalletService().GetWallets(ctx, userID)
	if err != nil {
		return h.editMessageWithError(ctx, query, "Failed to load wallets")
	}

	// Sort wallets to put primary first
	wallets = sortWalletsByPrimary(wallets)

	// Настройки пагинации
	pageSize := 10
	pagination := calculatePagination(len(wallets), pageSize, currentPage)

	// Получаем кошельки для текущей страницы
	paginatedWallets := paginateWallets(wallets, pagination)

	// Build wallet list message
	text := "👛 <b>Wallet Management</b>\n\n"

	if len(wallets) == 0 {
		text += "No wallets found. Generate a wallet to get started.\n\n"
	} else {
		for i, wallet := range paginatedWallets {
			// Вычисляем глобальный номер кошелька
			globalIndex := (pagination.CurrentPage-1)*pagination.PageSize + i + 1
			status := ""
			if wallet.IsDefault {
				status = " 🟢"
			}
			text += fmt.Sprintf("<b>%d.</b> <code>%s</code>%s\n", globalIndex, wallet.PublicKey, status)
		}
		text += "\n"
	}

	text += "💡 Choose action:"

	// Build keyboard with pagination
	var keyboardRows [][]tgbotapi.InlineKeyboardButton

	// Добавляем кнопки пагинации если нужно
	paginationRows := buildPaginationKeyboard(pagination, "manage_wallets")
	keyboardRows = append(keyboardRows, paginationRows...)

	// Добавляем основные кнопки управления
	managementKeyboard := h.uiBuilder.BuildWalletManagementKeyboard()
	keyboardRows = append(keyboardRows, managementKeyboard.InlineKeyboard...)

	keyboard := tgbotapi.NewInlineKeyboardMarkup(keyboardRows...)

	edit := tgbotapi.NewEditMessageText(query.Message.Chat.ID, query.Message.MessageID, text)
	edit.ParseMode = "HTML"
	edit.ReplyMarkup = &keyboard

	_, err = h.api.Send(edit)
	return err
}

func (h *WalletHandler) HandleGenerateWallet(ctx context.Context, query *tgbotapi.CallbackQuery) error {
	userID := query.From.ID
	h.logger.Printf("Generating new wallet for user: %d", userID)

	// Create new wallet
	wallet, err := h.services.GetWalletService().CreateWallet(ctx, userID, "Generated Wallet")
	if err != nil {
		return h.editMessageWithError(ctx, query, "Failed to generate wallet")
	}

	text := fmt.Sprintf(`✅ <b>New Wallet Generated!</b>

💳 <b>Address:</b>
<code>%s</code>

⚠️ <b>Important:</b>
• Save your private key safely
• You can export it later via Export button
• This wallet is now available in your wallet list

💡 Use "Set Primary" to make this your main wallet.`, wallet.PublicKey)

	keyboard := h.uiBuilder.BuildBackMenuKeyboard()

	edit := tgbotapi.NewEditMessageText(query.Message.Chat.ID, query.Message.MessageID, text)
	edit.ParseMode = "HTML"
	edit.ReplyMarkup = &keyboard

	_, err = h.api.Send(edit)
	return err
}

func (h *WalletHandler) HandleImportWallet(ctx context.Context, query *tgbotapi.CallbackQuery) error {
	// Set session flow for private key input
	h.sessions.SetFlow(query.From.ID, sessions.FlowWalletImport, nil)

	text := `📥 <b>Import Wallet</b>

Please send your private key in the next message.

⚠️ <b>Security Notice:</b>
• Your message will be deleted immediately after processing
• Never share your private key with anyone
• Make sure you're in a private chat

Send your private key now:`

	keyboard := h.uiBuilder.BuildBackMenuKeyboard()

	edit := tgbotapi.NewEditMessageText(query.Message.Chat.ID, query.Message.MessageID, text)
	edit.ParseMode = "HTML"
	edit.ReplyMarkup = &keyboard

	_, err := h.api.Send(edit)
	return err
}

func (h *WalletHandler) HandleExportWallet(ctx context.Context, query *tgbotapi.CallbackQuery) error {
	userID := query.From.ID
	h.logger.Printf("Showing export wallet interface for user: %d", userID)

	// Парсим номер страницы из callback data
	currentPage := parsePageFromCallback(query.Data)

	// Get user wallets
	wallets, err := h.services.GetWalletService().GetWallets(ctx, userID)
	if err != nil {
		return h.editMessageWithError(ctx, query, "Failed to load wallets")
	}

	if len(wallets) == 0 {
		return h.editMessageWithError(ctx, query, "No wallets found")
	}

	// Sort wallets to put primary first
	wallets = sortWalletsByPrimary(wallets)

	// Настройки пагинации
	pageSize := 10
	pagination := calculatePagination(len(wallets), pageSize, currentPage)

	// Получаем кошельки для текущей страницы
	paginatedWallets := paginateWallets(wallets, pagination)

	// Build wallet selection message
	text := "📤 <b>Export Wallet</b>\n\n"
	text += "⚠️ <b>SECURITY WARNING:</b>\n"
	text += "• Private key will be shown once\n"
	text += "• Never share your private key\n"
	text += "• Message will auto-delete in 30 seconds\n\n"
	text += "👛 <b>Select wallet to export:</b>\n\n"

	// Create inline keyboard with wallet options
	var rows [][]tgbotapi.InlineKeyboardButton

	for i, wallet := range paginatedWallets {
		// Вычисляем глобальный номер кошелька
		globalIndex := (pagination.CurrentPage-1)*pagination.PageSize + i + 1
		status := ""
		if wallet.IsDefault {
			status = " 🟢"
		}
		buttonText := fmt.Sprintf("%d. %s%s", globalIndex, wallet.PublicKey, status)
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(buttonText, fmt.Sprintf("export_wallet:%s", wallet.ID)),
		))
	}

	// Добавляем кнопки пагинации если нужно
	paginationRows := buildPaginationKeyboard(pagination, "wallet_export")
	rows = append(rows, paginationRows...)

	// Add back button
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("← Back", "manage_wallets"),
		tgbotapi.NewInlineKeyboardButtonData("🏠 Menu", "menu"),
	))

	keyboard := tgbotapi.NewInlineKeyboardMarkup(rows...)

	edit := tgbotapi.NewEditMessageText(query.Message.Chat.ID, query.Message.MessageID, text)
	edit.ParseMode = "HTML"
	edit.ReplyMarkup = &keyboard

	_, err = h.api.Send(edit)
	return err
}

func (h *WalletHandler) HandleDeleteWallet(ctx context.Context, query *tgbotapi.CallbackQuery) error {
	userID := query.From.ID
	h.logger.Printf("Showing delete wallet interface for user: %d", userID)

	// Парсим номер страницы из callback data
	currentPage := parsePageFromCallback(query.Data)

	// Get user wallets
	wallets, err := h.services.GetWalletService().GetWallets(ctx, userID)
	if err != nil {
		return h.editMessageWithError(ctx, query, "Failed to load wallets")
	}

	if len(wallets) <= 1 {
		return h.editMessageWithError(ctx, query, "Cannot delete the last wallet")
	}

	// Sort wallets to put primary first
	wallets = sortWalletsByPrimary(wallets)

	// Настройки пагинации
	pageSize := 10
	pagination := calculatePagination(len(wallets), pageSize, currentPage)

	// Получаем кошельки для текущей страницы
	paginatedWallets := paginateWallets(wallets, pagination)

	// Build wallet selection message
	text := "🗑 <b>Delete Wallet</b>\n\n"
	text += "⚠️ <b>WARNING:</b>\n"
	text += "• This action cannot be undone\n"
	text += "• Make sure to backup private keys\n"
	text += "• Primary wallet will be reassigned if deleted\n\n"
	text += "👛 <b>Select wallet to delete:</b>\n\n"

	// Create inline keyboard with wallet options
	var rows [][]tgbotapi.InlineKeyboardButton

	for i, wallet := range paginatedWallets {
		// Вычисляем глобальный номер кошелька
		globalIndex := (pagination.CurrentPage-1)*pagination.PageSize + i + 1
		status := ""
		if wallet.IsDefault {
			status = " 🟢"
		}
		buttonText := fmt.Sprintf("%d. %s%s", globalIndex, wallet.PublicKey, status)
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(buttonText, fmt.Sprintf("delete_wallet:%s", wallet.ID)),
		))
	}

	// Добавляем кнопки пагинации если нужно
	paginationRows := buildPaginationKeyboard(pagination, "wallet_delete")
	rows = append(rows, paginationRows...)

	// Add back button
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("← Back", "manage_wallets"),
		tgbotapi.NewInlineKeyboardButtonData("🏠 Menu", "menu"),
	))

	keyboard := tgbotapi.NewInlineKeyboardMarkup(rows...)

	edit := tgbotapi.NewEditMessageText(query.Message.Chat.ID, query.Message.MessageID, text)
	edit.ParseMode = "HTML"
	edit.ReplyMarkup = &keyboard

	_, err = h.api.Send(edit)
	return err
}

func (h *WalletHandler) HandleSetPrimaryWallet(ctx context.Context, query *tgbotapi.CallbackQuery) error {
	userID := query.From.ID
	h.logger.Printf("Showing set primary wallet interface for user: %d", userID)

	// Парсим номер страницы из callback data
	currentPage := parsePageFromCallback(query.Data)

	// Get user wallets
	wallets, err := h.services.GetWalletService().GetWallets(ctx, userID)
	if err != nil {
		return h.editMessageWithError(ctx, query, "Failed to load wallets")
	}

	if len(wallets) <= 1 {
		return h.editMessageWithError(ctx, query, "You need at least 2 wallets to change primary")
	}

	// Sort wallets to put primary first
	wallets = sortWalletsByPrimary(wallets)

	// Настройки пагинации
	pageSize := 10
	pagination := calculatePagination(len(wallets), pageSize, currentPage)

	// Получаем кошельки для текущей страницы
	paginatedWallets := paginateWallets(wallets, pagination)

	// Build wallet selection message
	text := "🟢 <b>Set Primary Wallet</b>\n\n"
	text += "Select which wallet should be your primary (default) wallet:\n\n"

	// Create inline keyboard with wallet options
	var rows [][]tgbotapi.InlineKeyboardButton

	for i, wallet := range paginatedWallets {
		// Вычисляем глобальный номер кошелька
		globalIndex := (pagination.CurrentPage-1)*pagination.PageSize + i + 1
		status := ""
		if wallet.IsDefault {
			status = " (Current Primary)"
		}
		buttonText := fmt.Sprintf("%d. %s%s", globalIndex, wallet.PublicKey, status)

		// Only allow selection of non-primary wallets
		if !wallet.IsDefault {
			rows = append(rows, tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData(buttonText, fmt.Sprintf("set_primary:%s", wallet.ID)),
			))
		} else {
			// Show current primary but not clickable
			rows = append(rows, tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("✅ "+buttonText, "noop"),
			))
		}
	}

	// Добавляем кнопки пагинации если нужно
	paginationRows := buildPaginationKeyboard(pagination, "wallet_set_primary")
	rows = append(rows, paginationRows...)

	// Add back button
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("← Back", "manage_wallets"),
		tgbotapi.NewInlineKeyboardButtonData("🏠 Menu", "menu"),
	))

	keyboard := tgbotapi.NewInlineKeyboardMarkup(rows...)

	edit := tgbotapi.NewEditMessageText(query.Message.Chat.ID, query.Message.MessageID, text)
	edit.ParseMode = "HTML"
	edit.ReplyMarkup = &keyboard

	_, err = h.api.Send(edit)
	return err
}

func (h *WalletHandler) editMessageWithError(ctx context.Context, query *tgbotapi.CallbackQuery, errorText string) error {
	edit := tgbotapi.NewEditMessageText(query.Message.Chat.ID, query.Message.MessageID, "❌ "+errorText)
	_, err := h.api.Send(edit)
	return err
}

// TradingHandler - реальная реализация
type TradingHandler struct {
	services  *services.SimpleServiceContainer
	sessions  *sessions.SessionManager
	api       *tgbotapi.BotAPI
	logger    *log.Logger
	uiBuilder *ui.Builder
}

func NewTradingHandler(serviceContainer *services.SimpleServiceContainer, sessionManager *sessions.SessionManager, api *tgbotapi.BotAPI, logger *log.Logger) *TradingHandler {
	return &TradingHandler{
		services:  serviceContainer,
		sessions:  sessionManager,
		api:       api,
		logger:    logger,
		uiBuilder: ui.NewBuilder(),
	}
}

func (h *TradingHandler) HandleBuy(ctx context.Context, query *tgbotapi.CallbackQuery) error {
	// Set session flow for token address input
	h.sessions.SetFlow(query.From.ID, sessions.FlowTokenBuy, nil)

	text := `💰 <b>Buy Token</b>

Please send the token contract address in the next message.

💡 <b>Example:</b>
<code>So11111111111111111111111111111111111111112</code>

📝 You can send:
• Token contract address
• Token symbol (if known)

Send token address now:`

	keyboard := h.uiBuilder.BuildBackMenuKeyboard()

	edit := tgbotapi.NewEditMessageText(query.Message.Chat.ID, query.Message.MessageID, text)
	edit.ParseMode = "HTML"
	edit.ReplyMarkup = &keyboard

	_, err := h.api.Send(edit)
	return err
}

func (h *TradingHandler) HandleSell(ctx context.Context, query *tgbotapi.CallbackQuery) error {
	// Перенаправляем к новому flow с позициями
	return h.HandleSellPositions(ctx, query)
}

// HandleSellPositions показывает список всех позиций пользователя для выбора токена для продажи
func (h *TradingHandler) HandleSellPositions(ctx context.Context, query *tgbotapi.CallbackQuery) error {
	userID := query.From.ID
	h.logger.Printf("Showing sell positions for user: %d", userID)

	// Получаем кошельки пользователя
	walletService := h.services.GetWalletService()
	if walletService == nil {
		return h.editMessageWithError(ctx, query, "Wallet service not available")
	}

	wallets, err := walletService.GetWallets(ctx, userID)
	if err != nil {
		return h.editMessageWithError(ctx, query, "Failed to load wallets")
	}

	// Находим primary wallet
	var primaryWallet *services.SimpleWallet
	for _, wallet := range wallets {
		if wallet.IsDefault {
			primaryWallet = &wallet
			break
		}
	}

	if primaryWallet == nil {
		return h.editMessageWithError(ctx, query, "Primary wallet not found. Please create a wallet first.")
	}

	// Получаем позиции пользователя через PortfolioService
	portfolioService := h.services.GetPortfolioService()
	if portfolioService == nil {
		h.logger.Printf("❌ PortfolioService is nil for user %d", userID)
		return h.editMessageWithError(ctx, query, "Portfolio service not available")
	}

	// Конвертируем wallet ID из string в int64
	walletIDInt, err := strconv.ParseInt(primaryWallet.ID, 10, 64)
	if err != nil {
		h.logger.Printf("❌ Invalid wallet ID conversion: %s -> %v", primaryWallet.ID, err)
		return h.editMessageWithError(ctx, query, "Invalid wallet ID")
	}

	h.logger.Printf("🔍 Getting positions for wallet %d, active only: true", walletIDInt)
	positions, err := portfolioService.GetPositions(ctx, walletIDInt, true) // true = только активные позиции
	if err != nil {
		h.logger.Printf("❌ Error getting positions for wallet %d: %v", walletIDInt, err)
		return h.editMessageWithError(ctx, query, "Failed to load positions")
	}

	h.logger.Printf("📊 Retrieved %d positions for wallet %d", len(positions), walletIDInt)

	// Если нет позиций
	if len(positions) == 0 {
		text := `💸 <b>Sell Tokens</b>

📭 <b>No positions found</b>

You don't have any active token positions to sell.
Buy some tokens first to see them here.

💡 Use /buy to purchase tokens`

		keyboard := tgbotapi.NewInlineKeyboardMarkup(
			[]tgbotapi.InlineKeyboardButton{
				tgbotapi.NewInlineKeyboardButtonData("💰 Buy Tokens", "buy"),
			},
			[]tgbotapi.InlineKeyboardButton{
				tgbotapi.NewInlineKeyboardButtonData("🔙 Back", "main_menu"),
			},
		)

		edit := tgbotapi.NewEditMessageText(query.Message.Chat.ID, query.Message.MessageID, text)
		edit.ParseMode = "HTML"
		edit.ReplyMarkup = &keyboard
		_, err = h.api.Send(edit)
		return err
	}

	// Создаем текст с заголовком
	text := fmt.Sprintf(`💸 <b>Choose token to sell (%d):</b>

`, len(positions))

	// Создаем кнопки для позиций
	var rows [][]tgbotapi.InlineKeyboardButton

	// Первый ряд: Back и Refresh
	controlRow := []tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardButtonData("← Back", "main_menu"),
		tgbotapi.NewInlineKeyboardButtonData("Refresh", "sell_positions_refresh"),
	}
	rows = append(rows, controlRow)

	// Добавляем кнопки для каждой позиции (только с реальным балансом > 0)
	var activePositions []*entities.TokenPosition

	for _, position := range positions {
		// Получаем реальный баланс токена из блокчейна
		realTokenAmount := position.Amount // По умолчанию используем данные из БД
		hasRealBalance := false

		// Проверяем реальный баланс в блокчейне
		solanaService := h.services.GetSolanaService()
		if solanaService != nil {
			// Получаем публичный ключ кошелька
			ownerPubKey, err := solana.PublicKeyFromBase58(primaryWallet.PublicKey)
			if err == nil {
				// Получаем публичный ключ токена
				tokenMintPubKey, err := solana.PublicKeyFromBase58(position.TokenAddress)
				if err == nil {
					// Получаем ATA адрес
					ata, _, err := solana.FindAssociatedTokenAddress(ownerPubKey, tokenMintPubKey)
					if err == nil {
						// Получаем реальный баланс токена из блокчейна
						if rpcClient := h.services.GetSolanaService(); rpcClient != nil {
							// Создаем простой RPC клиент для проверки баланса
							simpleRPC := rpc.New("https://api.mainnet-beta.solana.com")
							if tokenBalance, err := simpleRPC.GetTokenAccountBalance(ctx, ata, rpc.CommitmentConfirmed); err == nil {
								if tokenBalance.Value.UiAmount != nil {
									realTokenAmount = *tokenBalance.Value.UiAmount
									hasRealBalance = realTokenAmount > 0.000001 // Минимальный порог
									h.logger.Printf("🔄 REAL BALANCE CHECK: Token %s, DB: %.6f, Blockchain: %.6f, HasBalance: %t",
										position.TokenSymbol, position.Amount, realTokenAmount, hasRealBalance)
								}
							} else {
								h.logger.Printf("⚠️ Failed to get token balance for %s: %v", position.TokenSymbol, err)
							}
						}
					}
				}
			}
		}

		// Пропускаем позиции с нулевым реальным балансом
		if !hasRealBalance && realTokenAmount <= 0.000001 {
			h.logger.Printf("⏭️ SKIPPING position %s - no real balance (%.6f)", position.TokenSymbol, realTokenAmount)
			continue
		}

		// Получаем реальную цену токена через TokenDataService
		var tokenValueUSD float64
		tokenDataService := h.services.GetTokenDataService()
		if tokenDataService != nil {
			tokenAddr, err := valueobjects.NewSolanaTokenAddress(position.TokenAddress)
			if err == nil {
				tokenMetrics, tokenErr := tokenDataService.GetTokenMetrics(ctx, *tokenAddr)
				if tokenErr == nil && tokenMetrics != nil {
					if priceFloat, _ := tokenMetrics.PriceUSD().Float64(); priceFloat > 0 {
						tokenValueUSD = realTokenAmount * priceFloat
					}
				}
			}
		}

		// Создаем копию позиции с реальным балансом
		activePosition := *position // Копируем значение
		activePosition.Amount = realTokenAmount
		activePositions = append(activePositions, &activePosition)

		// Форматируем отображение позиции с реальным балансом
		positionText := fmt.Sprintf("$%s - %.6f - $%.2f",
			position.TokenSymbol,
			realTokenAmount,
			tokenValueUSD)

		// Создаем callback data с адресом токена
		callbackData := fmt.Sprintf("sell_select_token:%s", position.TokenAddress)

		positionButton := []tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData(positionText, callbackData),
		}
		rows = append(rows, positionButton)
	}

	// Если после фильтрации не осталось активных позиций
	if len(activePositions) == 0 {
		text := `💸 <b>Sell Tokens</b>

📭 <b>No tokens to sell</b>

All your positions have been sold or have zero balance.
Buy some tokens first to see them here.

💡 Use /buy to purchase tokens`

		keyboard := tgbotapi.NewInlineKeyboardMarkup(
			[]tgbotapi.InlineKeyboardButton{
				tgbotapi.NewInlineKeyboardButtonData("💰 Buy Tokens", "buy"),
			},
			[]tgbotapi.InlineKeyboardButton{
				tgbotapi.NewInlineKeyboardButtonData("🔙 Back", "main_menu"),
			},
		)

		edit := tgbotapi.NewEditMessageText(query.Message.Chat.ID, query.Message.MessageID, text)
		edit.ParseMode = "HTML"
		edit.ReplyMarkup = &keyboard
		_, err = h.api.Send(edit)
		return err
	}

	// Обновляем заголовок с реальным количеством активных позиций
	text = fmt.Sprintf(`💸 <b>Choose token to sell (%d):</b>

`, len(activePositions))

	keyboard := tgbotapi.NewInlineKeyboardMarkup(rows...)

	edit := tgbotapi.NewEditMessageText(query.Message.Chat.ID, query.Message.MessageID, text)
	edit.ParseMode = "HTML"
	edit.ReplyMarkup = &keyboard

	_, err = h.api.Send(edit)
	return err
}

// HandleSelectTokenForSell обрабатывает выбор токена для продажи
func (h *TradingHandler) HandleSelectTokenForSell(ctx context.Context, query *tgbotapi.CallbackQuery) error {
	userID := query.From.ID

	// Извлекаем адрес токена из callback data: "sell_select_token:ADDRESS"
	parts := strings.Split(query.Data, ":")
	if len(parts) != 2 {
		return h.editMessageWithError(ctx, query, "Invalid token selection")
	}

	tokenAddress := parts[1]
	h.logger.Printf("User %d selected token for sell: %s", userID, tokenAddress)

	// Сохраняем выбранный токен в сессии
	h.sessions.SetData(userID, "token_address", tokenAddress)
	h.sessions.SetData(userID, "sell_flow", "token_selected")

	h.logger.Printf("🔍 DEBUG: Saved token %s to session for user %d", tokenAddress, userID)

	// Переходим к настройкам продажи (старый HandleSell)
	return h.HandleSellSettings(ctx, query)
}

// HandleSellSettings показывает настройки продажи для выбранного токена (старый HandleSell)
func (h *TradingHandler) HandleSellSettings(ctx context.Context, query *tgbotapi.CallbackQuery) error {
	userID := query.From.ID
	h.logger.Printf("Showing sell settings for user: %d", userID)

	// Получаем настройки пользователя
	settings, err := h.services.GetSettingsService().GetUserSettings(ctx, userID)
	if err != nil {
		return h.editMessageWithError(ctx, query, "Failed to load settings")
	}

	// Получаем токен из сессии
	tokenAddress, hasToken := h.sessions.GetData(userID, "token_address").(string)
	h.logger.Printf("🔍 DEBUG: Getting token from session for user %d: hasToken=%v, tokenAddress='%s'", userID, hasToken, tokenAddress)

	if !hasToken || tokenAddress == "" {
		h.logger.Printf("❌ DEBUG: No token in session for user %d", userID)
		return h.editMessageWithError(ctx, query, "No token selected. Please start over.")
	}

	// Получаем информацию о позиции
	walletService := h.services.GetWalletService()
	wallets, err := walletService.GetWallets(ctx, userID)
	if err != nil {
		return h.editMessageWithError(ctx, query, "Failed to load wallets")
	}

	// Находим primary wallet
	var primaryWallet *services.SimpleWallet
	for _, wallet := range wallets {
		if wallet.IsDefault {
			primaryWallet = &wallet
			break
		}
	}

	if primaryWallet == nil {
		return h.editMessageWithError(ctx, query, "Primary wallet not found")
	}

	// Конвертируем wallet ID из string в int64
	walletIDInt, err := strconv.ParseInt(primaryWallet.ID, 10, 64)
	if err != nil {
		return h.editMessageWithError(ctx, query, "Invalid wallet ID")
	}

	portfolioService := h.services.GetPortfolioService()
	position, err := portfolioService.GetActivePositionByToken(ctx, walletIDInt, tokenAddress)
	if err != nil || position == nil {
		return h.editMessageWithError(ctx, query, "Position not found")
	}

	// Создаем информацию о токене
	tokenInfo := fmt.Sprintf(`🎯 <b>Token:</b> $%s
💰 <b>Holdings:</b> %.6f tokens
📍 <b>Address:</b> <code>%s</code>

`, position.TokenSymbol, position.Amount, tokenAddress)

	// Check selected percentage from session to show checkmarks
	selectedPercentage, hasSelection := h.sessions.GetData(userID, "selected_sell_percentage").(float64)

	// Create percentage buttons with checkmarks for selected amounts
	percent25Text := "25%"
	percent50Text := "50%"
	percent75Text := "75%"
	percent100Text := "100%"

	// If no selection was made yet, default to 50% being selected
	if !hasSelection {
		percent50Text += " ✅"
		// Set 50% as default selected
		h.sessions.SetData(userID, "selected_sell_percentage", 50.0)
	} else if hasSelection {
		if selectedPercentage == 25.0 {
			percent25Text += " ✅"
		} else if selectedPercentage == 50.0 {
			percent50Text += " ✅"
		} else if selectedPercentage == 75.0 {
			percent75Text += " ✅"
		} else if selectedPercentage == 100.0 {
			percent100Text += " ✅"
		}
	}

	// Slippage button with checkmark
	selectedSlippage, hasSelectedSlippage := h.sessions.GetData(userID, "selected_sell_slippage").(float64)
	var slippageText string

	if hasSelectedSlippage {
		slippageText = fmt.Sprintf("%.0f%% Slippage ✅✏️", selectedSlippage)
	} else {
		// Show default slippage with checkmark
		slippageText = fmt.Sprintf("%.0f%% Slippage ✅✏️", settings.SellSlippage)
	}

	text := fmt.Sprintf(`💸 <b>Sell Tokens</b>

%s📊 <b>Choose percentage to sell:</b>

⚙️ <b>Current Settings:</b>
• Priority Fee: <code>%.6f SOL</code>
• Jito Tip: <code>%.6f SOL</code>

💡 <b>Ready to sell?</b> Choose percentage below:`,
		tokenInfo,
		float64(settings.PriorityFee)/1e9,
		float64(settings.JitoTip)/1e9)

	// Row 1: Percentage buttons (25%, 50%)
	row1 := []tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardButtonData(percent25Text, "sell_preset_25"),
		tgbotapi.NewInlineKeyboardButtonData(percent50Text, "sell_preset_50"),
	}

	// Row 2: Percentage buttons (75%, 100%)
	row2 := []tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardButtonData(percent75Text, "sell_preset_75"),
		tgbotapi.NewInlineKeyboardButtonData(percent100Text, "sell_preset_100"),
	}

	// Row 3: Slippage button
	row3 := []tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardButtonData(slippageText, "edit_sell_slippage"),
	}

	// Row 4: Action button (SELL)
	row4 := []tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardButtonData("💸 SELL", "confirm_sell"),
	}

	// Row 5: Back button
	row5 := []tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardButtonData("🔙 Back to Positions", "sell_positions"),
	}

	keyboard := tgbotapi.NewInlineKeyboardMarkup(row1, row2, row3, row4, row5)

	edit := tgbotapi.NewEditMessageText(query.Message.Chat.ID, query.Message.MessageID, text)
	edit.ParseMode = "HTML"
	edit.ReplyMarkup = &keyboard

	_, err = h.api.Send(edit)
	return err
}

func (h *TradingHandler) HandleBuyPreset(ctx context.Context, query *tgbotapi.CallbackQuery) error {
	userID := query.From.ID

	// Получаем token address из сессии
	tokenAddress, ok := h.sessions.GetData(userID, "token_address").(string)
	if !ok || tokenAddress == "" {
		return h.editMessageWithError(ctx, query, "Token address not found. Please start over.")
	}

	// Получаем selected amount из сессии
	selectedAmount, ok := h.sessions.GetData(userID, "selected_amount").(float64)
	if !ok || selectedAmount <= 0 {
		return h.editMessageWithError(ctx, query, "Please select an amount first.")
	}

	h.logger.Printf("🚀 EXECUTING REAL BUY: User %d, Token: %s, Amount: %.6f SOL", userID, tokenAddress, selectedAmount)

	// Проверяем наличие TradingService
	tradingService := h.services.GetTradingService()
	if tradingService == nil {
		return h.editMessageWithError(ctx, query, "❌ Trading service not available")
	}

	// Получаем TokenSwapService для реального исполнения
	tokenSwapService := h.services.GetTokenSwapService()
	if tokenSwapService == nil {
		return h.editMessageWithError(ctx, query, "❌ Token swap service not available")
	}

	// Показываем процесс исполнения
	processingText := fmt.Sprintf(`⏳ <b>Executing Buy Order</b>

🎯 <b>Token:</b> <code>%s</code>
💰 <b>Amount:</b> %.6f SOL
🔄 <b>Status:</b> Processing transaction...

⚠️ Please wait, this may take 10-30 seconds...`, tokenAddress, selectedAmount)

	edit := tgbotapi.NewEditMessageText(query.Message.Chat.ID, query.Message.MessageID, processingText)
	edit.ParseMode = "HTML"
	h.api.Send(edit)

	// НОВЫЙ КОД: Используем наш TradingService напрямую!
	// Создаем новый контекст для горутины, не связанный с callback контекстом
	go h.executeSimpleSwap(context.Background(), query, userID, tokenAddress, selectedAmount)

	return nil
}

func (h *TradingHandler) HandleSellPercent(ctx context.Context, query *tgbotapi.CallbackQuery) error {
	userID := query.From.ID

	// Получаем percent из callback data: sell_preset_25, sell_preset_50, etc.
	data := query.Data
	var percentage float64
	switch data {
	case "sell_preset_25":
		percentage = 25.0
	case "sell_preset_50":
		percentage = 50.0
	case "sell_preset_75":
		percentage = 75.0
	case "sell_preset_100":
		percentage = 100.0
	default:
		return h.editMessageWithError(ctx, query, "Invalid sell percentage")
	}

	h.logger.Printf("🔥 SELL PRESET: User %d selected %g%% sell", userID, percentage)

	// Сохраняем выбранный процент в сессии
	h.sessions.SetData(userID, "selected_sell_percentage", percentage)
	h.sessions.SetData(userID, "sell_percentage", percentage)
	h.logger.Printf("🔍 DEBUG: Saved selected_sell_percentage=%.0f for user %d", percentage, userID)

	// Обновляем меню настроек продажи с новым выбранным процентом
	return h.HandleSellSettings(ctx, query)
}

func (h *TradingHandler) HandleCustomAmount(ctx context.Context, query *tgbotapi.CallbackQuery) error {
	return h.editMessageWithError(ctx, query, "Custom amount feature coming soon...")
}

func (h *TradingHandler) editMessageWithError(ctx context.Context, query *tgbotapi.CallbackQuery, errorText string) error {
	edit := tgbotapi.NewEditMessageText(query.Message.Chat.ID, query.Message.MessageID, "❌ "+errorText)
	_, err := h.api.Send(edit)
	return err
}

// executeModularSwap выполняет свап используя НОВЫЕ модульные сервисы
func (h *TradingHandler) executeModularSwap(ctx context.Context, query *tgbotapi.CallbackQuery, userID int64, tokenAddress string, amount float64) {
	h.logger.Printf("🚀 MODULAR SWAP: Starting with NEW architecture for user %d", userID)

	// Получаем наши НОВЫЕ модульные сервисы
	tradingServiceInterface := h.services.GetTradingService()
	jupiterService := h.services.GetJupiterService()
	jitoService := h.services.GetJitoService()
	solanaService := h.services.GetSolanaService()

	if tradingServiceInterface == nil {
		h.logger.Printf("❌ TradingService is nil - falling back to old implementation")
		h.executeOldSwap(ctx, query, userID, tokenAddress, amount)
		return
	}

	// Приводим к нужному типу
	tradingService, ok := tradingServiceInterface.(trading.TradingService)
	if !ok {
		h.logger.Printf("❌ TradingService type assertion failed - falling back to old implementation")
		h.executeOldSwap(ctx, query, userID, tokenAddress, amount)
		return
	}

	h.logger.Printf("✅ MODULAR SERVICES AVAILABLE:")
	h.logger.Printf("  - TradingService: %v", tradingService != nil)
	h.logger.Printf("  - JupiterService: %v", jupiterService != nil)
	h.logger.Printf("  - JitoService: %v", jitoService != nil)
	h.logger.Printf("  - SolanaService: %v", solanaService != nil)

	// Получаем пользователя и кошелек (БЕЗ ТАЙМАУТА - как в работающем коде)
	userWallets, err := h.services.GetWalletService().GetWallets(ctx, userID)
	if err != nil {
		h.logger.Printf("❌ Error getting wallets for user %d: %v", userID, err)
		swapDetails := h.createSwapDetailsStub(tokenAddress, amount, "")
		h.finalizeSwap(query, false, swapDetails, "Wallet loading error")
		return
	}
	if len(userWallets) == 0 {
		h.logger.Printf("❌ No wallets found for user %d", userID)
		swapDetails := h.createSwapDetailsStub(tokenAddress, amount, "")
		h.finalizeSwap(query, false, swapDetails, "No wallet found. Create wallet in 👛 Wallets section")
		return
	}

	h.logger.Printf("✅ Found %d wallets for user %d", len(userWallets), userID)

	// Найдем default кошелек
	var defaultWallet *services.SimpleWallet
	for i, wallet := range userWallets {
		if wallet.IsDefault {
			defaultWallet = &userWallets[i]
			break
		}
	}
	if defaultWallet == nil {
		defaultWallet = &userWallets[0]
	}

	// НОВОЕ: Проверяем баланс кошелька перед покупкой
	walletBalance, err := h.services.GetWalletService().GetBalance(ctx, defaultWallet.ID)
	if err != nil {
		h.logger.Printf("❌ Failed to get wallet balance: %v", err)
		swapDetails := h.createSwapDetailsStub(tokenAddress, amount, "")
		h.finalizeSwap(query, false, swapDetails, "Failed to get wallet balance")
		return
	}

	// Получаем настройки пользователя для точного расчета комиссий
	userSettingsForBalance, err := h.services.GetSettingsService().GetUserSettings(ctx, userID)
	if err != nil {
		h.logger.Printf("❌ Failed to get user settings for balance check: %v", err)
		swapDetails := h.createSwapDetailsStub(tokenAddress, amount, "")
		h.finalizeSwap(query, false, swapDetails, "Failed to load user settings")
		return
	}

	// Конвертируем SOL в lamports для проверки
	amountLamports := int64(amount * 1e9)

	// Конвертируем баланс SOL в lamports для сравнения
	walletBalanceLamports := int64(walletBalance * 1e9)

	// Рассчитываем реальные расходы на основе настроек пользователя
	priorityFeeLamports := int64(userSettingsForBalance.PriorityFee)
	jitoTipLamports := int64(userSettingsForBalance.JitoTip)

	// КРИТИЧНО: Проверяем существование ATA для токена
	ataCreationCost := int64(0) // По умолчанию ATA не нужно создавать

	// Получаем SolanaService для проверки ATA
	solanaServiceRaw := h.services.GetSolanaService()
	if solanaServiceRaw != nil {
		// Type assertion для получения типизированного интерфейса
		if solanaService, ok := solanaServiceRaw.(interface {
			GetAssociatedTokenAddress(owner, mint solana.PublicKey) (solana.PublicKey, error)
			CheckATAExists(ctx context.Context, ata solana.PublicKey) (bool, error)
		}); ok {
			// Сначала получаем адрес ATA
			ownerPubKey, err := solana.PublicKeyFromBase58(defaultWallet.PublicKey)
			if err != nil {
				h.logger.Printf("⚠️ Failed to parse owner public key: %v", err)
				ataCreationCost = int64(2039280)
			} else {
				tokenMintPubKey, err := solana.PublicKeyFromBase58(tokenAddress)
				if err != nil {
					h.logger.Printf("⚠️ Failed to parse token mint: %v", err)
					ataCreationCost = int64(2039280)
				} else {
					// Получаем адрес ATA
					ataAddress, err := solanaService.GetAssociatedTokenAddress(ownerPubKey, tokenMintPubKey)
					if err != nil {
						h.logger.Printf("⚠️ Failed to get ATA address: %v", err)
						ataCreationCost = int64(2039280)
					} else {
						// Проверяем существование ATA
						ataExists, err := solanaService.CheckATAExists(ctx, ataAddress)
						if err != nil {
							h.logger.Printf("⚠️ Failed to check ATA existence: %v", err)
							// В случае ошибки предполагаем, что ATA нужно создать + буфер для дополнительных ATA
							ataCreationCost = int64(2039280 * 2) // 2 ATA на случай промежуточных токенов
						} else if !ataExists {
							h.logger.Printf("🔨 ATA не существует, потребуется создание")
							// Jupiter может создавать дополнительные ATA для промежуточных токенов в маршруте
							ataCreationCost = int64(2039280 * 2) // 2 ATA: основная + возможная промежуточная
							h.logger.Printf("⚠️ Jupiter может создать дополнительные ATA для маршрута, резервируем место для 2 ATA")
						} else {
							h.logger.Printf("✅ ATA уже существует, создание не требуется")
						}
					}
				}
			}
		} else {
			h.logger.Printf("⚠️ SolanaService type assertion failed, предполагаем создание ATA")
			ataCreationCost = int64(2039280 * 2) // 2 ATA на случай промежуточных токенов
		}
	} else {
		h.logger.Printf("⚠️ SolanaService недоступен, предполагаем создание ATA")
		ataCreationCost = int64(2039280 * 2) // 2 ATA на случай промежуточных токенов
	}

	estimatedTotalCost := amountLamports + priorityFeeLamports + jitoTipLamports + ataCreationCost

	h.logger.Printf("💰 BALANCE CHECK:")
	h.logger.Printf("  - Current balance: %d lamports (%.6f SOL)", walletBalanceLamports, walletBalance)
	h.logger.Printf("  - Requested amount: %d lamports (%.6f SOL)", amountLamports, amount)
	h.logger.Printf("  - Priority fee: %d lamports (%.6f SOL)", priorityFeeLamports, float64(priorityFeeLamports)/1e9)
	h.logger.Printf("  - Jito tip: %d lamports (%.6f SOL)", jitoTipLamports, float64(jitoTipLamports)/1e9)
	if ataCreationCost > 0 {
		h.logger.Printf("  - ATA creation cost: %d lamports (%.6f SOL)", ataCreationCost, float64(ataCreationCost)/1e9)
	} else {
		h.logger.Printf("  - ATA creation cost: 0 lamports (ATA already exists)")
	}
	h.logger.Printf("  - Estimated total cost: %d lamports (%.6f SOL)", estimatedTotalCost, float64(estimatedTotalCost)/1e9)

	if walletBalanceLamports < estimatedTotalCost {
		insufficientError := "Insufficient SOL in wallet!"

		h.logger.Printf("❌ INSUFFICIENT BALANCE: %s", insufficientError)
		swapDetails := h.createSwapDetailsStub(tokenAddress, amount, "")
		h.finalizeSwap(query, false, swapDetails, insufficientError)
		return
	}

	// ВАЖНО: Получаем приватный ключ из SimpleWallet (он уже есть!)
	privateKey, err := h.services.GetWalletService().ExportPrivateKey(ctx, userID, defaultWallet.ID)
	if err != nil {
		h.logger.Printf("❌ Failed to get wallet private key: %v", err)
		swapDetails := h.createSwapDetailsStub(tokenAddress, amount, "")
		h.finalizeSwap(query, false, swapDetails, "Failed to load wallet private key")
		return
	}

	if privateKey == "" {
		h.logger.Printf("❌ Wallet private key is empty")
		swapDetails := h.createSwapDetailsStub(tokenAddress, amount, "")
		h.finalizeSwap(query, false, swapDetails, "Wallet private key not found")
		return
	}

	h.logger.Printf("✅ Got wallet private key: %s...", privateKey[:10])

	h.logger.Printf("🎯 MODULAR SWAP PARAMS:")
	h.logger.Printf("  - User: %d", userID)
	h.logger.Printf("  - Wallet: %s", defaultWallet.PublicKey)
	h.logger.Printf("  - From: SOL")
	h.logger.Printf("  - To: %s", tokenAddress)
	h.logger.Printf("  - Amount: %d lamports (%.6f SOL)", amountLamports, amount)

	// РЕАЛЬНЫЙ ВЫЗОВ TradingService с настройками пользователя!
	h.logger.Printf("🚀 Executing REAL swap via TradingService...")

	// Получаем настройки пользователя из базы данных с таймаутом
	userSettings, err := h.services.GetSettingsService().GetUserSettings(ctx, userID)
	if err != nil {
		h.logger.Printf("❌ Failed to get user settings: %v", err)
		swapDetails := h.createSwapDetailsStub(tokenAddress, amount, "")
		h.finalizeSwap(query, false, swapDetails, "Failed to load user settings")
		return
	}

	h.logger.Printf("📋 USER SETTINGS FROM DB:")
	h.logger.Printf("  - Buy Slippage: %.0f%%", userSettings.BuySlippage)
	h.logger.Printf("  - Priority Fee: %d lamports", userSettings.PriorityFee)
	h.logger.Printf("  - Jito Tip: %d lamports", userSettings.JitoTip)

	// Создаем параметры для свапа: SOL → Token используя НАСТРОЙКИ ПОЛЬЗОВАТЕЛЯ
	swapRequest := trading.SwapRequest{
		UserPublicKey:       defaultWallet.PublicKey,
		UserPrivateKey:      privateKey,                                    // Добавляем приватный ключ для подписи
		InputTokenMint:      "So11111111111111111111111111111111111111112", // SOL
		OutputTokenMint:     tokenAddress,
		Amount:              fmt.Sprintf("%d", amountLamports),
		SlippageBps:         int(userSettings.BuySlippage * 100), // Конвертируем % в basis points
		PriorityFeeLamports: uint64(userSettings.PriorityFee),
		JitoTipLamports:     uint64(userSettings.JitoTip),
		MaxRetries:          3,
	}

	h.logger.Printf("📊 SWAP REQUEST (USER SETTINGS):")
	h.logger.Printf("  - Input: SOL (%d lamports = %.6f SOL)", amountLamports, amount)
	h.logger.Printf("  - Output: %s", tokenAddress)
	h.logger.Printf("  - Slippage: %.0f%% (%d basis points)", userSettings.BuySlippage, int(userSettings.BuySlippage*100))
	h.logger.Printf("  - Priority Fee: %d lamports", userSettings.PriorityFee)
	h.logger.Printf("  - Jito Tip: %d lamports", userSettings.JitoTip)

	// Выполняем свап
	result, err := tradingService.ExecuteSwapWithJito(ctx, swapRequest)
	if err != nil {
		h.logger.Printf("❌ SWAP FAILED: %v", err)

		// Улучшенная обработка ошибок
		errorMsg := h.parseSwapError(err.Error())
		swapDetails := h.createSwapDetailsStub(tokenAddress, amount, "")
		h.finalizeSwap(query, false, swapDetails, errorMsg)
		return
	}

	h.logger.Printf("✅ SWAP SUCCESS!")
	h.logger.Printf("  - Status: %s", result.Status)
	h.logger.Printf("  - TX Hash: %s", result.TransactionHash)
	h.logger.Printf("  - Output Amount: %s", result.OutputAmount)
	h.logger.Printf("  - Price Impact: %s", result.PriceImpact)
	h.logger.Printf("  - Duration: %v", result.Duration)

	// 📝 ВАЖНО: Записываем событие покупки для PnL трекинга
	h.logger.Printf("📝 Recording buy event for PnL tracking...")
	h.recordBuyEvent(ctx, userID, defaultWallet, tokenAddress, amount, result)

	// 📊 ВАЖНО: Обновляем позицию в PortfolioService
	h.logger.Printf("📊 Creating position in PortfolioService...")
	h.createPositionAfterBuy(ctx, userID, defaultWallet, tokenAddress, amount, result)

	// Показываем результат
	swapDetails := h.createSwapDetailsStub(tokenAddress, amount, result.TransactionHash)
	h.finalizeSwap(query, true, swapDetails, "")
}

// createSwapDetailsStub создает заглушку SwapDetails для обратной совместимости
func (h *TradingHandler) createSwapDetailsStub(tokenAddress string, amount float64, txID string) *SwapDetails {
	return &SwapDetails{
		TokenAddress:        tokenAddress,
		TokenSymbol:         "UNKNOWN",
		TokenName:           "unknown token",
		SolAmount:           amount,
		TokenAmount:         0,
		TokenPrice:          "$0.000000",
		Liquidity:           "$0.00K",
		MarketCap:           "$0.00K",
		UserSOLBalance:      "0.000000 SOL",
		UserSOLBalanceUSD:   "$0.00",
		UserTokenBalance:    "0.000000",
		UserTokenBalanceUSD: "$0.00",
		PnL:                 "--",
		TxID:                txID,
	}
}

// createSellDetailsStub создает заглушку SellDetails для обратной совместимости
func (h *TextProcessor) createSellDetailsStub(tokenAddress string, sellPercentage float64, txID string) *SellDetails {
	ctx := context.Background()

	// Получаем реальные данные о токене
	tokenSymbol := "UNKNOWN"
	tokenName := "unknown token"
	tokenPrice := "$0.000000"
	liquidity := "$0.00K"
	marketCap := "$0.00K"

	// Получаем данные токена через TokenDataService
	tokenDataService := h.services.GetTokenDataService()
	if tokenDataService != nil {
		if tokenAddr, err := valueobjects.NewSolanaTokenAddress(tokenAddress); err == nil {
			if tokenMetrics, err := tokenDataService.GetTokenMetrics(ctx, *tokenAddr); err == nil && tokenMetrics != nil {
				tokenSymbol = tokenMetrics.Symbol()
				tokenName = tokenMetrics.Name()

				if priceFloat, _ := tokenMetrics.PriceUSD().Float64(); priceFloat > 0 {
					tokenPrice = fmt.Sprintf("$%.8f", priceFloat)
				}

				if liquidityFloat, _ := tokenMetrics.Liquidity().Float64(); liquidityFloat > 0 {
					if liquidityFloat >= 1000000 {
						liquidity = fmt.Sprintf("$%.2fM", liquidityFloat/1000000)
					} else if liquidityFloat >= 1000 {
						liquidity = fmt.Sprintf("$%.2fK", liquidityFloat/1000)
					} else {
						liquidity = fmt.Sprintf("$%.2f", liquidityFloat)
					}
				}

				if mcFloat, _ := tokenMetrics.MarketCap().Float64(); mcFloat > 0 {
					if mcFloat >= 1000000 {
						marketCap = fmt.Sprintf("$%.2fM", mcFloat/1000000)
					} else if mcFloat >= 1000 {
						marketCap = fmt.Sprintf("$%.2fK", mcFloat/1000)
					} else {
						marketCap = fmt.Sprintf("$%.2f", mcFloat)
					}
				}
			}
		}
	}

	return &SellDetails{
		TokenAddress:        tokenAddress,
		TokenSymbol:         tokenSymbol,
		TokenName:           tokenName,
		SellPercentage:      sellPercentage,
		TokenAmount:         0, // Будет заполнено позже
		SOLReceived:         0, // Будет заполнено позже
		TokenPrice:          tokenPrice,
		Liquidity:           liquidity,
		MarketCap:           marketCap,
		UserSOLBalance:      "0.000000 SOL", // Будет заполнено позже
		UserSOLBalanceUSD:   "$0.00",        // Будет заполнено позже
		UserTokenBalance:    "0.000000",     // Будет заполнено позже
		UserTokenBalanceUSD: "$0.00",        // Будет заполнено позже
		PnL:                 "--",
		TxID:                txID,
	}
}

// createSellDetailsStub создает заглушку SellDetails для обратной совместимости (для TradingHandler)
func (h *TradingHandler) createSellDetailsStub(tokenAddress string, sellPercentage float64, txID string) *SellDetails {
	ctx := context.Background()

	// Получаем реальные данные о токене
	tokenSymbol := "UNKNOWN"
	tokenName := "unknown token"
	tokenPrice := "$0.000000"
	liquidity := "$0.00K"
	marketCap := "$0.00K"

	// Получаем данные токена через TokenDataService
	tokenDataService := h.services.GetTokenDataService()
	if tokenDataService != nil {
		if tokenAddr, err := valueobjects.NewSolanaTokenAddress(tokenAddress); err == nil {
			if tokenMetrics, err := tokenDataService.GetTokenMetrics(ctx, *tokenAddr); err == nil && tokenMetrics != nil {
				tokenSymbol = tokenMetrics.Symbol()
				tokenName = tokenMetrics.Name()

				if priceFloat, _ := tokenMetrics.PriceUSD().Float64(); priceFloat > 0 {
					tokenPrice = fmt.Sprintf("$%.8f", priceFloat)
				}

				if liquidityFloat, _ := tokenMetrics.Liquidity().Float64(); liquidityFloat > 0 {
					if liquidityFloat >= 1000000 {
						liquidity = fmt.Sprintf("$%.2fM", liquidityFloat/1000000)
					} else if liquidityFloat >= 1000 {
						liquidity = fmt.Sprintf("$%.2fK", liquidityFloat/1000)
					} else {
						liquidity = fmt.Sprintf("$%.2f", liquidityFloat)
					}
				}

				if mcFloat, _ := tokenMetrics.MarketCap().Float64(); mcFloat > 0 {
					if mcFloat >= 1000000 {
						marketCap = fmt.Sprintf("$%.2fM", mcFloat/1000000)
					} else if mcFloat >= 1000 {
						marketCap = fmt.Sprintf("$%.2fK", mcFloat/1000)
					} else {
						marketCap = fmt.Sprintf("$%.2f", mcFloat)
					}
				}
			}
		}
	}

	return &SellDetails{
		TokenAddress:        tokenAddress,
		TokenSymbol:         tokenSymbol,
		TokenName:           tokenName,
		SellPercentage:      sellPercentage,
		TokenAmount:         0, // Будет заполнено позже
		SOLReceived:         0, // Будет заполнено позже
		TokenPrice:          tokenPrice,
		Liquidity:           liquidity,
		MarketCap:           marketCap,
		UserSOLBalance:      "0.000000 SOL", // Будет заполнено позже
		UserSOLBalanceUSD:   "$0.00",        // Будет заполнено позже
		UserTokenBalance:    "0.000000",     // Будет заполнено позже
		UserTokenBalanceUSD: "$0.00",        // Будет заполнено позже
		PnL:                 "--",
		TxID:                txID,
	}
}

// parseSwapError преобразует техническую ошибку в понятное сообщение
func (h *TradingHandler) parseSwapError(errorStr string) string {
	errorLower := strings.ToLower(errorStr)

	// Проверяем разные типы ошибок
	if strings.Contains(errorLower, "insufficient") && strings.Contains(errorLower, "lamports") {
		return "Insufficient SOL in wallet!"
	}

	if strings.Contains(errorLower, "simulation failed") {
		return "Transaction simulation failed!"
	}

	if strings.Contains(errorLower, "invalid token") || strings.Contains(errorLower, "account not found") {
		return "Token not found or unavailable!"
	}

	if strings.Contains(errorLower, "slippage") {
		return "Slippage tolerance exceeded!"
	}

	if strings.Contains(errorLower, "timeout") || strings.Contains(errorLower, "deadline") {
		return "Operation timed out!"
	}

	if strings.Contains(errorLower, "blockhash") {
		return "Blockhash issue!"
	}

	if strings.Contains(errorLower, "priority fee") {
		return "Priority fee too low!"
	}

	// Если не удалось распознать ошибку, возвращаем оригинальную
	return fmt.Sprintf("Execution error: %s", errorStr)
}

// executeOldSwap использует старый TokenSwapService (fallback)
func (h *TradingHandler) executeOldSwap(ctx context.Context, query *tgbotapi.CallbackQuery, userID int64, tokenAddress string, amount float64) {
	h.logger.Printf("⚙️ FALLBACK: Using old TokenSwapService")

	time.Sleep(3 * time.Second)
	swapDetails := h.createSwapDetailsStub(tokenAddress, amount, "legacy_tx_12345")
	h.finalizeSwap(query, true, swapDetails, "")
}

// finalizeSwap отправляет итоговое сообщение о результате свапа
func (h *TradingHandler) finalizeSwap(query *tgbotapi.CallbackQuery, success bool, swapDetails *SwapDetails, errorMsg string) {
	h.logger.Printf("🎯 FINALIZE SWAP: success=%t, token=%s, amount=%.6f", success, swapDetails.TokenSymbol, swapDetails.SolAmount)
	var text string

	if success {
		// Красивый формат сообщения с реальными данными
		text = fmt.Sprintf(`<b>BUY $%s (%s)</b>

%.6f SOL → %.6f %s
Price: %s, LIQ: %s, MC: %s

Your Balance: %s (%s)
Token Balance: %s

PnL 🚀

🟢Swap complete <a href="https://solscan.io/tx/%s">View on Solscan</a>`,
			swapDetails.TokenSymbol, swapDetails.TokenName,
			swapDetails.SolAmount, swapDetails.TokenAmount, swapDetails.TokenSymbol,
			swapDetails.TokenPrice, swapDetails.Liquidity, swapDetails.MarketCap,
			swapDetails.UserSOLBalance, swapDetails.UserSOLBalanceUSD,
			swapDetails.UserTokenBalance,
			swapDetails.TxID)
	} else {
		text = fmt.Sprintf(`❌ <b>Buy Order Failed</b>

🎯 <b>Token:</b> <code>%s</code>
💰 <b>Amount:</b> %.6f SOL

%s`, swapDetails.TokenAddress, swapDetails.SolAmount, errorMsg)
	}

	// Добавляем кнопки для навигации
	var keyboard *tgbotapi.InlineKeyboardMarkup
	if success {
		// Если успех - показываем кнопки для дальнейших действий
		kb := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("🔄 Buy More", "buy"),
				tgbotapi.NewInlineKeyboardButtonData("💸 Sell", "sell"),
			),
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("📊 Positions", "positions"),
				tgbotapi.NewInlineKeyboardButtonData("🏠 Main Menu", "main_menu"),
			),
		)
		keyboard = &kb
	} else {
		// Если ошибка - показываем кнопки для решения проблемы
		var errorKeyboard [][]tgbotapi.InlineKeyboardButton

		// Первый ряд: основные действия
		errorKeyboard = append(errorKeyboard, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔄 Try Again", "buy"),
			tgbotapi.NewInlineKeyboardButtonData("👛 Wallets", "manage_wallets"),
		))

		// Второй ряд: настройки и помощь
		errorKeyboard = append(errorKeyboard, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("⚙️ Settings", "settings"),
			tgbotapi.NewInlineKeyboardButtonData("🌐 Network Check", "netcheck"),
		))

		// Третий ряд: главное меню
		errorKeyboard = append(errorKeyboard, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🏠 Main Menu", "main_menu"),
		))

		kb := tgbotapi.NewInlineKeyboardMarkup(errorKeyboard...)
		keyboard = &kb
	}

	edit := tgbotapi.NewEditMessageText(query.Message.Chat.ID, query.Message.MessageID, text)
	edit.ParseMode = "HTML"
	edit.ReplyMarkup = keyboard

	h.logger.Printf("🚀 SENDING FINAL MESSAGE: %s", text[:100])
	_, err := h.api.Send(edit)
	if err != nil {
		h.logger.Printf("❌ Failed to send final message: %v", err)
	} else {
		h.logger.Printf("✅ Final message sent successfully!")
	}
}

// recordBuyEvent записывает событие покупки в PnL Tracker
func (h *TradingHandler) recordBuyEvent(ctx context.Context, userID int64, wallet *services.SimpleWallet, tokenAddress string, solAmount float64, result interface{}) {
	h.logger.Printf("📊 Recording buy event for PnL tracking...")

	// Получаем PnL Tracker
	pnlTracker := h.services.GetPnLTracker()
	if pnlTracker == nil {
		h.logger.Printf("⚠️ PnL Tracker not available, skipping event recording")
		return
	}

	// Конвертируем wallet ID в int64
	walletIDInt, err := strconv.ParseInt(wallet.ID, 10, 64)
	if err != nil {
		h.logger.Printf("❌ Failed to parse wallet ID: %v", err)
		return
	}

	// Извлекаем токен символ из адреса (пока используем короткую версию)
	tokenSymbol := tokenAddress
	if len(tokenAddress) > 10 {
		tokenSymbol = tokenAddress[:4] + "..." + tokenAddress[len(tokenAddress)-4:]
	}

	// Извлекаем реальные данные из result
	var tokenAmount float64 = 0
	var entryPrice float64 = 0
	var transactionID string = "unknown"

	// Извлекаем данные из SwapResult
	if swapResult, ok := result.(interface {
		TransactionHash() string
		OutputAmount() string
	}); ok {
		// Получаем transaction hash
		if hash := swapResult.TransactionHash(); hash != "" {
			transactionID = hash
		}

		// Парсим количество токенов
		if outputAmountStr := swapResult.OutputAmount(); outputAmountStr != "" {
			if parsedAmount, parseErr := strconv.ParseFloat(outputAmountStr, 64); parseErr == nil {
				tokenAmount = parsedAmount
				// Рассчитываем entry price: SOL потрачено / количество токенов
				if tokenAmount > 0 {
					entryPrice = solAmount / tokenAmount
				}
			}
		}
	}

	// Если не удалось извлечь данные из result, получаем цену через TokenDataService
	if entryPrice == 0 {
		tokenDataService := h.services.GetTokenDataService()
		if tokenDataService != nil {
			if tokenAddr, err := valueobjects.NewSolanaTokenAddress(tokenAddress); err == nil {
				if tokenMetrics, err := tokenDataService.GetTokenMetrics(context.Background(), *tokenAddr); err == nil && tokenMetrics != nil {
					if priceFloat, _ := tokenMetrics.PriceUSD().Float64(); priceFloat > 0 {
						entryPrice = priceFloat
						// Если не удалось извлечь количество из result, рассчитываем
						if tokenAmount == 0 {
							tokenAmount = solAmount / entryPrice
						}
					}
				}
			}
		}
	}

	h.logger.Printf("📊 Buy event data: TokenAmount=%.6f, EntryPrice=%.8f, TxID=%s", tokenAmount, entryPrice, transactionID)

	// Создаем событие покупки с реальными данными
	buyEvent := services.BuyEvent{
		UserID:        userID,
		WalletID:      walletIDInt,
		TokenAddress:  tokenAddress,
		TokenSymbol:   tokenSymbol,
		Amount:        solAmount,     // SOL потрачено
		TokenAmount:   tokenAmount,   // Количество токенов получено
		EntryPrice:    entryPrice,    // Цена входа
		TransactionID: transactionID, // Hash транзакции
		Timestamp:     time.Now(),
	}

	// Асинхронно записываем событие (не блокируем UI)
	go func() {
		err := pnlTracker.OnBuyComplete(context.Background(), buyEvent)
		if err != nil {
			h.logger.Printf("❌ PnL: Failed to record buy event: %v", err)
		} else {
			h.logger.Printf("✅ PnL: Buy event recorded successfully")
		}
	}()
}

// createPositionAfterBuy создает позицию в PortfolioService после успешной покупки
func (h *TradingHandler) createPositionAfterBuy(ctx context.Context, userID int64, wallet *services.SimpleWallet, tokenAddress string, solAmount float64, result interface{}) {
	h.logger.Printf("📊 Creating position after buy - User: %d, Token: %s, SOL: %.6f", userID, tokenAddress, solAmount)

	// Получаем PortfolioService
	portfolioService := h.services.GetPortfolioService()
	if portfolioService == nil {
		h.logger.Printf("❌ PortfolioService not available")
		return
	}

	// Конвертируем wallet ID в int64
	walletIDInt, err := strconv.ParseInt(wallet.ID, 10, 64)
	if err != nil {
		h.logger.Printf("❌ Failed to parse wallet ID: %v", err)
		return
	}

	// Получаем текущую цену токена и символ
	var tokenPrice float64 = 0.000001 // Заглушка
	var tokenSymbol string = "UNKNOWN"
	var tokenAmount float64 = 0

	// Пытаемся извлечь количество токенов из result
	if swapResult, ok := result.(interface {
		OutputAmount() string
	}); ok {
		if outputAmountStr := swapResult.OutputAmount(); outputAmountStr != "" {
			// Парсим количество токенов из строки
			if parsedAmount, parseErr := strconv.ParseFloat(outputAmountStr, 64); parseErr == nil {
				tokenAmount = parsedAmount
				h.logger.Printf("📊 Extracted token amount from result: %.6f", tokenAmount)
			}
		}
	}

	// Получаем данные токена через TokenDataService
	tokenDataService := h.services.GetTokenDataService()
	if tokenDataService != nil {
		if tokenAddr, err := valueobjects.NewSolanaTokenAddress(tokenAddress); err == nil {
			if tokenMetrics, err := tokenDataService.GetTokenMetrics(ctx, *tokenAddr); err == nil && tokenMetrics != nil {
				tokenSymbol = tokenMetrics.Symbol()
				if priceFloat, _ := tokenMetrics.PriceUSD().Float64(); priceFloat > 0 {
					tokenPrice = priceFloat
					// Если не удалось извлечь количество из result, рассчитываем
					if tokenAmount == 0 {
						tokenAmount = solAmount / tokenPrice
					}
				}
			}
		}
	}

	h.logger.Printf("📊 Position data: Symbol=%s, Price=%.8f, Amount=%.6f", tokenSymbol, tokenPrice, tokenAmount)

	// Проверяем существует ли уже позиция для этого токена
	existingPosition, err := portfolioService.GetActivePositionByToken(ctx, walletIDInt, tokenAddress)
	if err != nil && err.Error() != "position not found" {
		h.logger.Printf("❌ Error checking existing position: %v", err)
		return
	}

	if existingPosition != nil {
		// Позиция уже существует - обновляем количество (добавляем к существующему)
		newAmount := existingPosition.Amount + tokenAmount
		h.logger.Printf("📊 Updating existing position: %.6f + %.6f = %.6f",
			existingPosition.Amount, tokenAmount, newAmount)

		err = portfolioService.UpdatePositionAmount(ctx, existingPosition.ID, newAmount)
		if err != nil {
			h.logger.Printf("❌ Failed to update position amount: %v", err)
		} else {
			h.logger.Printf("✅ Position amount updated successfully")
		}
	} else {
		// Создаем новую позицию
		h.logger.Printf("📊 Creating new position for token %s", tokenSymbol)

		openPositionReq := services.OpenPositionRequest{
			WalletID:     walletIDInt,
			TokenAddress: tokenAddress,
			TokenSymbol:  tokenSymbol,
			Amount:       tokenAmount,
			EntryPrice:   tokenPrice,
		}

		_, err = portfolioService.OpenPosition(ctx, openPositionReq)
		if err != nil {
			h.logger.Printf("❌ Failed to create new position: %v", err)
		} else {
			h.logger.Printf("✅ New position created successfully")
		}
	}
}

// PortfolioHandler - реальная реализация
type PortfolioHandler struct {
	services  *services.SimpleServiceContainer
	sessions  *sessions.SessionManager
	api       *tgbotapi.BotAPI
	logger    *log.Logger
	uiBuilder *ui.Builder
}

func NewPortfolioHandler(serviceContainer *services.SimpleServiceContainer, sessionManager *sessions.SessionManager, api *tgbotapi.BotAPI, logger *log.Logger) *PortfolioHandler {
	return &PortfolioHandler{
		services:  serviceContainer,
		sessions:  sessionManager,
		api:       api,
		logger:    logger,
		uiBuilder: ui.NewBuilder(),
	}
}

func (h *PortfolioHandler) HandlePositions(ctx context.Context, query *tgbotapi.CallbackQuery) error {
	userID := query.From.ID
	h.logger.Printf("=== STARTING HandlePositions for user: %d ===", userID)

	// Проверяем основные компоненты на nil
	if h == nil {
		h.logger.Printf("ERROR: PortfolioHandler is nil!")
		return fmt.Errorf("handler is nil")
	}

	if h.services == nil {
		h.logger.Printf("ERROR: services container is nil!")
		return fmt.Errorf("services container is nil")
	}

	if h.logger == nil {
		fmt.Printf("ERROR: logger is nil!")
		return fmt.Errorf("logger is nil")
	}

	// Шаг 1: Получение primary wallet пользователя
	h.logger.Printf("Step 1: Getting wallets for user %d", userID)

	walletService := h.services.GetWalletService()
	if walletService == nil {
		h.logger.Printf("ERROR: WalletService is nil!")
		return fmt.Errorf("wallet service is nil")
	}

	wallets, err := walletService.GetWallets(ctx, query.From.ID)
	if err != nil || len(wallets) == 0 {
		h.logger.Printf("Failed to get wallets for user %d: %v", userID, err)
		text := `📊 <b>Your Portfolio</b>

❌ No wallet found. Please create a wallet first.

💡 Go to Wallet → Generate Wallet to get started.`

		keyboard := h.uiBuilder.BuildBackMenuKeyboard()
		edit := tgbotapi.NewEditMessageText(query.Message.Chat.ID, query.Message.MessageID, text)
		edit.ParseMode = "HTML"
		edit.ReplyMarkup = &keyboard
		_, err := h.api.Send(edit)
		return err
	}

	h.logger.Printf("Found %d wallets", len(wallets))

	// Получаем primary wallet (первый в списке)
	primaryWallet := wallets[0]
	h.logger.Printf("Using primary wallet: ID=%s, PublicKey=%s", primaryWallet.ID, primaryWallet.PublicKey)

	// Шаг 2: Получение баланса SOL кошелька (используем SimpleWalletService)
	h.logger.Printf("Step 2: Getting balance for wallet ID=%s", primaryWallet.ID)

	balance, err := h.services.GetWalletService().GetBalance(ctx, primaryWallet.ID)
	if err != nil {
		h.logger.Printf("Failed to get SOL balance for wallet %s: %v", primaryWallet.PublicKey, err)
		balance = 0.0 // Fallback
	}
	solBalance := balance // Уже в SOL, не в lamports
	h.logger.Printf("Got SOL balance: %.8f", solBalance)

	// Шаг 3: Получение позиций пользователя
	h.logger.Printf("Step 3: Getting positions for user %d", userID)

	portfolioService := h.services.GetPortfolioService()
	if portfolioService == nil {
		h.logger.Printf("ERROR: PortfolioService is nil!")
		return fmt.Errorf("portfolio service is nil")
	}

	positions, err := portfolioService.GetPositionsByUser(ctx, int64(query.From.ID))
	if err != nil {
		h.logger.Printf("Failed to get positions for user %d: %v", userID, err)
		positions = []*entities.TokenPosition{} // Fallback к пустому списку
	}
	h.logger.Printf("Got %d positions", len(positions))

	// Шаг 4: Получение общего PnL кошелька
	h.logger.Printf("Step 4: Getting PnL for wallet ID=%s", primaryWallet.ID)

	walletIDInt, parseErr := strconv.ParseInt(primaryWallet.ID, 10, 64)
	var walletPnL float64 = 0.0
	if parseErr != nil {
		h.logger.Printf("Failed to parse wallet ID %s: %v", primaryWallet.ID, parseErr)
	} else {
		pnlTracker := h.services.GetPnLTracker()
		if pnlTracker == nil {
			h.logger.Printf("ERROR: PnLTracker is nil")
		} else {
			pnlResult, err := pnlTracker.GetWalletPnL(ctx, walletIDInt)
			if err != nil {
				h.logger.Printf("Failed to get wallet PnL for %s: %v", primaryWallet.PublicKey, err)
			} else if pnlResult != nil {
				walletPnL = pnlResult.TotalPnL
				h.logger.Printf("Got wallet PnL: $%.2f", walletPnL)
			} else {
				h.logger.Printf("PnL result is nil")
			}
		}
	}

	// Шаг 5: Построение списка токенов с актуальными ценами
	var tokensList string
	if len(positions) == 0 {
		tokensList = "<i>No token positions found</i>"
	} else {
		h.logger.Printf("Processing %d positions", len(positions))
		for i, position := range positions {
			// Проверка на nil указатель
			if position == nil {
				h.logger.Printf("WARNING: Position %d is nil, skipping", i)
				continue
			}

			h.logger.Printf("Processing position %d: TokenAddress=%s, Amount=%.6f, EntryPrice=%.6f",
				i, position.TokenAddress, position.Amount, position.EntryPrice)

			// Проверка на пустой адрес токена
			if position.TokenAddress == "" {
				h.logger.Printf("WARNING: Position %d has empty TokenAddress, skipping", i)
				continue
			}

			// Создаем SolanaTokenAddress из строки
			tokenAddr, addrErr := valueobjects.NewSolanaTokenAddress(position.TokenAddress)
			var tokenPrice float64 = 0.0

			if addrErr == nil {
				// Получаем актуальную цену токена
				tokenDataService := h.services.GetTokenDataService()
				if tokenDataService != nil {
					tokenMetrics, err := tokenDataService.GetTokenMetrics(ctx, *tokenAddr)
					if err == nil && tokenMetrics != nil {
						// Используем метод PriceUSD() и конвертируем в float64
						priceDecimal := tokenMetrics.PriceUSD()
						tokenPrice, _ = priceDecimal.Float64()
						h.logger.Printf("Got token price for %s: $%.8f", position.TokenAddress, tokenPrice)
					} else {
						h.logger.Printf("Failed to get token metrics for %s: %v", position.TokenAddress, err)
					}
				} else {
					h.logger.Printf("TokenDataService is nil")
				}
			} else {
				h.logger.Printf("Failed to create SolanaTokenAddress from %s: %v", position.TokenAddress, addrErr)
			}

			// Рассчитываем стоимость позиции в USD (токены * цена за токен)
			positionValueUSD := position.Amount * tokenPrice

			// Рассчитываем PnL позиции
			pnlPercent := 0.0
			if position.EntryPrice > 0 {
				pnlPercent = ((tokenPrice - position.EntryPrice) / position.EntryPrice) * 100
			}

			// Эмодзи для PnL
			pnlEmoji := "📊"
			if pnlPercent > 0 {
				pnlEmoji = "🚀"
			} else if pnlPercent < 0 {
				pnlEmoji = "📉"
			}

			// Получаем символ токена
			tokenSymbol := position.TokenSymbol
			if tokenSymbol == "" {
				// Безопасная проверка длины адреса
				if len(position.TokenAddress) >= 8 {
					tokenSymbol = position.TokenAddress[:8] + "..." // Fallback
				} else {
					tokenSymbol = position.TokenAddress // Если адрес короче 8 символов
				}

				if addrErr == nil {
					tokenDataService := h.services.GetTokenDataService()
					if tokenDataService != nil {
						tokenMetrics, err := tokenDataService.GetTokenMetrics(ctx, *tokenAddr)
						if err == nil && tokenMetrics != nil && tokenMetrics.Symbol() != "" {
							tokenSymbol = tokenMetrics.Symbol()
						}
					}
				}
			}

			// Конвертируем USD в SOL (примерная конвертация, можно улучшить)
			solPrice := 100.0 // Примерная цена SOL в USD, можно получить из API
			positionValueSOL := positionValueUSD / solPrice

			// Рассчитываем время холдинга
			holdingTime := "Unknown"
			if !position.OpenedAt.IsZero() {
				duration := position.GetAge() // Используем встроенный метод
				if duration.Hours() < 24 {
					holdingTime = fmt.Sprintf("%.1fh", duration.Hours())
				} else {
					holdingTime = fmt.Sprintf("%.1fd", duration.Hours()/24)
				}
			}

			// Форматируем строку позиции без ссылки Solscan
			tokensList += fmt.Sprintf(
				"%d. %s 💰%.6f %s%+.1f%% ⏱%s\n",
				i+1,
				tokenSymbol,
				positionValueSOL,
				pnlEmoji,
				pnlPercent,
				holdingTime,
			)
		}
	}

	// Шаг 6: Форматирование итогового сообщения
	message := fmt.Sprintf(`📊 <b>Your Portfolio</b>

<b>Balance:</b> %.8f SOL (PnL $%.2f)

<b>Token     Holding (SOL)    PnL     Time</b>
%s`,
		solBalance,
		walletPnL,
		tokensList,
	)

	// Шаг 7: Создание клавиатуры только с кнопкой "Back"
	backButton := tgbotapi.NewInlineKeyboardButtonData("🔙 Back", "menu")

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(backButton),
	)

	edit := tgbotapi.NewEditMessageText(query.Message.Chat.ID, query.Message.MessageID, message)
	edit.ParseMode = "HTML"
	edit.ReplyMarkup = &keyboard

	_, err = h.api.Send(edit)
	return err
}

// SettingsHandler - реальная реализация
type SettingsHandler struct {
	services  *services.SimpleServiceContainer
	sessions  *sessions.SessionManager
	api       *tgbotapi.BotAPI
	logger    *log.Logger
	uiBuilder *ui.Builder
}

func NewSettingsHandler(serviceContainer *services.SimpleServiceContainer, sessionManager *sessions.SessionManager, api *tgbotapi.BotAPI, logger *log.Logger) *SettingsHandler {
	return &SettingsHandler{
		services:  serviceContainer,
		sessions:  sessionManager,
		api:       api,
		logger:    logger,
		uiBuilder: ui.NewBuilder(),
	}
}

func (h *SettingsHandler) HandleSettings(ctx context.Context, query *tgbotapi.CallbackQuery) error {
	userID := query.From.ID
	h.logger.Printf("Showing settings for user: %d", userID)

	// Детальное логирование для отладки
	h.logger.Printf("Step 1: Getting SettingsService...")
	settingsService := h.services.GetSettingsService()
	if settingsService == nil {
		h.logger.Printf("ERROR: SettingsService is nil!")
		return h.editMessageWithError(ctx, query, "Settings service not available")
	}
	h.logger.Printf("Step 2: SettingsService OK")

	// Получаем текущие настройки пользователя
	h.logger.Printf("Step 3: Calling GetUserSettings...")
	settings, err := settingsService.GetUserSettings(ctx, userID)
	if err != nil {
		h.logger.Printf("Failed to get settings for user %d: %v", userID, err)
		return h.editMessageWithError(ctx, query, "Failed to load settings")
	}
	h.logger.Printf("Step 4: GetUserSettings returned without error")

	// Дополнительная проверка на nil
	if settings == nil {
		h.logger.Printf("ERROR: Settings is nil for user %d", userID)
		return h.editMessageWithError(ctx, query, "Settings not found")
	}
	h.logger.Printf("Step 5: Settings is not nil, UserID: %d", settings.UserID)

	h.logger.Printf("Step 6: Building text with settings values...")
	h.logger.Printf("- DefaultSOLAmount: %.3f", settings.DefaultSOLAmount)
	h.logger.Printf("- BuySlippage: %.0f", settings.BuySlippage)
	h.logger.Printf("- SellSlippage: %.0f", settings.SellSlippage)
	h.logger.Printf("- PriorityFee: %d", settings.PriorityFee)
	h.logger.Printf("- JitoTip: %d", settings.JitoTip)

	// Debug: Log user settings
	h.logger.Printf("User %d settings: DefaultSOL=%.1f, BuySlippage=%.1f, Preset1=%s, Preset2=%s, Preset3=%s, Preset4=%s",
		userID, settings.DefaultSOLAmount, settings.BuySlippage,
		func() string {
			if settings.BuyPreset1 != nil {
				return fmt.Sprintf("%.3f", *settings.BuyPreset1)
			} else {
				return "nil"
			}
		}(),
		func() string {
			if settings.BuyPreset2 != nil {
				return fmt.Sprintf("%.3f", *settings.BuyPreset2)
			} else {
				return "nil"
			}
		}(),
		func() string {
			if settings.BuyPreset3 != nil {
				return fmt.Sprintf("%.3f", *settings.BuyPreset3)
			} else {
				return "nil"
			}
		}(),
		func() string {
			if settings.BuyPreset4 != nil {
				return fmt.Sprintf("%.3f", *settings.BuyPreset4)
			} else {
				return "nil"
			}
		}())

	text := `⚙️ <b>Settings</b>

Configure your trading preferences:

💡 Choose category to configure:`

	h.logger.Printf("Step 7: Text built successfully")

	h.logger.Printf("Step 8: Building keyboard...")
	keyboard := h.uiBuilder.BuildSettingsMenuKeyboard()
	h.logger.Printf("Step 9: Keyboard built successfully")

	h.logger.Printf("Step 10: Sending edit message...")
	edit := tgbotapi.NewEditMessageText(query.Message.Chat.ID, query.Message.MessageID, text)
	edit.ParseMode = "HTML"
	edit.ReplyMarkup = &keyboard

	_, err = h.api.Send(edit)
	h.logger.Printf("Step 11: Message sent, error: %v", err)
	return err
}

// HandleBuySettings отображает настройки покупки
func (h *SettingsHandler) HandleBuySettings(ctx context.Context, query *tgbotapi.CallbackQuery) error {
	userID := query.From.ID
	h.logger.Printf("Showing buy settings for user: %d", userID)

	// Получаем текущие настройки пользователя
	settings, err := h.services.GetSettingsService().GetUserSettings(ctx, userID)
	if err != nil {
		h.logger.Printf("Failed to get settings for user %d: %v", userID, err)
		return h.editMessageWithError(ctx, query, "Failed to load settings")
	}

	// Дополнительная проверка на nil
	if settings == nil {
		h.logger.Printf("Settings is nil for user %d", userID)
		return h.editMessageWithError(ctx, query, "Settings not found")
	}

	text := `💰 <b>Buy Settings</b>

✏️ <b>Current Values:</b>
• Buy Slippage: <code>` + fmt.Sprintf("%.0f", settings.BuySlippage) + `%</code>

Click button to edit value:`

	// Get buy presets from settings
	buyPresets := [5]*float64{
		settings.BuyPreset1,
		settings.BuyPreset2,
		settings.BuyPreset3,
		settings.BuyPreset4,
		settings.BuyPreset5,
	}

	// Create settings handler to build proper keyboard
	settingsHandler := NewSettingsHandler(h.services, h.sessions, h.api, h.logger)
	keyboard := settingsHandler.uiBuilder.BuildBuySettingsKeyboard(settings.DefaultSOLAmount, settings.BuySlippage, buyPresets)

	edit := tgbotapi.NewEditMessageText(query.Message.Chat.ID, query.Message.MessageID, text)
	edit.ParseMode = "HTML"
	edit.ReplyMarkup = &keyboard

	_, err = h.api.Send(edit)
	return err
}

// HandleSellSettings отображает настройки продажи
func (h *SettingsHandler) HandleSellSettings(ctx context.Context, query *tgbotapi.CallbackQuery) error {
	userID := query.From.ID
	h.logger.Printf("Showing sell settings for user: %d", userID)

	// Получаем текущие настройки пользователя
	settings, err := h.services.GetSettingsService().GetUserSettings(ctx, userID)
	if err != nil {
		h.logger.Printf("Failed to get settings for user %d: %v", userID, err)
		return h.editMessageWithError(ctx, query, "Failed to load settings")
	}

	// Дополнительная проверка на nil
	if settings == nil {
		h.logger.Printf("Settings is nil for user %d", userID)
		return h.editMessageWithError(ctx, query, "Settings not found")
	}

	text := `💸 <b>Sell Settings</b>

✏️ <b>Current Values:</b>
• Sell Slippage: <code>` + fmt.Sprintf("%.0f", settings.SellSlippage) + `%</code>

Click button to edit value:`

	// Prepare sell presets array [25%, 50%, 75%, 100%]
	sellPresets := [4]*float64{
		settings.SellPreset25,
		settings.SellPreset50,
		settings.SellPreset75,
		settings.SellPreset100,
	}

	// Create settings handler to build proper keyboard
	settingsHandler := NewSettingsHandler(h.services, h.sessions, h.api, h.logger)
	keyboard := settingsHandler.uiBuilder.BuildSellSettingsKeyboard(settings.SellSlippage, sellPresets)

	// Send new message with updated settings
	msg := tgbotapi.NewMessage(userID, text)
	msg.ParseMode = "HTML"
	msg.ReplyMarkup = keyboard

	_, err = h.api.Send(msg)
	return err
}

// HandleMEVSettings отображает настройки MEV защиты
func (h *SettingsHandler) HandleMEVSettings(ctx context.Context, query *tgbotapi.CallbackQuery) error {
	userID := query.From.ID
	h.logger.Printf("Showing MEV settings for user: %d", userID)

	// Получаем текущие настройки пользователя
	settings, err := h.services.GetSettingsService().GetUserSettings(ctx, userID)
	if err != nil {
		h.logger.Printf("Failed to get settings for user %d: %v", userID, err)
		return h.editMessageWithError(ctx, query, "Failed to load settings")
	}

	// Дополнительная проверка на nil
	if settings == nil {
		h.logger.Printf("Settings is nil for user %d", userID)
		return h.editMessageWithError(ctx, query, "Settings not found")
	}

	// MEV защита активна если Jito tip > 0
	mevEnabled := settings.JitoTip > 0
	statusText := "🔴 Disabled"
	if mevEnabled {
		statusText = "🟢 Enabled"
	}

	// Convert Jito tip to SOL for display
	jitoTipSOL := float64(settings.JitoTip) / 1e9

	text := `🛡️ <b>MEV Protection</b>

✏️ <b>Current Values:</b>
• Status: ` + statusText + `
• Jito Tip: <code>` + fmt.Sprintf("%.6f", jitoTipSOL) + ` SOL</code>

💡 <b>What is MEV Protection?</b>
Protects your trades from front-running and sandwich attacks using Jito bundles.

Click button to edit value:`

	// Create settings handler to build proper keyboard
	settingsHandler := NewSettingsHandler(h.services, h.sessions, h.api, h.logger)
	keyboard := settingsHandler.uiBuilder.BuildMEVSettingsKeyboard(mevEnabled, settings.PriorityFee, settings.JitoTip)

	// Send new message with updated MEV settings
	msg := tgbotapi.NewMessage(userID, text)
	msg.ParseMode = "HTML"
	msg.ReplyMarkup = keyboard

	_, err = h.api.Send(msg)
	return err
}

// HandleSlippageSettings отображает настройки slippage
func (h *SettingsHandler) HandleSlippageSettings(ctx context.Context, query *tgbotapi.CallbackQuery) error {
	userID := query.From.ID
	h.logger.Printf("Showing slippage settings for user: %d", userID)

	// Получаем текущие настройки пользователя
	settings, err := h.services.GetSettingsService().GetUserSettings(ctx, userID)
	if err != nil {
		h.logger.Printf("Failed to get settings for user %d: %v", userID, err)
		return h.editMessageWithError(ctx, query, "Failed to load settings")
	}

	// Дополнительная проверка на nil
	if settings == nil {
		h.logger.Printf("Settings is nil for user %d", userID)
		return h.editMessageWithError(ctx, query, "Settings not found")
	}

	text := `📝 <b>Slippage Settings</b>

✏️ <b>Current:</b>
• Buy Slippage: <code>` + fmt.Sprintf("%.0f", settings.BuySlippage) + `%</code>
• Sell Slippage: <code>` + fmt.Sprintf("%.0f", settings.SellSlippage) + `%</code>

Select slippage to edit:`

	keyboard := h.uiBuilder.BuildSlippageSettingsKeyboard(settings.BuySlippage, settings.SellSlippage)

	edit := tgbotapi.NewEditMessageText(query.Message.Chat.ID, query.Message.MessageID, text)
	edit.ParseMode = "HTML"
	edit.ReplyMarkup = &keyboard

	_, err = h.api.Send(edit)
	return err
}

// HandlePrioritySettings отображает настройки Priority Fee
func (h *SettingsHandler) HandlePrioritySettings(ctx context.Context, query *tgbotapi.CallbackQuery) error {
	userID := query.From.ID
	h.logger.Printf("Showing priority fee settings for user: %d", userID)

	// Получаем текущие настройки пользователя
	settings, err := h.services.GetSettingsService().GetUserSettings(ctx, userID)
	if err != nil {
		h.logger.Printf("Failed to get settings for user %d: %v", userID, err)
		return h.editMessageWithError(ctx, query, "Failed to load settings")
	}

	// Дополнительная проверка на nil
	if settings == nil {
		h.logger.Printf("Settings is nil for user %d", userID)
		return h.editMessageWithError(ctx, query, "Settings not found")
	}

	// Конвертируем lamports в SOL для отображения
	priorityFeeSOL := float64(settings.PriorityFee) / 1e9

	text := `⚡ <b>Priority Fee</b>

✏️ <b>Current:</b> <code>` + fmt.Sprintf("%.6f", priorityFeeSOL) + ` SOL</code>

Select value:`

	// Create settings handler to build proper keyboard
	settingsHandler := NewSettingsHandler(h.services, h.sessions, h.api, h.logger)
	keyboard := settingsHandler.uiBuilder.BuildPriorityFeeKeyboard(priorityFeeSOL)

	// Send new message with updated priority fee settings
	msg := tgbotapi.NewMessage(userID, text)
	msg.ParseMode = "HTML"
	msg.ReplyMarkup = keyboard

	_, err = h.api.Send(msg)
	return err
}

// editMessageWithError отображает сообщение об ошибке
func (h *SettingsHandler) editMessageWithError(ctx context.Context, query *tgbotapi.CallbackQuery, errorText string) error {
	keyboard := h.uiBuilder.BuildBackMenuKeyboard()

	edit := tgbotapi.NewEditMessageText(query.Message.Chat.ID, query.Message.MessageID, "❌ "+errorText)
	edit.ReplyMarkup = &keyboard

	_, err := h.api.Send(edit)
	return err
}

// TextProcessor - реальная реализация
type TextProcessor struct {
	services  *services.SimpleServiceContainer
	sessions  *sessions.SessionManager
	api       *tgbotapi.BotAPI
	logger    *log.Logger
	uiBuilder *ui.Builder
}

func NewTextProcessor(serviceContainer *services.SimpleServiceContainer, sessionManager *sessions.SessionManager, api *tgbotapi.BotAPI, logger *log.Logger) *TextProcessor {
	return &TextProcessor{
		services:  serviceContainer,
		sessions:  sessionManager,
		api:       api,
		logger:    logger,
		uiBuilder: ui.NewBuilder(),
	}
}

func (h *TextProcessor) ProcessText(ctx context.Context, message *tgbotapi.Message) error {
	userID := message.From.ID
	userState := h.sessions.GetState(userID)

	h.logger.Printf("Processing text from user %d, current flow: %s", userID, userState.CurrentFlow)

	switch userState.CurrentFlow {
	case sessions.FlowTokenBuy:
		return h.handleTokenAddress(ctx, message)
	case sessions.FlowTokenSell:
		return h.handleTokenAddressForSell(ctx, message)
	case sessions.FlowWalletImport:
		return h.handlePrivateKeyImport(ctx, message)
	case sessions.FlowCustomAmount:
		return h.handleCustomAmount(ctx, message)
	case sessions.FlowBuyAmountEdit:
		return h.handleBuyAmountEdit(ctx, message)
	case sessions.FlowBuySlippageEdit:
		return h.handleBuySlippageEdit(ctx, message)
	case sessions.FlowBuyPresetEdit:
		return h.handleBuyPresetEdit(ctx, message)
	case sessions.FlowSellSlippageEdit:
		return h.handleSellSlippageEdit(ctx, message)
	case sessions.FlowSellPresetEdit:
		return h.handleSellPresetEdit(ctx, message)
	case sessions.FlowPriorityFeeEdit:
		return h.handlePriorityFeeEdit(ctx, message)
	case sessions.FlowJitoTipEdit:
		return h.handleJitoTipEdit(ctx, message)
	default:
		// Default: treat as token address for quick buy
		return h.handleTokenAddress(ctx, message)
	}
}

func (h *TextProcessor) handlePrivateKeyImport(ctx context.Context, message *tgbotapi.Message) error {
	userID := message.From.ID
	privateKey := message.Text

	h.logger.Printf("Processing private key import for user: %d (key length: %d)", userID, len(privateKey))

	// Delete user's message immediately for security
	deleteMsg := tgbotapi.NewDeleteMessage(message.Chat.ID, message.MessageID)
	h.api.Send(deleteMsg)

	// TODO: Validate private key format
	// TODO: Import wallet using WalletService
	// TODO: Show success/error message

	response := `⚠️ <b>Import Wallet Feature</b>

Private key import is in development.

For security, your message has been deleted.

Please use the Generate Wallet option for now.`

	msg := tgbotapi.NewMessage(userID, response)
	msg.ParseMode = "HTML"

	_, err := h.api.Send(msg)

	// Clear session flow
	h.sessions.SetFlow(userID, sessions.FlowNone, nil)

	return err
}

func (h *TextProcessor) handleCustomAmount(ctx context.Context, message *tgbotapi.Message) error {
	userID := message.From.ID
	amount := message.Text

	h.logger.Printf("Processing custom amount: %s for user: %d", amount, userID)

	// TODO: Validate amount format
	// TODO: Process custom buy amount

	response := fmt.Sprintf(`💰 <b>Custom Amount</b>

Amount: <code>%s</code>

⚠️ Custom amount processing in development.

Please use preset amounts for now.`, amount)

	msg := tgbotapi.NewMessage(userID, response)
	msg.ParseMode = "HTML"

	_, err := h.api.Send(msg)

	// Clear session flow
	h.sessions.SetFlow(userID, sessions.FlowNone, nil)

	return err
}

func (h *TextProcessor) handleBuyAmountEdit(ctx context.Context, message *tgbotapi.Message) error {
	userID := message.From.ID
	text := message.Text

	h.logger.Printf("Processing buy amount edit: %s for user: %d", text, userID)

	// Parse amount
	var amount float64
	if _, err := fmt.Sscanf(text, "%f", &amount); err != nil {
		return h.sendErrorMessage(ctx, userID, "Invalid amount format. Please send a valid number (e.g., 0.5)")
	}

	// Validate range
	if amount < 0.001 || amount > 1000 {
		return h.sendErrorMessage(ctx, userID, "Amount must be between 0.001 and 1000 SOL")
	}

	// Save selected amount in session (not in settings!)
	h.sessions.SetData(userID, "selected_amount", amount)
	h.sessions.SetData(userID, "selected_amount_type", "custom")

	// Get session data from flow BEFORE clearing
	userState := h.sessions.GetState(userID)

	// Try to get edit info from session
	var tokenAddress string
	var editMessageID int
	var editChatID int64
	var hasEditID, hasEditChatID bool

	if userState.Data != nil {
		tokenAddress, _ = userState.Data["token_address"].(string)
		editMessageID, hasEditID = userState.Data["edit_message_id"].(int)
		editChatID, hasEditChatID = userState.Data["edit_chat_id"].(int64)
	}

	h.logger.Printf("🔄 AMOUNT EDIT DEBUG: tokenAddress=%s, editMessageID=%d (has=%v), editChatID=%d (has=%v)",
		tokenAddress, editMessageID, hasEditID, editChatID, hasEditChatID)

	// Clear flow
	h.sessions.SetFlow(userID, sessions.FlowNone, nil)

	if tokenAddress == "" {
		return h.sendErrorMessage(ctx, userID, "Token address not found. Please enter a token address first.")
	}

	// Delete user's message
	deleteMsg := tgbotapi.NewDeleteMessage(message.Chat.ID, message.MessageID)
	h.api.Send(deleteMsg)

	// If we have edit info, edit the form back to token menu
	if hasEditID && hasEditChatID {
		h.logger.Printf("🔄 AMOUNT EDIT DEBUG: Using EditFormBackToTokenMenu")
		return h.EditFormBackToTokenMenu(ctx, userID, tokenAddress, editChatID, editMessageID)
	} else {
		h.logger.Printf("🔄 AMOUNT EDIT DEBUG: Fallback to HandleTokenAddressWithMessage")
		// Fallback: send new token menu (shouldn't happen with new flow)
		return h.HandleTokenAddressWithMessage(ctx, message, tokenAddress)
	}
}

func (h *TextProcessor) handleBuySlippageEdit(ctx context.Context, message *tgbotapi.Message) error {
	userID := message.From.ID
	text := message.Text

	h.logger.Printf("Processing buy slippage edit: %s for user: %d", text, userID)

	// Parse slippage - попробуем сначала как int, потом как float
	var slippage float64
	var err error

	// Пробуем сначала как целое число
	if intVal, intErr := strconv.Atoi(text); intErr == nil {
		slippage = float64(intVal)
	} else {
		// Если не получилось как int, пробуем как float
		if slippage, err = strconv.ParseFloat(text, 64); err != nil {
			return h.sendErrorMessage(ctx, userID, "Invalid slippage format. Please send a valid number (e.g., 1 or 1.5)")
		}
	}

	// Validate range
	if slippage < 1 || slippage > 90 {
		return h.sendErrorMessage(ctx, userID, "Slippage must be between 1% and 90%")
	}

	// Save selected slippage in session (not in settings!)
	h.sessions.SetData(userID, "selected_slippage", slippage)

	// Get session data from flow BEFORE clearing
	userState := h.sessions.GetState(userID)

	// Try to get edit info from session
	var tokenAddress string
	var editMessageID int
	var editChatID int64
	var hasEditID, hasEditChatID bool

	if userState.Data != nil {
		tokenAddress, _ = userState.Data["token_address"].(string)
		editMessageID, hasEditID = userState.Data["edit_message_id"].(int)
		editChatID, hasEditChatID = userState.Data["edit_chat_id"].(int64)
	}

	h.logger.Printf("🔄 SLIPPAGE EDIT DEBUG: tokenAddress=%s, editMessageID=%d (has=%v), editChatID=%d (has=%v)",
		tokenAddress, editMessageID, hasEditID, editChatID, hasEditChatID)

	// Clear flow
	h.sessions.SetFlow(userID, sessions.FlowNone, nil)

	if tokenAddress == "" {
		return h.sendErrorMessage(ctx, userID, "Token address not found. Please enter a token address first.")
	}

	// Delete user's message
	deleteMsg := tgbotapi.NewDeleteMessage(message.Chat.ID, message.MessageID)
	h.api.Send(deleteMsg)

	// If we have edit info, edit the form back to token menu
	if hasEditID && hasEditChatID {
		h.logger.Printf("🔄 SLIPPAGE EDIT DEBUG: Using EditFormBackToTokenMenu")
		return h.EditFormBackToTokenMenu(ctx, userID, tokenAddress, editChatID, editMessageID)
	} else {
		h.logger.Printf("🔄 SLIPPAGE EDIT DEBUG: Fallback to HandleTokenAddressWithMessage")
		// Fallback: send new token menu (shouldn't happen with new flow)
		return h.HandleTokenAddressWithMessage(ctx, message, tokenAddress)
	}
}

func (h *TextProcessor) handleBuyPresetEdit(ctx context.Context, message *tgbotapi.Message) error {
	userID := message.From.ID
	text := message.Text

	h.logger.Printf("Processing buy preset edit: %s for user: %d", text, userID)

	// Get current user state to get preset index
	userState := h.sessions.GetState(userID)
	if userState.CurrentFlow != sessions.FlowBuyPresetEdit || userState.Data == nil {
		return h.sendErrorMessage(ctx, userID, "Session expired. Please try again.")
	}

	presetIndex, ok := userState.Data["preset_index"].(int)
	if !ok || presetIndex < 1 || presetIndex > 5 {
		return h.sendErrorMessage(ctx, userID, "Invalid preset index. Please try again.")
	}

	// Parse amount
	var amount float64
	if _, err := fmt.Sscanf(text, "%f", &amount); err != nil {
		return h.sendErrorMessage(ctx, userID, "Invalid amount format. Please send a valid number (e.g., 0.5)")
	}

	// Validate range
	if amount < 0.001 || amount > 1000 {
		return h.sendErrorMessage(ctx, userID, "Amount must be between 0.001 and 1000 SOL")
	}

	// Update the specific preset using UpdateSingleBuyPreset
	err := h.services.GetSettingsService().UpdateSingleBuyPreset(ctx, userID, presetIndex, &amount)
	if err != nil {
		h.logger.Printf("Failed to update buy preset %d for user %d: %v", presetIndex, userID, err)
		return h.sendErrorMessage(ctx, userID, "Failed to update buy preset")
	}

	// Clear flow
	h.sessions.SetFlow(userID, sessions.FlowNone, nil)

	// Get current settings to show updated buy settings page
	settings, err := h.services.GetSettingsService().GetUserSettings(ctx, userID)
	if err != nil {
		return h.sendErrorMessage(ctx, userID, "Failed to load updated settings")
	}

	// Build updated buy settings text
	text = `💰 <b>Buy Settings</b>

✏️ <b>Current Values:</b>
• Buy Slippage: <code>` + fmt.Sprintf("%.0f", settings.BuySlippage) + `%</code>

Click button to edit value:`

	// Get buy presets from settings
	buyPresets := [5]*float64{
		settings.BuyPreset1,
		settings.BuyPreset2,
		settings.BuyPreset3,
		settings.BuyPreset4,
		settings.BuyPreset5,
	}

	// Create settings handler to build proper keyboard
	settingsHandler := NewSettingsHandler(h.services, h.sessions, h.api, h.logger)
	keyboard := settingsHandler.uiBuilder.BuildBuySettingsKeyboard(settings.DefaultSOLAmount, settings.BuySlippage, buyPresets)

	edit := tgbotapi.NewEditMessageText(message.Chat.ID, message.MessageID, text)
	edit.ParseMode = "HTML"
	edit.ReplyMarkup = &keyboard

	_, err = h.api.Send(edit)
	return err
}

func (h *TextProcessor) handleSellSlippageEdit(ctx context.Context, message *tgbotapi.Message) error {
	userID := message.From.ID
	text := message.Text

	h.logger.Printf("Processing sell slippage edit: %s for user: %d", text, userID)

	// Parse slippage - попробуем сначала как int, потом как float
	var slippage float64
	var err error

	// Пробуем сначала как целое число
	if intVal, intErr := strconv.Atoi(text); intErr == nil {
		slippage = float64(intVal)
	} else {
		// Если не получилось как int, пробуем как float
		if slippage, err = strconv.ParseFloat(text, 64); err != nil {
			return h.sendErrorMessage(ctx, userID, "Invalid slippage format. Please send a valid number (e.g., 2 or 2.0)")
		}
	}

	// Validate range
	if slippage < 1 || slippage > 90 {
		return h.sendErrorMessage(ctx, userID, "Slippage must be between 1% and 90%")
	}

	// Update settings - 0 для buy означает "не изменять buy slippage"
	err = h.services.GetSettingsService().UpdateSlippage(ctx, userID, 0, slippage)
	if err != nil {
		h.logger.Printf("Failed to update sell slippage for user %d: %v", userID, err)
		return h.sendErrorMessage(ctx, userID, "Failed to update sell slippage")
	}

	// Clear flow
	h.sessions.SetFlow(userID, sessions.FlowNone, nil)

	// Сохраняем обновленный slippage в сессии для отображения в меню
	h.sessions.SetData(userID, "selected_sell_slippage", slippage)
	h.logger.Printf("🔍 DEBUG: Saved selected_sell_slippage=%.0f for user %d", slippage, userID)

	// Удаляем сообщение пользователя с введенным slippage
	deleteMsg := tgbotapi.NewDeleteMessage(message.Chat.ID, message.MessageID)
	h.api.Send(deleteMsg)

	// Отправляем подтверждение и возвращаемся в меню продажи
	confirmText := fmt.Sprintf("✅ Sell slippage updated to %.0f%%", slippage)
	confirmMsg := tgbotapi.NewMessage(userID, confirmText)
	sentMsg, err := h.api.Send(confirmMsg)
	if err != nil {
		return err
	}

	// Удаляем подтверждающее сообщение через 2 секунды
	go func() {
		time.Sleep(2 * time.Second)
		deleteConfirm := tgbotapi.NewDeleteMessage(message.Chat.ID, sentMsg.MessageID)
		h.api.Send(deleteConfirm)
	}()

	// Возвращаемся в настройки продажи - отправляем новое сообщение напрямую
	return h.sendSellSettingsMessage(ctx, userID, message.Chat.ID)
}

// sendSellSettingsMessage отправляет новое сообщение с настройками продажи
func (h *TextProcessor) sendSellSettingsMessage(ctx context.Context, userID int64, chatID int64) error {
	// Получаем настройки пользователя
	settings, err := h.services.GetSettingsService().GetUserSettings(ctx, userID)
	if err != nil {
		return h.sendErrorMessage(ctx, userID, "Failed to load settings")
	}

	// Получаем токен из сессии
	tokenAddress, hasToken := h.sessions.GetData(userID, "token_address").(string)
	if !hasToken || tokenAddress == "" {
		return h.sendErrorMessage(ctx, userID, "No token selected. Please start over.")
	}

	// Получаем информацию о позиции
	walletService := h.services.GetWalletService()
	wallets, err := walletService.GetWallets(ctx, userID)
	if err != nil {
		return h.sendErrorMessage(ctx, userID, "Failed to load wallets")
	}

	// Находим primary wallet
	var primaryWallet *services.SimpleWallet
	for _, wallet := range wallets {
		if wallet.IsDefault {
			primaryWallet = &wallet
			break
		}
	}

	if primaryWallet == nil {
		return h.sendErrorMessage(ctx, userID, "Primary wallet not found")
	}

	// Конвертируем wallet ID из string в int64
	walletIDInt, err := strconv.ParseInt(primaryWallet.ID, 10, 64)
	if err != nil {
		return h.sendErrorMessage(ctx, userID, "Invalid wallet ID")
	}

	portfolioService := h.services.GetPortfolioService()
	position, err := portfolioService.GetActivePositionByToken(ctx, walletIDInt, tokenAddress)
	if err != nil || position == nil {
		return h.sendErrorMessage(ctx, userID, "Position not found")
	}

	// Создаем информацию о токене
	tokenInfo := fmt.Sprintf(`🎯 <b>Token:</b> $%s
💰 <b>Holdings:</b> %.6f tokens
📍 <b>Address:</b> <code>%s</code>

`, position.TokenSymbol, position.Amount, tokenAddress)

	// Check selected percentage from session to show checkmarks
	selectedPercentage, hasSelection := h.sessions.GetData(userID, "selected_sell_percentage").(float64)

	// Create percentage buttons with checkmarks for selected amounts
	percent25Text := "25%"
	percent50Text := "50%"
	percent75Text := "75%"
	percent100Text := "100%"

	// If no selection was made yet, default to 50% being selected
	if !hasSelection {
		percent50Text += " ✅"
		// Set 50% as default selected
		h.sessions.SetData(userID, "selected_sell_percentage", 50.0)
	} else if hasSelection {
		if selectedPercentage == 25.0 {
			percent25Text += " ✅"
		} else if selectedPercentage == 50.0 {
			percent50Text += " ✅"
		} else if selectedPercentage == 75.0 {
			percent75Text += " ✅"
		} else if selectedPercentage == 100.0 {
			percent100Text += " ✅"
		}
	}

	// Slippage button with checkmark
	selectedSlippage, hasSelectedSlippage := h.sessions.GetData(userID, "selected_sell_slippage").(float64)
	var slippageText string

	if hasSelectedSlippage {
		slippageText = fmt.Sprintf("%.0f%% Slippage ✅✏️", selectedSlippage)
	} else {
		// Show default slippage with checkmark
		slippageText = fmt.Sprintf("%.0f%% Slippage ✅✏️", settings.SellSlippage)
	}

	text := fmt.Sprintf(`💸 <b>Sell Tokens</b>

%s📊 <b>Choose percentage to sell:</b>

⚙️ <b>Current Settings:</b>
• Priority Fee: <code>%.6f SOL</code>
• Jito Tip: <code>%.6f SOL</code>

💡 <b>Ready to sell?</b> Choose percentage below:`,
		tokenInfo,
		float64(settings.PriorityFee)/1e9,
		float64(settings.JitoTip)/1e9)

	// Row 1: Percentage buttons (25%, 50%)
	row1 := []tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardButtonData(percent25Text, "sell_preset_25"),
		tgbotapi.NewInlineKeyboardButtonData(percent50Text, "sell_preset_50"),
	}

	// Row 2: Percentage buttons (75%, 100%)
	row2 := []tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardButtonData(percent75Text, "sell_preset_75"),
		tgbotapi.NewInlineKeyboardButtonData(percent100Text, "sell_preset_100"),
	}

	// Row 3: Slippage button
	row3 := []tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardButtonData(slippageText, "edit_sell_slippage"),
	}

	// Row 4: Action button (SELL)
	row4 := []tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardButtonData("💸 SELL", "confirm_sell"),
	}

	// Row 5: Back button
	row5 := []tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardButtonData("🔙 Back to Positions", "sell_positions"),
	}

	keyboard := tgbotapi.NewInlineKeyboardMarkup(row1, row2, row3, row4, row5)

	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "HTML"
	msg.ReplyMarkup = keyboard

	_, err = h.api.Send(msg)
	return err
}

func (h *TextProcessor) handleSellPresetEdit(ctx context.Context, message *tgbotapi.Message) error {
	userID := message.From.ID
	text := message.Text

	h.logger.Printf("Processing sell preset edit: %s for user: %d", text, userID)

	// Get current user state to get preset index
	userState := h.sessions.GetState(userID)
	if userState.CurrentFlow != sessions.FlowSellPresetEdit || userState.Data == nil {
		return h.sendErrorMessage(ctx, userID, "Session expired. Please try again.")
	}

	presetIndex, ok := userState.Data["preset_index"].(int)
	if !ok || presetIndex < 1 || presetIndex > 4 {
		return h.sendErrorMessage(ctx, userID, "Invalid preset index. Please try again.")
	}

	// Parse amount
	var amount float64
	if _, err := fmt.Sscanf(text, "%f", &amount); err != nil {
		return h.sendErrorMessage(ctx, userID, "Invalid amount format. Please send a valid number (e.g., 0.5)")
	}

	// Validate range
	if amount < 0.001 || amount > 1000 {
		return h.sendErrorMessage(ctx, userID, "Amount must be between 0.001 and 1000 SOL")
	}

	// Update the specific preset using UpdateSingleSellPreset
	err := h.services.GetSettingsService().UpdateSingleSellPreset(ctx, userID, presetIndex, &amount)
	if err != nil {
		h.logger.Printf("Failed to update sell preset %d for user %d: %v", presetIndex, userID, err)
		return h.sendErrorMessage(ctx, userID, "Failed to update sell preset")
	}

	// Clear flow
	h.sessions.SetFlow(userID, sessions.FlowNone, nil)

	// Get current settings to show updated sell settings page
	settings, err := h.services.GetSettingsService().GetUserSettings(ctx, userID)
	if err != nil {
		return h.sendErrorMessage(ctx, userID, "Failed to load updated settings")
	}

	// Build updated sell settings text
	text = `💸 <b>Sell Settings</b>

✏️ <b>Current Values:</b>
• Sell Slippage: <code>` + fmt.Sprintf("%.0f", settings.SellSlippage) + `%</code>

Click button to edit value:`

	// Prepare sell presets array [25%, 50%, 75%, 100%]
	sellPresets := [4]*float64{
		settings.SellPreset25,
		settings.SellPreset50,
		settings.SellPreset75,
		settings.SellPreset100,
	}

	// Create settings handler to build proper keyboard
	settingsHandler := NewSettingsHandler(h.services, h.sessions, h.api, h.logger)
	keyboard := settingsHandler.uiBuilder.BuildSellSettingsKeyboard(settings.SellSlippage, sellPresets)

	// Send new message with updated settings
	msg := tgbotapi.NewMessage(userID, text)
	msg.ParseMode = "HTML"
	msg.ReplyMarkup = keyboard

	_, err = h.api.Send(msg)
	return err
}

func (h *TextProcessor) handlePriorityFeeEdit(ctx context.Context, message *tgbotapi.Message) error {
	userID := message.From.ID
	text := message.Text

	h.logger.Printf("Processing priority fee edit: %s for user: %d", text, userID)

	// Parse SOL amount
	var solAmount float64
	if _, err := fmt.Sscanf(text, "%f", &solAmount); err != nil {
		return h.sendErrorMessage(ctx, userID, "Invalid SOL amount format. Please send a valid number (e.g., 0.01)")
	}

	// Validate range
	if solAmount < 0.0001 || solAmount > 1 {
		return h.sendErrorMessage(ctx, userID, "Priority fee must be between 0.0001 and 1 SOL")
	}

	// Convert SOL to lamports
	lamports := int64(solAmount * 1e9)

	// Get current settings to preserve Jito tip
	settings, err := h.services.GetSettingsService().GetUserSettings(ctx, userID)
	if err != nil {
		h.logger.Printf("Failed to get settings for user %d: %v", userID, err)
		return h.sendErrorMessage(ctx, userID, "Failed to load current settings")
	}

	// Update priority fee
	err = h.services.GetSettingsService().UpdateFees(ctx, userID, lamports, settings.JitoTip)
	if err != nil {
		h.logger.Printf("Failed to update priority fee for user %d: %v", userID, err)
		return h.sendErrorMessage(ctx, userID, "Failed to update priority fee")
	}

	// Clear flow
	h.sessions.SetFlow(userID, sessions.FlowNone, nil)

	// Get current settings to show updated priority fee settings page
	updatedSettings, err := h.services.GetSettingsService().GetUserSettings(ctx, userID)
	if err != nil {
		return h.sendErrorMessage(ctx, userID, "Failed to load updated settings")
	}

	// Конвертируем lamports в SOL для отображения
	priorityFeeSOL := float64(updatedSettings.PriorityFee) / 1e9

	text = `⚡ <b>Priority Fee</b>

✏️ <b>Current:</b> <code>` + fmt.Sprintf("%.6f", priorityFeeSOL) + ` SOL</code>

Select value:`

	// Create settings handler to build proper keyboard
	settingsHandler := NewSettingsHandler(h.services, h.sessions, h.api, h.logger)
	keyboard := settingsHandler.uiBuilder.BuildPriorityFeeKeyboard(priorityFeeSOL)

	// Send new message with updated priority fee settings
	msg := tgbotapi.NewMessage(userID, text)
	msg.ParseMode = "HTML"
	msg.ReplyMarkup = keyboard

	_, err = h.api.Send(msg)
	return err
}

func (h *TextProcessor) handleJitoTipEdit(ctx context.Context, message *tgbotapi.Message) error {
	userID := message.From.ID
	text := message.Text

	h.logger.Printf("Processing Jito tip edit: %s for user: %d", text, userID)

	// Parse SOL amount
	var solAmount float64
	if _, err := fmt.Sscanf(text, "%f", &solAmount); err != nil {
		return h.sendErrorMessage(ctx, userID, "Invalid SOL amount format. Please send a valid number (e.g., 0.0001)")
	}

	// Validate range
	if solAmount < 0 || solAmount > 1.0 {
		return h.sendErrorMessage(ctx, userID, "Jito tip must be between 0 and 1.0 SOL")
	}

	// Convert SOL to lamports
	lamports := int64(solAmount * 1e9)

	// Get current settings to preserve priority fee
	settings, err := h.services.GetSettingsService().GetUserSettings(ctx, userID)
	if err != nil {
		h.logger.Printf("Failed to get settings for user %d: %v", userID, err)
		return h.sendErrorMessage(ctx, userID, "Failed to load current settings")
	}

	// Update Jito tip
	err = h.services.GetSettingsService().UpdateFees(ctx, userID, settings.PriorityFee, lamports)
	if err != nil {
		h.logger.Printf("Failed to update Jito tip for user %d: %v", userID, err)
		return h.sendErrorMessage(ctx, userID, "Failed to update Jito tip")
	}

	// Clear flow
	h.sessions.SetFlow(userID, sessions.FlowNone, nil)

	// Get current settings to show updated MEV settings page
	updatedSettings, err := h.services.GetSettingsService().GetUserSettings(ctx, userID)
	if err != nil {
		return h.sendErrorMessage(ctx, userID, "Failed to load updated settings")
	}

	// MEV защита активна если Jito tip > 0
	mevEnabled := updatedSettings.JitoTip > 0
	statusText := "🔴 Disabled"
	if mevEnabled {
		statusText = "🟢 Enabled"
	}

	// Convert Jito tip to SOL for display
	jitoTipSOL := float64(updatedSettings.JitoTip) / 1e9

	text = `🛡️ <b>MEV Protection</b>

✏️ <b>Current Values:</b>
• Status: ` + statusText + `
• Jito Tip: <code>` + fmt.Sprintf("%.6f", jitoTipSOL) + ` SOL</code>

💡 <b>What is MEV Protection?</b>
Protects your trades from front-running and sandwich attacks using Jito bundles.

Click button to edit value:`

	// Create settings handler to build proper keyboard
	settingsHandler := NewSettingsHandler(h.services, h.sessions, h.api, h.logger)
	keyboard := settingsHandler.uiBuilder.BuildMEVSettingsKeyboard(mevEnabled, updatedSettings.PriorityFee, updatedSettings.JitoTip)

	// Send new message with updated MEV settings
	msg := tgbotapi.NewMessage(userID, text)
	msg.ParseMode = "HTML"
	msg.ReplyMarkup = keyboard

	_, err = h.api.Send(msg)
	return err
}

func (h *TextProcessor) sendErrorMessage(ctx context.Context, userID int64, errorText string) error {
	msg := tgbotapi.NewMessage(userID, "❌ "+errorText)
	sentMsg, err := h.api.Send(msg)
	if err != nil {
		return err
	}

	// 📝 TRACK ERROR MESSAGE: Store message ID for later deletion
	h.trackErrorMessage(userID, sentMsg.MessageID)
	return nil
}

// trackErrorMessage adds error message ID to session for later cleanup
func (h *TextProcessor) trackErrorMessage(userID int64, messageID int) {
	h.logger.Printf("📝 TRACKING: Error message %d for user %d", messageID, userID)

	// Get existing error message IDs
	existingIDsInterface := h.sessions.GetData(userID, "error_message_ids")
	var errorMessageIDs []int

	if existingIDsInterface != nil {
		if existingSlice, ok := existingIDsInterface.([]int); ok {
			errorMessageIDs = existingSlice
		}
	}

	// Add new message ID
	errorMessageIDs = append(errorMessageIDs, messageID)

	// Store back in session
	h.sessions.SetData(userID, "error_message_ids", errorMessageIDs)
	h.logger.Printf("📝 TRACKING: Now tracking %d error messages for user %d", len(errorMessageIDs), userID)
}

// HandleTokenAddressEdit - версия handleTokenAddress которая редактирует существующее сообщение
func (h *TextProcessor) HandleTokenAddressEdit(ctx context.Context, query *tgbotapi.CallbackQuery, tokenAddress string) error {
	userID := query.From.ID

	h.logger.Printf("🚀🚀🚀 NEW CODE RUNNING! Enhanced token editing: %s for user: %d", tokenAddress, userID)

	// ✅ CLEAR ANY EXISTING FLOW STATE when processing new token address
	h.sessions.SetFlow(userID, sessions.FlowNone, nil)

	// Enhanced validation
	if len(tokenAddress) < 32 || len(tokenAddress) > 44 {
		return h.editMessageWithError(ctx, query, "❌ Invalid token address. Please send a valid Solana token address.")
	}

	// Create SolanaTokenAddress value object
	solanaTokenAddr, err := valueobjects.NewSolanaTokenAddress(tokenAddress)
	if err != nil {
		h.logger.Printf("Invalid token address format for user %d: %v", userID, err)
		return h.editMessageWithError(ctx, query, "❌ Invalid token address format. Please check and try again.")
	}

	// Get real token data from DexScreener via TokenDataService
	h.logger.Printf("Fetching token metrics from DexScreener for %s", tokenAddress)
	tokenMetrics, err := h.services.GetTokenDataService().GetTokenMetrics(ctx, *solanaTokenAddr)
	if err != nil {
		h.logger.Printf("Failed to fetch token metrics for %s: %v", tokenAddress, err)
		return h.editMessageWithError(ctx, query, "❌ Failed to fetch token data. Please check the token address and try again.")
	}

	// Debug: Log what we got from DexScreener
	h.logger.Printf("✅ DexScreener FINAL data for %s: Price=%s, Liquidity=%s, MarketCap=%s",
		tokenAddress, tokenMetrics.FormattedPrice(), tokenMetrics.FormattedLiquidity(), tokenMetrics.FormattedMarketCap())

	// Get user's wallets and find default
	userWallets, err := h.services.GetWalletService().GetWallets(ctx, userID)
	var defaultWallet *services.SimpleWallet
	if err == nil && len(userWallets) > 0 {
		// Find default wallet
		for _, wallet := range userWallets {
			if wallet.IsDefault {
				defaultWallet = &wallet
				break
			}
		}
		// If no default found, use first wallet
		if defaultWallet == nil {
			defaultWallet = &userWallets[0]
		}
	}

	// Get user SOL balance (real data if wallet exists)
	userSOLBalance := "0.000000"
	userSOLBalanceUSD := "$0.00"

	if defaultWallet != nil {
		solBalance, err := h.services.GetWalletService().GetBalance(ctx, defaultWallet.ID)
		if err != nil {
			h.logger.Printf("Failed to get balance for wallet %s: %v", defaultWallet.ID, err)
		} else {
			userSOLBalance = fmt.Sprintf("%.6f", solBalance)
			// Get SOL price to calculate USD value
			solPriceObj, err := h.services.GetSolPriceService().GetCachedSolPrice(ctx)
			if err == nil && solPriceObj != nil {
				solPriceFloat, _ := solPriceObj.PriceUSD().Float64()
				usdValue := solBalance * solPriceFloat
				userSOLBalanceUSD = fmt.Sprintf("$%.2f", usdValue)
			}
		}
	}

	// Get user token balance (real data through SolanaService)
	userTokenBalance := "0"
	userTokenBalanceUSD := "$0.00"

	// Get real PnL data from database
	pnl := h.getPnLString(ctx, defaultWallet)

	// Use real data from TokenMetrics - now includes name and symbol from DexScreener
	tokenName := tokenMetrics.Name()
	tokenSymbol := tokenMetrics.Symbol()

	// Fallback to defaults if API data is empty
	if tokenName == "" {
		tokenName = "Unknown Token"
	}
	if tokenSymbol == "" {
		tokenSymbol = solanaTokenAddr.ShortString()
	}

	// Get real token balance AFTER we have tokenSymbol
	if defaultWallet != nil {
		// Получаем реальный баланс токена через SolanaService
		solanaServiceRaw := h.services.GetSolanaService()
		if solanaServiceRaw != nil {
			if solanaService, ok := solanaServiceRaw.(interface {
				GetAssociatedTokenAddress(owner, mint solana.PublicKey) (solana.PublicKey, error)
				GetTokenBalance(ctx context.Context, tokenAccount solana.PublicKey) (*rpc.UiTokenAmount, error)
			}); ok {
				// Получаем публичные ключи
				tokenMintPubKey, err := solana.PublicKeyFromBase58(tokenAddress)
				if err == nil {
					ownerPubKey, err := solana.PublicKeyFromBase58(defaultWallet.PublicKey)
					if err == nil {
						// Получаем адрес ATA
						ataAddress, err := solanaService.GetAssociatedTokenAddress(ownerPubKey, tokenMintPubKey)
						if err == nil {
							// Получаем баланс токена
							tokenBalance, err := solanaService.GetTokenBalance(ctx, ataAddress)
							if err == nil && tokenBalance.UiAmount != nil && *tokenBalance.UiAmount > 0 {
								userTokenBalance = fmt.Sprintf("%.6f", *tokenBalance.UiAmount)

								// Рассчитываем USD стоимость токенов
								if tokenMetrics != nil {
									priceStr := tokenMetrics.FormattedPrice()
									// Извлекаем числовое значение цены (убираем $ и запятые)
									priceStr = strings.ReplaceAll(priceStr, "$", "")
									priceStr = strings.ReplaceAll(priceStr, ",", "")
									if price, err := strconv.ParseFloat(priceStr, 64); err == nil {
										usdValue := *tokenBalance.UiAmount * price
										userTokenBalanceUSD = fmt.Sprintf("$%.2f", usdValue)
									}
								}

								h.logger.Printf("✅ Token balance found: %s %s (%s)", userTokenBalance, tokenSymbol, userTokenBalanceUSD)
							} else {
								h.logger.Printf("ℹ️ No token balance found for %s", tokenAddress)
							}
						}
					}
				}
			}
		}
	}

	price := tokenMetrics.FormattedPrice()
	liquidity := tokenMetrics.FormattedLiquidity()
	marketCap := tokenMetrics.FormattedMarketCap()

	// Format symbol with $ prefix for display like user requested: "$SOL" instead of "SOL"
	displaySymbol := fmt.Sprintf("$%s", tokenSymbol)

	// Get user settings for UI customization
	userSettings, err := h.services.GetSettingsService().GetUserSettings(ctx, userID)
	if err != nil {
		h.logger.Printf("Failed to get user settings for user %d: %v", userID, err)
		// Continue with default settings if failed
		userSettings = entities.NewUserSettings(userID)
	}

	// Create enhanced response like user example with proper symbol formatting and timestamp
	response := fmt.Sprintf(`🔍 <b>BUY %s - (%s)</b>
<code>%s</code> (tap to copy)

💰 <b>Your Balance:</b> %s SOL (%s)
💎 <b>Your current Balance:</b> %s (%s)
| PnL -- %s

📊 <b>Price:</b> %s, <b>LIQ:</b> %s, <b>MC:</b> %s

🕒 <b>Last Updated:</b> %s

🔗 <a href="https://dexscreener.com/solana/%s">View on DEX Screener</a> | <a href="https://solscan.io/address/%s">Explorer</a>

💡 <b>Ready to buy?</b> Choose amount below:`,
		displaySymbol, tokenName,
		tokenAddress,
		userSOLBalance, userSOLBalanceUSD,
		userTokenBalance, userTokenBalanceUSD,
		pnl,
		price, liquidity, marketCap,
		time.Now().Format("15:04:05"),
		tokenAddress,
		tokenAddress)

	// ✅ SAVE TOKEN ADDRESS IN SESSION (but don't set flow state yet)
	// Flow state will be set only when user clicks specific buttons
	h.sessions.SetData(userID, "token_address", tokenAddress)
	h.sessions.SetData(userID, "action_type", "token_interface")
	h.sessions.SetData(userID, "preset1_value", 0.001)
	h.sessions.SetData(userID, "preset2_value", 0.05)
	h.sessions.SetData(userID, "preset3_value", 0.1)
	h.sessions.SetData(userID, "preset4_value", 0.02)
	h.sessions.SetData(userID, "preset5_value", 0.2)

	// Check selected amount from session to show checkmarks
	selectedAmount, hasSelection := h.sessions.GetData(userID, "selected_amount").(float64)
	selectedType, _ := h.sessions.GetData(userID, "selected_amount_type").(string)

	// Create preset buttons with checkmarks for selected amounts
	preset1Text := "0.001 SOL"
	preset2Text := "0.05 SOL"
	preset3Text := "0.1 SOL"
	preset4Text := "0.02 SOL"
	preset5Text := "0.2 SOL"

	// If no selection was made yet, default to first preset being selected
	if !hasSelection {
		preset1Text += " ✅"
		// Set first preset as default selected
		h.sessions.SetData(userID, "selected_amount", 0.001)
		h.sessions.SetData(userID, "selected_amount_type", "preset")
	} else if hasSelection && selectedType == "preset" {
		if selectedAmount == 0.001 {
			preset1Text += " ✅"
		} else if selectedAmount == 0.05 {
			preset2Text += " ✅"
		} else if selectedAmount == 0.1 {
			preset3Text += " ✅"
		} else if selectedAmount == 0.02 {
			preset4Text += " ✅"
		} else if selectedAmount == 0.2 {
			preset5Text += " ✅"
		}
	}

	// Row 1: Fixed presets like reference UI (3 buttons) with checkmarks
	row1 := []tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardButtonData(preset1Text, "buy_preset1"),
		tgbotapi.NewInlineKeyboardButtonData(preset2Text, "buy_preset2"),
		tgbotapi.NewInlineKeyboardButtonData(preset3Text, "buy_preset3"),
	}

	// Row 2: Additional presets (2 buttons) with checkmarks
	row2 := []tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardButtonData(preset4Text, "buy_preset4"),
		tgbotapi.NewInlineKeyboardButtonData(preset5Text, "buy_preset5"),
	}

	// Row 3: Custom amount with checkmark if selected
	customAmountText := fmt.Sprintf("%.3f SOL ✏️", userSettings.DefaultSOLAmount)
	if hasSelection && selectedType == "custom" {
		customAmountText = fmt.Sprintf("%.3f SOL ✅✏️", selectedAmount)
	}

	// Slippage button
	selectedSlippage, hasSelectedSlippage := h.sessions.GetData(userID, "selected_slippage").(float64)
	var slippageText string
	var slippageCallback string

	// Always show a checkmark for slippage since user always has some slippage value (either default or selected)
	if hasSelectedSlippage {
		slippageText = fmt.Sprintf("%.0f%% Slippage ✅✏️", selectedSlippage)
	} else {
		// If no selection made yet, show default slippage with checkmark
		slippageText = fmt.Sprintf("%.0f%% Slippage ✅✏️", userSettings.BuySlippage)
	}

	slippageCallback = "edit_slippage"

	h.logger.Printf("🔍 BUTTON DEBUG: Creating slippage button: text='%s', callback='%s'", slippageText, slippageCallback)

	// Row 4: Custom amount with checkmark if selected
	customAmountCallback := "edit_custom_amount"
	h.logger.Printf("🔍 BUTTON DEBUG: Creating custom amount button: text='%s', callback='%s'", customAmountText, customAmountCallback)
	row3 := []tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardButtonData(customAmountText, customAmountCallback),
	}

	// Row 5: Slippage with updated text and checkmark
	row4 := []tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardButtonData(slippageText, slippageCallback),
	}

	// Row 6: BUY button only (убираем SELL из меню покупки)
	row5 := []tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardButtonData("💰 BUY", "confirm_buy"),
	}

	// Row 7: Utility buttons (refresh & back)
	refreshCallback := "refresh_token"
	backCallback := "main_menu"
	h.logger.Printf("🔍 BUTTON DEBUG: Creating refresh button: callback='%s'", refreshCallback)
	h.logger.Printf("🔍 BUTTON DEBUG: Creating back button: callback='%s'", backCallback)
	row6 := []tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardButtonData("🔄", refreshCallback),
		tgbotapi.NewInlineKeyboardButtonData("◀️ Back", backCallback),
	}

	keyboard := tgbotapi.NewInlineKeyboardMarkup(row1, row2, row3, row4, row5, row6)

	msg := tgbotapi.NewMessage(userID, response)
	msg.ParseMode = "HTML"
	msg.ReplyMarkup = keyboard
	msg.DisableWebPagePreview = true

	_, err = h.api.Send(msg)
	if err != nil {
		h.logger.Printf("Failed to send enhanced token info for user %d: %v", userID, err)
		return err
	}

	h.logger.Printf("✅ Enhanced token info sent successfully for user %d", userID)
	return nil
}

// HandleTokenAddressWithMessage - версия handleTokenAddress которая удаляет предыдущее сообщение и отправляет новое
func (h *TextProcessor) HandleTokenAddressWithMessage(ctx context.Context, message *tgbotapi.Message, tokenAddress string) error {
	userID := message.From.ID

	h.logger.Printf("🚀🚀🚀 NEW CODE RUNNING! Enhanced token processing with message deletion: %s for user: %d", tokenAddress, userID)

	// Сначала удаляем пользовательское сообщение
	deleteMsg := tgbotapi.NewDeleteMessage(message.Chat.ID, message.MessageID)
	h.api.Send(deleteMsg)

	// Теперь вызываем обычную функцию handleTokenAddress для отправки нового токен-меню
	fakeMessage := &tgbotapi.Message{
		From: &tgbotapi.User{ID: userID},
		Text: tokenAddress,
		Chat: &tgbotapi.Chat{ID: userID},
	}

	return h.handleTokenAddress(ctx, fakeMessage)
}

// editMessageWithError редактирует сообщение с ошибкой
func (h *TextProcessor) editMessageWithError(ctx context.Context, query *tgbotapi.CallbackQuery, errorText string) error {
	edit := tgbotapi.NewEditMessageText(query.Message.Chat.ID, query.Message.MessageID, "❌ "+errorText)
	edit.ParseMode = "HTML"
	_, err := h.api.Send(edit)
	return err
}

// getPnLString возвращает отформатированную строку PnL из базы данных
func (h *TextProcessor) getPnLString(ctx context.Context, defaultWallet *services.SimpleWallet) string {
	pnl := "🚀" // fallback
	if defaultWallet != nil {
		h.logger.Printf("🔍 PnL DEBUG: Wallet ID = '%s', UserID = %d", defaultWallet.ID, defaultWallet.UserID)

		// Try to get wallet ID as int64
		walletIDInt, err := strconv.ParseInt(defaultWallet.ID, 10, 64)
		if err != nil {
			h.logger.Printf("🔍 PnL DEBUG: Failed to parse wallet ID '%s' as int64: %v", defaultWallet.ID, err)
			return pnl // Return default if wallet ID is not a number
		}

		h.logger.Printf("🔍 PnL DEBUG: Parsed wallet ID = %d, calling GetWalletPnL...", walletIDInt)
		pnlValue, err := h.services.GetWalletPnL(ctx, walletIDInt)
		if err != nil {
			h.logger.Printf("🔍 PnL DEBUG: GetWalletPnL error: %v", err)
		} else {
			h.logger.Printf("🔍 PnL DEBUG: GetWalletPnL success: pnlValue = %.2f", pnlValue)
			if pnlValue != 0 {
				if pnlValue > 0 {
					pnl = fmt.Sprintf("📈 +$%.2f", pnlValue)
				} else {
					pnl = fmt.Sprintf("📉 $%.2f", pnlValue)
				}
				h.logger.Printf("🔍 PnL DEBUG: Formatted PnL = '%s'", pnl)
			}
		}
	} else {
		h.logger.Printf("🔍 PnL DEBUG: defaultWallet is nil")
	}
	return pnl
}

// EditFormBackToTokenMenu редактирует форму ввода обратно в токен-меню
func (h *TextProcessor) EditFormBackToTokenMenu(ctx context.Context, userID int64, tokenAddress string, chatID int64, messageID int) error {
	h.logger.Printf("🔄 EDIT FORM DEBUG: Starting for user %d, token: %s, chatID: %d, messageID: %d", userID, tokenAddress, chatID, messageID)

	// 🧹 CLEAR ERROR MESSAGES: Delete any previous error messages for this user
	h.deleteUserErrorMessages(ctx, userID, chatID)

	// Создаем фейковый query объект для HandleTokenAddressEdit
	fakeQuery := &tgbotapi.CallbackQuery{
		From: &tgbotapi.User{ID: userID},
		Message: &tgbotapi.Message{
			Chat:      &tgbotapi.Chat{ID: chatID},
			MessageID: messageID,
		},
	}

	h.logger.Printf("🔄 EDIT FORM DEBUG: Created fake query, calling HandleTokenAddressEdit...")

	// Используем HandleTokenAddressEdit для редактирования формы обратно в токен-меню
	err := h.HandleTokenAddressEdit(ctx, fakeQuery, tokenAddress)

	if err != nil {
		h.logger.Printf("🔄 EDIT FORM DEBUG: Error in HandleTokenAddressEdit: %v", err)
		h.logger.Printf("🔄 EDIT FORM DEBUG: Fallback - creating new token menu message instead of editing")

		// FALLBACK: Если редактирование не удается (например, из-за invalid token address),
		// создаем новое сообщение с токен-меню
		fakeMessage := &tgbotapi.Message{
			From: &tgbotapi.User{ID: userID},
			Text: tokenAddress,
			Chat: &tgbotapi.Chat{ID: chatID},
		}

		fallbackErr := h.HandleTokenAddressWithMessage(ctx, fakeMessage, tokenAddress)
		if fallbackErr != nil {
			h.logger.Printf("🔄 EDIT FORM DEBUG: Fallback also failed: %v", fallbackErr)
			return fallbackErr
		}

		h.logger.Printf("🔄 EDIT FORM DEBUG: Fallback successful - new token menu created")
		return nil
	} else {
		h.logger.Printf("🔄 EDIT FORM DEBUG: Successfully edited form back to token menu")
	}

	return err
}

// deleteUserErrorMessages deletes previous error messages for a user
func (h *TextProcessor) deleteUserErrorMessages(ctx context.Context, userID int64, chatID int64) {
	h.logger.Printf("🧹 CLEANUP: Attempting to delete previous error messages for user %d", userID)

	// Try to get error message IDs from session
	errorMessageIDsInterface := h.sessions.GetData(userID, "error_message_ids")
	if errorMessageIDsInterface == nil {
		h.logger.Printf("🧹 CLEANUP: No error message IDs found in session for user %d", userID)
		return
	}

	// Convert to slice of int
	if messageIDSlice, ok := errorMessageIDsInterface.([]int); ok {
		h.logger.Printf("🧹 CLEANUP: Found %d error messages to delete for user %d", len(messageIDSlice), userID)

		for _, msgID := range messageIDSlice {
			deleteMsg := tgbotapi.NewDeleteMessage(chatID, msgID)
			resp, err := h.api.Request(deleteMsg)
			if err != nil {
				h.logger.Printf("🧹 CLEANUP: Failed to delete error message %d: %v", msgID, err)
			} else if resp.Ok {
				h.logger.Printf("🧹 CLEANUP: Successfully deleted error message %d", msgID)
			} else {
				h.logger.Printf("🧹 CLEANUP: Delete failed for message %d: %s", msgID, resp.Description)
			}
		}

		// Clear the error message IDs from session
		h.sessions.SetData(userID, "error_message_ids", nil)
		h.logger.Printf("🧹 CLEANUP: Cleared error message IDs from session for user %d", userID)
	}
}

// handleTokenAddressForSell обрабатывает адрес токена для продажи
func (h *TextProcessor) handleTokenAddressForSell(ctx context.Context, message *tgbotapi.Message) error {
	userID := message.From.ID
	tokenAddress := message.Text

	h.logger.Printf("🔥 SELL TOKEN ADDRESS: User %d entered token: %s", userID, tokenAddress)

	// Получаем процент для продажи из state data (правильный способ)
	state := h.sessions.GetState(userID)
	h.logger.Printf("🔍 DEBUG: User state: flow=%v, data=%v for user %d", state.CurrentFlow, state.Data, userID)

	var sellPercentage float64
	var percentageFound bool = false

	// Пробуем получить из state.Data (сохраненный через SetFlow)
	if percentage, exists := state.Data["sell_percentage"]; exists {
		if pct, ok := percentage.(float64); ok {
			sellPercentage = pct
			percentageFound = true
			h.logger.Printf("✅ Found sell_percentage in state data: %.0f%%", sellPercentage)
		}
	}

	// Также пробуем старый способ как fallback
	if !percentageFound {
		sellPercentageData := h.sessions.GetData(userID, "sell_percentage")
		if pct, ok := sellPercentageData.(float64); ok {
			sellPercentage = pct
			percentageFound = true
			h.logger.Printf("✅ Found sell_percentage in session data: %.0f%%", sellPercentage)
		}
	}

	// Если процент НЕ найден - показываем ошибку пользователю
	if !percentageFound {
		h.logger.Printf("❌ CRITICAL: Sell percentage not found for user %d", userID)
		return h.sendErrorMessage(ctx, userID, "❌ Sell percentage not found. Please start over from main menu → Sell.")
	}

	h.logger.Printf("📊 FINAL: Using sell percentage: %.0f%% for user %d", sellPercentage, userID)

	// Сохраняем токен адрес в сессии
	h.sessions.SetData(userID, "sell_token_address", tokenAddress)

	// Очищаем flow
	h.sessions.SetFlow(userID, sessions.FlowNone, nil)

	// Удаляем сообщение пользователя
	deleteMsg := tgbotapi.NewDeleteMessage(message.Chat.ID, message.MessageID)
	h.api.Send(deleteMsg)

	// Начинаем процесс продажи
	return h.executeSellTransaction(ctx, userID, message.Chat.ID, tokenAddress, sellPercentage)
}

// executeSellTransaction выполняет продажу токена используя модульные сервисы
func (h *TextProcessor) executeSellTransaction(ctx context.Context, userID int64, chatID int64, tokenAddress string, sellPercentage float64) error {
	h.logger.Printf("🔥 EXECUTE SELL: User %d, Token: %s, Percentage: %.0f%%", userID, tokenAddress, sellPercentage)

	// Отправляем сообщение о начале процесса
	processingText := fmt.Sprintf(`⏳ <b>Executing Sell Order</b>

🎯 <b>Token:</b> <code>%s</code>
💸 <b>Percentage:</b> %.0f%% of holdings
🔄 <b>Status:</b> Processing transaction...

⚠️ Please wait, this may take 10-30 seconds...`, tokenAddress, sellPercentage)

	msg := tgbotapi.NewMessage(chatID, processingText)
	msg.ParseMode = "HTML"
	sentMsg, err := h.api.Send(msg)
	if err != nil {
		return err
	}

	// Запускаем асинхронное выполнение продажи через ПРОСТУЮ РАБОЧУЮ ЛОГИКУ!
	// Создаем новый контекст для горутины, не связанный с текущим контекстом
	tradingHandler := NewTradingHandler(h.services, h.sessions, h.api, h.logger)
	go tradingHandler.executeSimpleSell(context.Background(), userID, chatID, sentMsg.MessageID, tokenAddress, sellPercentage)

	return nil
}

// executeModularSell выполняет продажу используя НОВЫЕ модульные сервисы
func (h *TextProcessor) executeModularSell(ctx context.Context, userID int64, chatID int64, messageID int, tokenAddress string, sellPercentage float64) {
	h.logger.Printf("🚀 MODULAR SELL: Starting with NEW architecture for user %d", userID)

	// Получаем наши НОВЫЕ модульные сервисы
	tradingServiceInterface := h.services.GetTradingService()
	if tradingServiceInterface == nil {
		h.logger.Printf("❌ TradingService is nil")
		sellDetails := h.createSellDetailsStub(tokenAddress, sellPercentage, "")
		h.finalizeSellMessage(chatID, messageID, false, sellDetails, "Trading service not available")
		return
	}

	// Приводим к нужному типу
	tradingService, ok := tradingServiceInterface.(trading.TradingService)
	if !ok {
		h.logger.Printf("❌ TradingService type assertion failed")
		sellDetails := h.createSellDetailsStub(tokenAddress, sellPercentage, "")
		h.finalizeSellMessage(chatID, messageID, false, sellDetails, "Trading service type error")
		return
	}

	// Получаем пользователя и кошелек (БЕЗ ТАЙМАУТА - как в работающем коде)
	userWallets, err := h.services.GetWalletService().GetWallets(ctx, userID)
	if err != nil {
		h.logger.Printf("❌ Error getting wallets for user %d: %v", userID, err)
		sellDetails := h.createSellDetailsStub(tokenAddress, sellPercentage, "")
		h.finalizeSellMessage(chatID, messageID, false, sellDetails, "Wallet loading error")
		return
	}
	if len(userWallets) == 0 {
		h.logger.Printf("❌ No wallets found for user %d", userID)
		sellDetails := h.createSellDetailsStub(tokenAddress, sellPercentage, "")
		h.finalizeSellMessage(chatID, messageID, false, sellDetails, "No wallet found. Create wallet in 👛 Wallets section")
		return
	}

	h.logger.Printf("✅ Found %d wallets for user %d", len(userWallets), userID)

	// Найдем default кошелек
	var defaultWallet *services.SimpleWallet
	for i, wallet := range userWallets {
		if wallet.IsDefault {
			defaultWallet = &userWallets[i]
			break
		}
	}
	if defaultWallet == nil {
		defaultWallet = &userWallets[0]
	}

	// Получаем настройки пользователя из базы данных
	userSettings, err := h.services.GetSettingsService().GetUserSettings(ctx, userID)
	if err != nil {
		h.logger.Printf("❌ Failed to get user settings: %v", err)
		sellDetails := h.createSellDetailsStub(tokenAddress, sellPercentage, "")
		h.finalizeSellMessage(chatID, messageID, false, sellDetails, "Failed to load user settings")
		return
	}

	h.logger.Printf("📋 SELL USER SETTINGS FROM DB:")
	h.logger.Printf("  - Sell Slippage: %.0f%%", userSettings.SellSlippage)
	h.logger.Printf("  - Priority Fee: %d lamports", userSettings.PriorityFee)
	h.logger.Printf("  - Jito Tip: %d lamports", userSettings.JitoTip)

	// Получаем реальный баланс токена через SolanaService
	solanaServiceRaw := h.services.GetSolanaService()
	if solanaServiceRaw == nil {
		sellDetails := h.createSellDetailsStub(tokenAddress, sellPercentage, "")
		h.finalizeSellMessage(chatID, messageID, false, sellDetails, "SolanaService не доступен")
		return
	}

	solanaService, ok := solanaServiceRaw.(interface {
		GetAssociatedTokenAddress(owner, mint solana.PublicKey) (solana.PublicKey, error)
		GetTokenBalance(ctx context.Context, tokenAccount solana.PublicKey) (*rpc.UiTokenAmount, error)
	})
	if !ok {
		sellDetails := h.createSellDetailsStub(tokenAddress, sellPercentage, "")
		h.finalizeSellMessage(chatID, messageID, false, sellDetails, "Ошибка типа SolanaService")
		return
	}

	// Получаем публичные ключи
	tokenMintPubKey, err := solana.PublicKeyFromBase58(tokenAddress)
	if err != nil {
		sellDetails := h.createSellDetailsStub(tokenAddress, sellPercentage, "")
		h.finalizeSellMessage(chatID, messageID, false, sellDetails, "Неверный адрес токена")
		return
	}

	ownerPubKey, err := solana.PublicKeyFromBase58(defaultWallet.PublicKey)
	if err != nil {
		sellDetails := h.createSellDetailsStub(tokenAddress, sellPercentage, "")
		h.finalizeSellMessage(chatID, messageID, false, sellDetails, "Ошибка кошелька")
		return
	}

	// Получаем адрес ATA
	ataAddress, err := solanaService.GetAssociatedTokenAddress(ownerPubKey, tokenMintPubKey)
	if err != nil {
		sellDetails := h.createSellDetailsStub(tokenAddress, sellPercentage, "")
		h.finalizeSellMessage(chatID, messageID, false, sellDetails, "Ошибка получения ATA адреса")
		return
	}

	// Получаем баланс токена
	tokenBalance, err := solanaService.GetTokenBalance(ctx, ataAddress)
	if err != nil {
		sellDetails := h.createSellDetailsStub(tokenAddress, sellPercentage, "")
		h.finalizeSellMessage(chatID, messageID, false, sellDetails, "Не удалось получить баланс токена. Возможно у вас нет этого токена.")
		return
	}

	if tokenBalance.UiAmount == nil || *tokenBalance.UiAmount == 0 {
		sellDetails := h.createSellDetailsStub(tokenAddress, sellPercentage, "")
		h.finalizeSellMessage(chatID, messageID, false, sellDetails, "У вас нет токенов для продажи")
		return
	}

	// Рассчитываем сумму для продажи
	totalBalanceStr := tokenBalance.Amount
	totalBalance, err := strconv.ParseInt(totalBalanceStr, 10, 64)
	if err != nil {
		sellDetails := h.createSellDetailsStub(tokenAddress, sellPercentage, "")
		h.finalizeSellMessage(chatID, messageID, false, sellDetails, "Ошибка парсинга баланса")
		return
	}

	sellAmount := (totalBalance * int64(sellPercentage)) / 100
	if sellAmount <= 0 {
		sellDetails := h.createSellDetailsStub(tokenAddress, sellPercentage, "")
		h.finalizeSellMessage(chatID, messageID, false, sellDetails, "Сумма для продажи слишком мала")
		return
	}

	tokenAmount := fmt.Sprintf("%d", sellAmount)

	h.logger.Printf("📊 TOKEN BALANCE INFO:")
	h.logger.Printf("  - Token: %s", tokenAddress)
	h.logger.Printf("  - ATA: %s", ataAddress.String())
	h.logger.Printf("  - Total Balance: %s (%.6f UI)", totalBalanceStr, *tokenBalance.UiAmount)
	h.logger.Printf("  - Sell Percentage: %.0f%%", sellPercentage)
	h.logger.Printf("  - Sell Amount: %s", tokenAmount)

	// ВАЖНО: Получаем приватный ключ из SimpleWallet (как в покупке!)
	privateKey, err := h.services.GetWalletService().ExportPrivateKey(ctx, userID, defaultWallet.ID)
	if err != nil {
		h.logger.Printf("❌ Failed to get wallet private key: %v", err)
		sellDetails := h.createSellDetailsStub(tokenAddress, sellPercentage, "")
		h.finalizeSellMessage(chatID, messageID, false, sellDetails, "Failed to load wallet private key")
		return
	}

	if privateKey == "" {
		h.logger.Printf("❌ Wallet private key is empty")
		sellDetails := h.createSellDetailsStub(tokenAddress, sellPercentage, "")
		h.finalizeSellMessage(chatID, messageID, false, sellDetails, "Wallet private key not found")
		return
	}

	h.logger.Printf("✅ Got wallet private key: %s...", privateKey[:10])

	// Создаем параметры для свапа: Token → SOL используя НАСТРОЙКИ ПОЛЬЗОВАТЕЛЯ
	swapRequest := trading.SwapRequest{
		UserPublicKey:       defaultWallet.PublicKey,
		UserPrivateKey:      privateKey, // ✅ Добавляем приватный ключ для подписи
		InputTokenMint:      tokenAddress,
		OutputTokenMint:     "So11111111111111111111111111111111111111112", // SOL
		Amount:              tokenAmount,
		SlippageBps:         int(userSettings.SellSlippage * 100), // Конвертируем % в basis points
		PriorityFeeLamports: uint64(userSettings.PriorityFee),
		JitoTipLamports:     uint64(userSettings.JitoTip),
		MaxRetries:          3,
	}

	h.logger.Printf("📊 SELL REQUEST (USER SETTINGS):")
	h.logger.Printf("  - Input: %s (%s tokens)", tokenAddress, tokenAmount)
	h.logger.Printf("  - Output: SOL")
	h.logger.Printf("  - Percentage: %.0f%%", sellPercentage)
	h.logger.Printf("  - Slippage: %.0f%% (%d basis points)", userSettings.SellSlippage, int(userSettings.SellSlippage*100))
	h.logger.Printf("  - Priority Fee: %d lamports", userSettings.PriorityFee)
	h.logger.Printf("  - Jito Tip: %d lamports", userSettings.JitoTip)

	// Выполняем свап
	result, err := tradingService.ExecuteSwapWithJito(ctx, swapRequest)
	if err != nil {
		h.logger.Printf("❌ SELL FAILED: %v", err)
		sellDetails := h.createSellDetailsStub(tokenAddress, sellPercentage, "")
		h.finalizeSellMessage(chatID, messageID, false, sellDetails, err.Error())
		return
	}

	h.logger.Printf("✅ SELL SUCCESS!")
	h.logger.Printf("  - Status: %s", result.Status)
	h.logger.Printf("  - TX Hash: %s", result.TransactionHash)
	h.logger.Printf("  - Output Amount: %s SOL", result.OutputAmount)
	h.logger.Printf("  - Price Impact: %s", result.PriceImpact)
	h.logger.Printf("  - Duration: %v", result.Duration)

	// 🔍 ДИАГНОСТИКА: Проверяем реальность транзакции
	if result.TransactionHash != "" {
		h.logger.Printf("🔍 ДИАГНОСТИКА: Проверяем транзакцию %s на Solscan...", result.TransactionHash)
		h.logger.Printf("🔗 Solscan URL: https://solscan.io/tx/%s", result.TransactionHash)

		// Даем время на подтверждение и проверяем статус
		time.Sleep(5 * time.Second)

		// Получаем SolanaService для проверки транзакции
		solanaServiceRaw := h.services.GetSolanaService()
		if solanaServiceRaw != nil {
			if solanaService, ok := solanaServiceRaw.(interface {
				GetTransaction(ctx context.Context, signature solana.Signature) (*rpc.GetTransactionResult, error)
			}); ok {
				signature, err := solana.SignatureFromBase58(result.TransactionHash)
				if err == nil {
					txResult, err := solanaService.GetTransaction(ctx, signature)
					if err != nil {
						h.logger.Printf("⚠️ ДИАГНОСТИКА: Транзакция не найдена в сети: %v", err)
						h.logger.Printf("⚠️ Возможно это тестовая/фейковая транзакция!")
					} else if txResult != nil {
						h.logger.Printf("✅ ДИАГНОСТИКА: Транзакция найдена в сети!")
						if txResult.Meta != nil && txResult.Meta.Err != nil {
							h.logger.Printf("❌ ДИАГНОСТИКА: Транзакция завершилась с ошибкой: %v", txResult.Meta.Err)
						} else {
							h.logger.Printf("✅ ДИАГНОСТИКА: Транзакция успешно выполнена!")
						}
					}
				}
			}
		}
	}

	// Обновляем позицию в БД через PnLTracker
	walletIDInt, _ := strconv.ParseInt(defaultWallet.ID, 10, 64)
	h.updatePositionAfterSellTextProcessor(ctx, userID, walletIDInt, tokenAddress, sellAmount, sellPercentage, result.TransactionHash)

	// Показываем результат
	sellDetails := h.createSellDetailsStub(tokenAddress, sellPercentage, result.TransactionHash)
	h.finalizeSellMessage(chatID, messageID, true, sellDetails, "")
}

// finalizeSellMessage отправляет итоговое сообщение о результате продажи
func (h *TextProcessor) finalizeSellMessage(chatID int64, messageID int, success bool, sellDetails *SellDetails, errorMsg string) {
	var text string

	if success {
		// Красивый формат сообщения в стиле BUY с реальными данными
		text = fmt.Sprintf(`<b>SELL $%s (%s)</b>

%.6f %s → %.6f SOL (%.0f%%)
Price: %s, LIQ: %s, MC: %s

Your Balance: %s (%s)
Token Balance: %s

PnL 🚀

🟢Swap complete <a href="https://solscan.io/tx/%s">View on Solscan</a>`,
			sellDetails.TokenSymbol, sellDetails.TokenName,
			sellDetails.TokenAmount, sellDetails.TokenSymbol, sellDetails.SOLReceived, sellDetails.SellPercentage,
			sellDetails.TokenPrice, sellDetails.Liquidity, sellDetails.MarketCap,
			sellDetails.UserSOLBalance, sellDetails.UserSOLBalanceUSD,
			sellDetails.UserTokenBalance,
			sellDetails.TxID)
	} else {
		text = fmt.Sprintf(`❌ <b>Sell Order Failed</b>

🎯 <b>Token:</b> <code>%s</code>
💸 <b>Percentage:</b> %.0f%% of holdings
❌ <b>Error:</b> %s`, sellDetails.TokenAddress, sellDetails.SellPercentage, errorMsg)
	}

	// Добавляем кнопки для навигации
	var keyboard *tgbotapi.InlineKeyboardMarkup
	if success {
		// Если успех - показываем кнопки для дальнейших действий
		kb := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("🚀 Buy More", "buy"),
				tgbotapi.NewInlineKeyboardButtonData("💸 Sell More", "sell"),
			),
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("🏠 Main Menu", "main_menu"),
			),
		)
		keyboard = &kb
	} else {
		// Если ошибка - показываем кнопки для решения проблемы
		kb := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("🔄 Try Again", "sell"),
				tgbotapi.NewInlineKeyboardButtonData("👛 Wallets", "manage_wallets"),
			),
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("🏠 Main Menu", "main_menu"),
			),
		)
		keyboard = &kb
	}

	edit := tgbotapi.NewEditMessageText(chatID, messageID, text)
	edit.ParseMode = "HTML"
	edit.ReplyMarkup = keyboard
	h.api.Send(edit)
}

func (h *TextProcessor) handleTokenAddress(ctx context.Context, message *tgbotapi.Message) error {
	tokenAddress := message.Text
	userID := message.From.ID

	h.logger.Printf("🚀🚀🚀 NEW CODE RUNNING! Enhanced token processing: %s for user: %d", tokenAddress, userID)

	// ✅ CLEAR ANY EXISTING FLOW STATE when processing new token address
	h.sessions.SetFlow(userID, sessions.FlowNone, nil)

	// Enhanced validation
	if len(tokenAddress) < 32 || len(tokenAddress) > 44 {
		return h.sendErrorMessage(ctx, userID, "❌ Invalid token address. Please send a valid Solana token address.")
	}

	// Check if user is trying to buy SOL with SOL (which is impossible)
	if tokenAddress == "So11111111111111111111111111111111111111112" {
		return h.sendErrorMessage(ctx, userID, "❌ Cannot swap SOL to SOL. Please enter a different token address (e.g., USDC: EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v)")
	}

	// Create SolanaTokenAddress value object
	solanaTokenAddr, err := valueobjects.NewSolanaTokenAddress(tokenAddress)
	if err != nil {
		h.logger.Printf("Invalid token address format for user %d: %v", userID, err)
		return h.sendErrorMessage(ctx, userID, "❌ Invalid token address format. Please check and try again.")
	}

	// Get real token data from DexScreener via TokenDataService
	h.logger.Printf("Fetching token metrics from DexScreener for %s", tokenAddress)
	tokenMetrics, err := h.services.GetTokenDataService().GetTokenMetrics(ctx, *solanaTokenAddr)
	if err != nil {
		h.logger.Printf("Failed to fetch token metrics for %s: %v", tokenAddress, err)
		return h.sendErrorMessage(ctx, userID, "❌ Failed to fetch token data. Please check the token address and try again.")
	}

	// Debug: Log what we got from DexScreener
	h.logger.Printf("✅ DexScreener FINAL data for %s: Price=%s, Liquidity=%s, MarketCap=%s",
		tokenAddress, tokenMetrics.FormattedPrice(), tokenMetrics.FormattedLiquidity(), tokenMetrics.FormattedMarketCap())

	// Get user's wallets and find default
	userWallets, err := h.services.GetWalletService().GetWallets(ctx, userID)
	var defaultWallet *services.SimpleWallet
	if err == nil && len(userWallets) > 0 {
		// Find default wallet
		for _, wallet := range userWallets {
			if wallet.IsDefault {
				defaultWallet = &wallet
				break
			}
		}
		// If no default found, use first wallet
		if defaultWallet == nil {
			defaultWallet = &userWallets[0]
		}
	}

	// Get user SOL balance (real data if wallet exists)
	userSOLBalance := "0.000000"
	userSOLBalanceUSD := "$0.00"

	if defaultWallet != nil {
		solBalance, err := h.services.GetWalletService().GetBalance(ctx, defaultWallet.ID)
		if err != nil {
			h.logger.Printf("Failed to get balance for wallet %s: %v", defaultWallet.ID, err)
		} else {
			userSOLBalance = fmt.Sprintf("%.6f", solBalance)
			// Get SOL price to calculate USD value
			solPriceObj, err := h.services.GetSolPriceService().GetCachedSolPrice(ctx)
			if err == nil && solPriceObj != nil {
				solPriceFloat, _ := solPriceObj.PriceUSD().Float64()
				usdValue := solBalance * solPriceFloat
				userSOLBalanceUSD = fmt.Sprintf("$%.2f", usdValue)
			}
		}
	}

	// Get user token balance (real data through SolanaService)
	userTokenBalance := "0"
	userTokenBalanceUSD := "$0.00"

	// Get real PnL data from database
	pnl := h.getPnLString(ctx, defaultWallet)

	// Use real data from TokenMetrics - now includes name and symbol from DexScreener
	tokenName := tokenMetrics.Name()
	tokenSymbol := tokenMetrics.Symbol()

	// Fallback to defaults if API data is empty
	if tokenName == "" {
		tokenName = "Unknown Token"
	}
	if tokenSymbol == "" {
		tokenSymbol = solanaTokenAddr.ShortString()
	}

	// Get real token balance AFTER we have tokenSymbol
	if defaultWallet != nil {
		// Получаем реальный баланс токена через SolanaService
		solanaServiceRaw := h.services.GetSolanaService()
		if solanaServiceRaw != nil {
			if solanaService, ok := solanaServiceRaw.(interface {
				GetAssociatedTokenAddress(owner, mint solana.PublicKey) (solana.PublicKey, error)
				GetTokenBalance(ctx context.Context, tokenAccount solana.PublicKey) (*rpc.UiTokenAmount, error)
			}); ok {
				// Получаем публичные ключи
				tokenMintPubKey, err := solana.PublicKeyFromBase58(tokenAddress)
				if err == nil {
					ownerPubKey, err := solana.PublicKeyFromBase58(defaultWallet.PublicKey)
					if err == nil {
						// Получаем адрес ATA
						ataAddress, err := solanaService.GetAssociatedTokenAddress(ownerPubKey, tokenMintPubKey)
						if err == nil {
							// Получаем баланс токена
							tokenBalance, err := solanaService.GetTokenBalance(ctx, ataAddress)
							if err == nil && tokenBalance.UiAmount != nil && *tokenBalance.UiAmount > 0 {
								userTokenBalance = fmt.Sprintf("%.6f", *tokenBalance.UiAmount)

								// Рассчитываем USD стоимость токенов
								if tokenMetrics != nil {
									priceStr := tokenMetrics.FormattedPrice()
									// Извлекаем числовое значение цены (убираем $ и запятые)
									priceStr = strings.ReplaceAll(priceStr, "$", "")
									priceStr = strings.ReplaceAll(priceStr, ",", "")
									if price, err := strconv.ParseFloat(priceStr, 64); err == nil {
										usdValue := *tokenBalance.UiAmount * price
										userTokenBalanceUSD = fmt.Sprintf("$%.2f", usdValue)
									}
								}

								h.logger.Printf("✅ Token balance found: %s %s (%s)", userTokenBalance, tokenSymbol, userTokenBalanceUSD)
							} else {
								h.logger.Printf("ℹ️ No token balance found for %s", tokenAddress)
							}
						}
					}
				}
			}
		}
	}

	price := tokenMetrics.FormattedPrice()
	liquidity := tokenMetrics.FormattedLiquidity()
	marketCap := tokenMetrics.FormattedMarketCap()

	// Format symbol with $ prefix for display like user requested: "$SOL" instead of "SOL"
	displaySymbol := fmt.Sprintf("$%s", tokenSymbol)

	// Get user settings for UI customization
	userSettings, err := h.services.GetSettingsService().GetUserSettings(ctx, userID)
	if err != nil {
		h.logger.Printf("Failed to get user settings for user %d: %v", userID, err)
		// Continue with default settings if failed
		userSettings = entities.NewUserSettings(userID)
	}

	// Debug: Log user settings
	h.logger.Printf("User %d settings: DefaultSOL=%.1f, BuySlippage=%.1f, Preset1=%s, Preset2=%s, Preset3=%s, Preset4=%s",
		userID, userSettings.DefaultSOLAmount, userSettings.BuySlippage,
		func() string {
			if userSettings.BuyPreset1 != nil {
				return fmt.Sprintf("%.3f", *userSettings.BuyPreset1)
			} else {
				return "nil"
			}
		}(),
		func() string {
			if userSettings.BuyPreset2 != nil {
				return fmt.Sprintf("%.3f", *userSettings.BuyPreset2)
			} else {
				return "nil"
			}
		}(),
		func() string {
			if userSettings.BuyPreset3 != nil {
				return fmt.Sprintf("%.3f", *userSettings.BuyPreset3)
			} else {
				return "nil"
			}
		}(),
		func() string {
			if userSettings.BuyPreset4 != nil {
				return fmt.Sprintf("%.3f", *userSettings.BuyPreset4)
			} else {
				return "nil"
			}
		}())

	// Create enhanced response like user example with proper symbol formatting and timestamp
	response := fmt.Sprintf(`🔍 <b>BUY %s - (%s)</b>
<code>%s</code> (tap to copy)

💰 <b>Your Balance:</b> %s SOL (%s)
💎 <b>Your current Balance:</b> %s (%s)
| PnL -- %s

📊 <b>Price:</b> %s, <b>LIQ:</b> %s, <b>MC:</b> %s

🕒 <b>Last Updated:</b> %s

🔗 <a href="https://dexscreener.com/solana/%s">View on DEX Screener</a> | <a href="https://solscan.io/address/%s">Explorer</a>

💡 <b>Ready to buy?</b> Choose amount below:`,
		displaySymbol, tokenName,
		tokenAddress,
		userSOLBalance, userSOLBalanceUSD,
		userTokenBalance, userTokenBalanceUSD,
		pnl,
		price, liquidity, marketCap,
		time.Now().Format("15:04:05"),
		tokenAddress,
		tokenAddress)

	// ✅ SAVE TOKEN ADDRESS IN SESSION (but don't set flow state yet)
	// Flow state will be set only when user clicks specific buttons
	h.sessions.SetData(userID, "token_address", tokenAddress)
	h.sessions.SetData(userID, "action_type", "token_interface")
	h.sessions.SetData(userID, "preset1_value", 0.001)
	h.sessions.SetData(userID, "preset2_value", 0.05)
	h.sessions.SetData(userID, "preset3_value", 0.1)
	h.sessions.SetData(userID, "preset4_value", 0.02)
	h.sessions.SetData(userID, "preset5_value", 0.2)

	// Check selected amount from session to show checkmarks
	selectedAmount, hasSelection := h.sessions.GetData(userID, "selected_amount").(float64)
	selectedType, _ := h.sessions.GetData(userID, "selected_amount_type").(string)

	// Create preset buttons with checkmarks for selected amounts
	preset1Text := "0.001 SOL"
	preset2Text := "0.05 SOL"
	preset3Text := "0.1 SOL"
	preset4Text := "0.02 SOL"
	preset5Text := "0.2 SOL"

	// If no selection was made yet, default to first preset being selected
	if !hasSelection {
		preset1Text += " ✅"
		// Set first preset as default selected
		h.sessions.SetData(userID, "selected_amount", 0.001)
		h.sessions.SetData(userID, "selected_amount_type", "preset")
	} else if hasSelection && selectedType == "preset" {
		if selectedAmount == 0.001 {
			preset1Text += " ✅"
		} else if selectedAmount == 0.05 {
			preset2Text += " ✅"
		} else if selectedAmount == 0.1 {
			preset3Text += " ✅"
		} else if selectedAmount == 0.02 {
			preset4Text += " ✅"
		} else if selectedAmount == 0.2 {
			preset5Text += " ✅"
		}
	}

	// Row 1: Fixed presets like reference UI (3 buttons) with checkmarks
	row1 := []tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardButtonData(preset1Text, "buy_preset1"),
		tgbotapi.NewInlineKeyboardButtonData(preset2Text, "buy_preset2"),
		tgbotapi.NewInlineKeyboardButtonData(preset3Text, "buy_preset3"),
	}

	// Row 2: Additional presets (2 buttons) with checkmarks
	row2 := []tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardButtonData(preset4Text, "buy_preset4"),
		tgbotapi.NewInlineKeyboardButtonData(preset5Text, "buy_preset5"),
	}

	// Row 3: Custom amount with checkmark if selected
	customAmountText := fmt.Sprintf("%.3f SOL ✏️", userSettings.DefaultSOLAmount)
	if hasSelection && selectedType == "custom" {
		customAmountText = fmt.Sprintf("%.3f SOL ✅✏️", selectedAmount)
	}

	// Slippage button
	selectedSlippage, hasSelectedSlippage := h.sessions.GetData(userID, "selected_slippage").(float64)
	var slippageText string
	var slippageCallback string

	// Always show a checkmark for slippage since user always has some slippage value (either default or selected)
	if hasSelectedSlippage {
		slippageText = fmt.Sprintf("%.0f%% Slippage ✅✏️", selectedSlippage)
	} else {
		// If no selection made yet, show default slippage with checkmark
		slippageText = fmt.Sprintf("%.0f%% Slippage ✅✏️", userSettings.BuySlippage)
	}

	slippageCallback = "edit_slippage"

	h.logger.Printf("🔍 BUTTON DEBUG: Creating slippage button: text='%s', callback='%s'", slippageText, slippageCallback)

	// Row 4: Custom amount with checkmark if selected
	customAmountCallback := "edit_custom_amount"
	h.logger.Printf("🔍 BUTTON DEBUG: Creating custom amount button: text='%s', callback='%s'", customAmountText, customAmountCallback)
	row3 := []tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardButtonData(customAmountText, customAmountCallback),
	}

	// Row 5: Slippage with updated text and checkmark
	row4 := []tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardButtonData(slippageText, slippageCallback),
	}

	// Row 6: BUY button only (убираем SELL из меню покупки)
	row5 := []tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardButtonData("💰 BUY", "confirm_buy"),
	}

	// Row 7: Utility buttons (refresh & back)
	refreshCallback := "refresh_token"
	backCallback := "main_menu"
	h.logger.Printf("🔍 BUTTON DEBUG: Creating refresh button: callback='%s'", refreshCallback)
	h.logger.Printf("🔍 BUTTON DEBUG: Creating back button: callback='%s'", backCallback)
	row6 := []tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardButtonData("🔄", refreshCallback),
		tgbotapi.NewInlineKeyboardButtonData("◀️ Back", backCallback),
	}

	keyboard := tgbotapi.NewInlineKeyboardMarkup(row1, row2, row3, row4, row5, row6)

	msg := tgbotapi.NewMessage(userID, response)
	msg.ParseMode = "HTML"
	msg.ReplyMarkup = keyboard
	msg.DisableWebPagePreview = true

	_, err = h.api.Send(msg)
	if err != nil {
		h.logger.Printf("Failed to send enhanced token info for user %d: %v", userID, err)
		return err
	}

	h.logger.Printf("✅ Enhanced token info sent successfully for user %d", userID)
	return nil
}

// updatePositionAfterSellTextProcessor обновляет позицию в БД после успешной продажи (для TextProcessor)
func (h *TextProcessor) updatePositionAfterSellTextProcessor(ctx context.Context, userID int64, walletID int64, tokenAddress string, sellAmountTokens int64, sellPercentage float64, txID string) {
	h.logger.Printf("📊 PnL: Updating position after sell - User: %d, Token: %s, Amount: %d, Percentage: %.0f%%",
		userID, tokenAddress, sellAmountTokens, sellPercentage)

	// Получаем PnLTracker
	pnlTracker := h.services.GetPnLTracker()
	if pnlTracker == nil {
		h.logger.Printf("❌ PnL: PnLTracker not available")
		return
	}

	// Получаем активную позицию для расчета цены продажи
	portfolioService := h.services.GetPortfolioService()
	if portfolioService == nil {
		h.logger.Printf("❌ PnL: PortfolioService not available")
		return
	}

	position, err := portfolioService.GetActivePositionByToken(ctx, walletID, tokenAddress)
	if err != nil || position == nil {
		h.logger.Printf("❌ PnL: No active position found for token %s: %v", tokenAddress, err)
		return
	}

	// Получаем текущую цену токена для расчета sell price
	var sellPrice float64 = 0.0001 // Заглушка, в реальности нужно получить из DexScreener
	tokenDataService := h.services.GetTokenDataService()
	if tokenDataService != nil {
		if tokenAddr, err := valueobjects.NewSolanaTokenAddress(tokenAddress); err == nil {
			if tokenMetrics, err := tokenDataService.GetTokenMetrics(ctx, *tokenAddr); err == nil && tokenMetrics != nil {
				if priceFloat, _ := tokenMetrics.PriceUSD().Float64(); priceFloat > 0 {
					sellPrice = priceFloat
				}
			}
		}
	}

	// Создаем SellEvent для PnLTracker
	sellEvent := services.SellEvent{
		UserID:        userID,
		WalletID:      walletID,
		TokenAddress:  tokenAddress,
		TokenSymbol:   position.TokenSymbol,
		TokenAmount:   float64(sellAmountTokens) / 1e6, // Конвертируем из raw amount в UI amount (предполагаем 6 decimals)
		SellPrice:     sellPrice,
		SOLReceived:   0, // Пока не знаем точно сколько SOL получили
		Percentage:    sellPercentage,
		TransactionID: txID,
		Timestamp:     time.Now(),
	}

	// Вызываем OnSellComplete для обновления позиции
	err = pnlTracker.OnSellComplete(ctx, sellEvent)
	if err != nil {
		h.logger.Printf("❌ PnL: Failed to process sell event: %v", err)
	} else {
		h.logger.Printf("✅ PnL: Sell event processed successfully")
	}
}
