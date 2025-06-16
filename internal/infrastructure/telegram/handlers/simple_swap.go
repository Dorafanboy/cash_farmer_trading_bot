package handlers

import (
	"context"
	"encoding/base64"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"cash-farmer/internal/application/services"
	"cash-farmer/internal/domain/entities"
	"cash-farmer/internal/domain/valueobjects"
	"cash-farmer/internal/infrastructure/blockchain/jito"

	"github.com/gagliardetto/solana-go"
	associatedtokenaccount "github.com/gagliardetto/solana-go/programs/associated-token-account"
	"github.com/gagliardetto/solana-go/programs/system"
	"github.com/gagliardetto/solana-go/rpc"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/goccy/go-json"
	"github.com/valyala/fasthttp"
)

var (
	simpleRpcURLs = []string{
		"https://mainnet.helius-rpc.com/?api-key=97f4303e-3cbd-4417-8c0f-bfdcf31e3cec",
		"https://rpc.hellomoon.io",
		"https://mainnet.genesysgo.org",
	}
	simpleJitoEndpoints = []string{
		"https://mainnet.block-engine.jito.wtf/api/v1",
		"https://amsterdam.mainnet.block-engine.jito.wtf/api/v1",
		"https://frankfurt.mainnet.block-engine.jito.wtf/api/v1",
		"https://london.mainnet.block-engine.jito.wtf/api/v1",
		"https://singapore.mainnet.block-engine.jito.wtf",
		"https://ny.mainnet.block-engine.jito.wtf/api/v1",
		"https://slc.mainnet.block-engine.jito.wtf/api/v1",
		"https://tokyo.mainnet.block-engine.jito.wtf/api/v1",
	}
	simpleBlockhash       atomic.Value
	simpleFastHttpCli     = &fasthttp.Client{ReadTimeout: 15 * time.Second, WriteTimeout: 15 * time.Second}
	simpleLastRequestTime atomic.Value
)

type BundleStatus struct {
	BundleId           string   `json:"bundle_id"`
	Transactions       []string `json:"transactions"`
	Slot               uint64   `json:"slot"`
	ConfirmationStatus string   `json:"confirmation_status"`
	Err                struct {
		Ok interface{} `json:"Ok"`
	} `json:"err"`
}

type StatusContext struct {
	Slot uint64 `json:"slot"`
}

type BundleStatusResponse struct {
	Context StatusContext  `json:"context"`
	Value   []BundleStatus `json:"value"`
}

// Временная функция-обертка для обратной совместимости
func (h *TradingHandler) finalizeSwapLegacy(query *tgbotapi.CallbackQuery, success bool, tokenAddress string, amount float64, txID string, errorMsg string) {
	if !success {
		// Для ошибок используем минимальные данные
		swapDetails := &SwapDetails{
			TokenAddress:        tokenAddress,
			TokenSymbol:         "TOKEN",
			TokenName:           "Token",
			SolAmount:           amount,
			TokenAmount:         0,
			TokenPrice:          "$0.00",
			Liquidity:           "$0",
			MarketCap:           "$0",
			UserSOLBalance:      "0 SOL",
			UserSOLBalanceUSD:   "$0",
			UserTokenBalance:    "0",
			UserTokenBalanceUSD: "$0",
			PnL:                 "🚀",
			TxID:                txID,
		}
		h.finalizeSwap(query, success, swapDetails, errorMsg)
		return
	}

	// Для успешных покупок получаем реальные данные о токене
	userID := query.From.ID

	// Получаем данные о токене из DexScreener
	h.logger.Printf("🔍 Getting token data for successful purchase...")

	// Получаем TokenDataService для получения данных о токене
	tokenDataService := h.services.GetTokenDataService()
	if tokenDataService == nil {
		h.logger.Printf("⚠️ TokenDataService not available, using basic data")
		// Fallback к базовым данным
		swapDetails := &SwapDetails{
			TokenAddress:        tokenAddress,
			TokenSymbol:         "TOKEN",
			TokenName:           "Token",
			SolAmount:           amount,
			TokenAmount:         0,
			TokenPrice:          "$0.00",
			Liquidity:           "$0",
			MarketCap:           "$0",
			UserSOLBalance:      "0 SOL",
			UserSOLBalanceUSD:   "$0",
			UserTokenBalance:    "0",
			UserTokenBalanceUSD: "$0",
			PnL:                 "🚀",
			TxID:                txID,
		}
		h.finalizeSwap(query, success, swapDetails, errorMsg)
		return
	}

	// Получаем данные о токене
	ctx := context.Background()
	tokenAddr, err := valueobjects.NewSolanaTokenAddress(tokenAddress)
	if err != nil {
		h.logger.Printf("⚠️ Invalid token address: %v", err)
		// Fallback к базовым данным
		swapDetails := &SwapDetails{
			TokenAddress:        tokenAddress,
			TokenSymbol:         "TOKEN",
			TokenName:           "Token",
			SolAmount:           amount,
			TokenAmount:         0,
			TokenPrice:          "$0.00",
			Liquidity:           "$0",
			MarketCap:           "$0",
			UserSOLBalance:      "0 SOL",
			UserSOLBalanceUSD:   "$0",
			UserTokenBalance:    "0",
			UserTokenBalanceUSD: "$0",
			PnL:                 "🚀",
			TxID:                txID,
		}
		h.finalizeSwap(query, success, swapDetails, errorMsg)
		return
	}

	tokenMetrics, err := tokenDataService.GetTokenMetrics(ctx, *tokenAddr)
	if err != nil {
		h.logger.Printf("⚠️ Failed to get token data: %v", err)
		// Fallback к базовым данным
		swapDetails := &SwapDetails{
			TokenAddress:        tokenAddress,
			TokenSymbol:         "TOKEN",
			TokenName:           "Token",
			SolAmount:           amount,
			TokenAmount:         0,
			TokenPrice:          "$0.00",
			Liquidity:           "$0",
			MarketCap:           "$0",
			UserSOLBalance:      "0 SOL",
			UserSOLBalanceUSD:   "$0",
			UserTokenBalance:    "0",
			UserTokenBalanceUSD: "$0",
			PnL:                 "🚀",
			TxID:                txID,
		}
		h.finalizeSwap(query, success, swapDetails, errorMsg)
		return
	}

	// Получаем баланс пользователя
	wallets, err := h.services.GetWalletService().GetWallets(ctx, userID)
	if err != nil || len(wallets) == 0 {
		h.logger.Printf("⚠️ Failed to get user wallets for balance")
	}

	var userSOLBalance string = "0.000000 SOL"
	var userSOLBalanceUSD string = "$0.00"

	if len(wallets) > 0 {
		// Находим primary wallet
		var primaryWallet *services.SimpleWallet
		for i, wallet := range wallets {
			if wallet.IsDefault {
				primaryWallet = &wallets[i]
				break
			}
		}
		if primaryWallet == nil {
			primaryWallet = &wallets[0]
		}

		// Получаем баланс SOL
		balance, err := h.services.GetWalletService().GetBalance(ctx, primaryWallet.ID)
		if err == nil {
			userSOLBalance = fmt.Sprintf("%.6f SOL", balance)

			// Получаем цену SOL для USD конвертации
			solPriceService := h.services.GetSolPriceService()
			if solPriceService != nil {
				solPriceData, err := solPriceService.GetCachedSolPrice(ctx)
				if err == nil && solPriceData != nil {
					solPriceFloat, _ := solPriceData.PriceUSD().Float64()
					userSOLBalanceUSD = fmt.Sprintf("$%.2f", balance*solPriceFloat)
				}
			}
		}
	}

	// Форматируем данные токена
	tokenSymbol := tokenMetrics.Symbol()
	if tokenSymbol == "" {
		tokenSymbol = "TOKEN"
	}

	tokenName := tokenMetrics.Name()
	if tokenName == "" {
		tokenName = "Token"
	}

	// Используем готовые форматированные методы
	tokenPrice := tokenMetrics.FormattedPrice()
	liquidity := tokenMetrics.FormattedLiquidity()
	marketCap := tokenMetrics.FormattedMarketCap()

	// Создаем красивые SwapDetails с реальными данными
	swapDetails := &SwapDetails{
		TokenAddress:        tokenAddress,
		TokenSymbol:         tokenSymbol,
		TokenName:           tokenName,
		SolAmount:           amount,
		TokenAmount:         0, // TODO: Рассчитать из amount и price
		TokenPrice:          tokenPrice,
		Liquidity:           liquidity,
		MarketCap:           marketCap,
		UserSOLBalance:      userSOLBalance,
		UserSOLBalanceUSD:   userSOLBalanceUSD,
		UserTokenBalance:    "Loading...", // TODO: Получить реальный баланс токена
		UserTokenBalanceUSD: "$--",
		PnL:                 "🚀",
		TxID:                txID,
	}

	h.logger.Printf("✅ Created enhanced swap details with real token data")
	h.finalizeSwap(query, success, swapDetails, errorMsg)
}

// executeSimpleSwap - рабочая реализация свапа на основе simple_working_main.go
func (h *TradingHandler) executeSimpleSwap(ctx context.Context, query *tgbotapi.CallbackQuery, userID int64, tokenAddress string, amount float64) {
	h.logger.Printf("🚀 SIMPLE SWAP: Starting with PROVEN working logic for user %d", userID)

	// Получаем пользователя и кошелек
	userWallets, err := h.services.GetWalletService().GetWallets(ctx, userID)
	if err != nil {
		h.logger.Printf("❌ Error getting wallets for user %d: %v", userID, err)
		h.finalizeSwapLegacy(query, false, tokenAddress, amount, "", "Wallet loading error")
		return
	}
	if len(userWallets) == 0 {
		h.logger.Printf("❌ No wallets found for user %d", userID)
		h.finalizeSwapLegacy(query, false, tokenAddress, amount, "", "No wallet found. Create wallet in 👛 Wallets section")
		return
	}

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

	// Получаем приватный ключ
	privateKeyStr, err := h.services.GetWalletService().ExportPrivateKey(ctx, userID, defaultWallet.ID)
	if err != nil {
		h.logger.Printf("❌ Failed to get wallet private key: %v", err)
		h.finalizeSwapLegacy(query, false, tokenAddress, amount, "", "Failed to load wallet private key")
		return
	}

	// Парсим приватный ключ
	priv, err := solana.PrivateKeyFromBase58(privateKeyStr)
	if err != nil {
		h.logger.Printf("❌ Failed to parse private key: %v", err)
		h.finalizeSwapLegacy(query, false, tokenAddress, amount, "", "Invalid private key format")
		return
	}

	pub := priv.PublicKey()
	h.logger.Printf("✅ Using wallet: %s", pub.String())

	// Получаем настройки пользователя
	userSettings, err := h.services.GetSettingsService().GetUserSettings(ctx, userID)
	if err != nil {
		h.logger.Printf("❌ Failed to get user settings: %v", err)
		h.finalizeSwapLegacy(query, false, tokenAddress, amount, "", "Failed to load user settings")
		return
	}

	h.logger.Printf("📋 USER SETTINGS:")
	h.logger.Printf("  - Buy Slippage: %.0f%%", userSettings.BuySlippage)
	h.logger.Printf("  - Priority Fee: %d lamports", userSettings.PriorityFee)
	h.logger.Printf("  - Jito Tip: %d lamports", userSettings.JitoTip)

	// Инициализируем RPC клиент и blockhash
	rpcClient := rpc.New(simpleRpcURLs[0])
	go h.simpleRefreshBlockhash(ctx, rpcClient)
	time.Sleep(1 * time.Second) // Ждем получения blockhash

	// Определяем токены для свапа
	inputMint := "So11111111111111111111111111111111111111112" // SOL
	outputMint := tokenAddress

	h.logger.Printf("🔄 Swap: SOL → %s", tokenAddress)
	h.logger.Printf("💰 Amount: %.6f SOL", amount)

	// Получаем tip floor и рассчитываем tip
	tipFloor, err := h.simpleGetTipFloor()
	if err != nil {
		h.logger.Printf("⚠️ Failed to get tip floor, using default: %v", err)
		tipFloor = 0.002
	}

	// Используем настройки пользователя для tip или рассчитываем от tip floor
	var recommendedTip uint64
	if userSettings.JitoTip > 0 {
		recommendedTip = uint64(userSettings.JitoTip)
		h.logger.Printf("🎯 Using user tip setting: %d lamports", recommendedTip)
	} else {
		recommendedTip = uint64(tipFloor * 5 * 1e9) // x5 от 75th percentile
		minTip := uint64(100000)                    // минимум 0.0001 SOL
		maxTip := uint64(500000)                    // максимум 0.0005 SOL
		if recommendedTip < minTip {
			recommendedTip = minTip
		}
		if recommendedTip > maxTip {
			recommendedTip = maxTip
		}
		h.logger.Printf("🎯 Calculated tip: %d lamports (%.6f SOL)", recommendedTip, float64(recommendedTip)/1e9)
	}

	// Проверяем и создаем ATA если нужно
	targetMint := solana.MustPublicKeyFromBase58(outputMint)
	ata, _, err := solana.FindAssociatedTokenAddress(pub, targetMint)
	if err != nil {
		h.logger.Printf("❌ Error finding ATA: %v", err)
		h.finalizeSwapLegacy(query, false, tokenAddress, amount, "", "ATA calculation error")
		return
	}

	_, err = rpcClient.GetAccountInfo(ctx, ata)
	var tx1 *solana.Transaction

	if err != nil {
		h.logger.Printf("🔨 Creating ATA for token: %s", ata.String())
		tx1, err = h.simpleBuildATATransaction(ctx, priv, pub, targetMint)
		if err != nil {
			h.logger.Printf("❌ buildATA error: %v", err)
			h.finalizeSwapLegacy(query, false, tokenAddress, amount, "", "ATA creation error")
			return
		}
	} else {
		h.logger.Printf("✅ ATA already exists: %s", ata.String())
		tx1 = nil
	}

	// Создаем swap транзакцию
	amountLamports := uint64(amount * 1e9)
	tx2, err := h.simpleGetSwapTransaction(ctx, inputMint, outputMint, pub, priv, amountLamports, userSettings)
	if err != nil {
		h.logger.Printf("❌ getSwap error: %v", err)
		h.finalizeSwapLegacy(query, false, tokenAddress, amount, "", "Swap transaction creation error")
		return
	}

	h.logger.Printf("✅ Swap transaction created")

	// Получаем tip accounts
	tipAccountsResponse, successfulEndpoint, err := h.simpleGetTipAccountsAcrossEndpoints()
	if err != nil {
		h.logger.Printf("❌ Failed to get tip accounts: %v", err)
		h.finalizeSwapLegacy(query, false, tokenAddress, amount, "", "Tip accounts unavailable")
		return
	}

	randomTipAccount := tipAccountsResponse[0]
	h.logger.Printf("✅ Tip account from %s: %s", successfulEndpoint, randomTipAccount)

	// Создаем tip транзакцию
	tipTx, err := h.simpleCreateTipTransaction(priv, recommendedTip, simpleBlockhash.Load().(solana.Hash), randomTipAccount)
	if err != nil {
		h.logger.Printf("❌ Tip transaction error: %v", err)
		h.finalizeSwapLegacy(query, false, tokenAddress, amount, "", "Tip transaction creation error")
		return
	}

	// Подготавливаем bundle
	var signed1 []byte
	if tx1 != nil {
		signed1, _ = tx1.MarshalBinary()
	}
	signed2, _ := tx2.MarshalBinary()
	signedTipTx, _ := tipTx.MarshalBinary()

	var bundleRequest [][]string
	if tx1 != nil {
		bundleRequest = [][]string{
			{base64.StdEncoding.EncodeToString(signed1), base64.StdEncoding.EncodeToString(signed2), base64.StdEncoding.EncodeToString(signedTipTx)},
		}
		h.logger.Printf("📦 Bundle with 3 transactions: ATA + Swap + Tip")
	} else {
		bundleRequest = [][]string{
			{base64.StdEncoding.EncodeToString(signed2), base64.StdEncoding.EncodeToString(signedTipTx)},
		}
		h.logger.Printf("📦 Bundle with 2 transactions: Swap + Tip")
	}

	// Отправляем bundle
	h.logger.Printf("🚀 Sending bundle through multiple endpoints...")
	bundleId, usedEndpoint, err := h.simpleSendBundleAcrossEndpoints(bundleRequest)
	if err != nil {
		h.logger.Printf("❌ Failed to send bundle: %v", err)
		h.finalizeSwapLegacy(query, false, tokenAddress, amount, "", "Bundle submission failed")
		return
	}

	h.logger.Printf("✅ Bundle sent successfully via %s. Bundle ID: %s", usedEndpoint, bundleId)
	h.logger.Printf("💰 Tip amount: %.6f SOL", float64(recommendedTip)/1e9)

	// Получаем данные токена для финального сообщения
	h.logger.Printf("🔍 Getting token data for successful purchase...")

	// Получаем данные токена из DexScreener
	ctxToken := context.Background()
	tokenDataService := h.services.GetTokenDataService()
	if tokenDataService == nil {
		h.logger.Printf("⚠️ TokenDataService not available, using legacy finalize")
		h.finalizeSwapLegacy(query, true, tokenAddress, amount, bundleId, "")
		return
	}

	tokenAddr, err := valueobjects.NewSolanaTokenAddress(tokenAddress)
	if err != nil {
		h.logger.Printf("⚠️ Invalid token address, using legacy finalize: %v", err)
		h.finalizeSwapLegacy(query, true, tokenAddress, amount, bundleId, "")
		return
	}

	tokenMetrics, err := tokenDataService.GetTokenMetrics(ctxToken, *tokenAddr)
	if err != nil {
		h.logger.Printf("⚠️ Failed to get token metrics, using legacy finalize: %v", err)
		h.finalizeSwapLegacy(query, true, tokenAddress, amount, bundleId, "")
		return
	}

	// Получаем баланс SOL пользователя
	userSOLBalance := "Loading..."
	userSOLBalanceUSD := "$--"

	walletService := h.services.GetWalletService()
	if walletService != nil {
		balance, err := walletService.GetBalance(ctxToken, defaultWallet.ID)
		if err == nil {
			userSOLBalance = fmt.Sprintf("%.6f SOL", balance)

			// Получаем цену SOL для USD конвертации
			solPriceService := h.services.GetSolPriceService()
			if solPriceService != nil {
				solPriceData, err := solPriceService.GetCachedSolPrice(ctxToken)
				if err == nil && solPriceData != nil {
					solPriceFloat, _ := solPriceData.PriceUSD().Float64()
					userSOLBalanceUSD = fmt.Sprintf("$%.2f", balance*solPriceFloat)
				}
			}
		}
	}

	// Форматируем данные токена
	tokenSymbol := tokenMetrics.Symbol()
	if tokenSymbol == "" {
		tokenSymbol = "TOKEN"
	}

	tokenName := tokenMetrics.Name()
	if tokenName == "" {
		tokenName = "Token"
	}

	// Используем готовые форматированные методы
	tokenPrice := tokenMetrics.FormattedPrice()
	liquidity := tokenMetrics.FormattedLiquidity()
	marketCap := tokenMetrics.FormattedMarketCap()

	// Рассчитываем количество токенов из SOL amount и цены
	var calculatedTokenAmount float64
	if priceFloat, _ := tokenMetrics.PriceUSD().Float64(); priceFloat > 0 {
		// Получаем цену SOL для конвертации
		solPriceService := h.services.GetSolPriceService()
		if solPriceService != nil {
			solPriceData, err := solPriceService.GetCachedSolPrice(ctxToken)
			if err == nil && solPriceData != nil {
				solPriceFloat, _ := solPriceData.PriceUSD().Float64()
				usdAmount := amount * solPriceFloat            // SOL в USD
				calculatedTokenAmount = usdAmount / priceFloat // USD в токены
				h.logger.Printf("💰 CALCULATION: %.6f SOL * $%.2f = $%.6f / $%.8f = %.6f %s",
					amount, solPriceFloat, usdAmount, priceFloat, calculatedTokenAmount, tokenSymbol)
			}
		}
	}

	// Рассчитываем USD стоимость токенов
	var tokenValueUSD float64
	if priceFloat, _ := tokenMetrics.PriceUSD().Float64(); priceFloat > 0 {
		tokenValueUSD = calculatedTokenAmount * priceFloat
	}

	// Получаем реальный баланс токенов пользователя (попытка)
	userTokenBalance := fmt.Sprintf("%.6f %s ($%.2f)", calculatedTokenAmount, tokenSymbol, tokenValueUSD)
	userTokenBalanceUSD := fmt.Sprintf("$%.2f", tokenValueUSD)

	// Пытаемся получить точный баланс из блокчейна
	if walletService != nil {
		// TODO: Добавить метод GetTokenBalance в WalletService
		h.logger.Printf("🔍 Would get token balance for %s from wallet %d", tokenAddress, defaultWallet.ID)
	}

	// Создаем красивые SwapDetails с реальными данными
	swapDetails := &SwapDetails{
		TokenAddress:        tokenAddress,
		TokenSymbol:         tokenSymbol,
		TokenName:           tokenName,
		SolAmount:           amount,
		TokenAmount:         calculatedTokenAmount,
		TokenPrice:          tokenPrice,
		Liquidity:           liquidity,
		MarketCap:           marketCap,
		UserSOLBalance:      userSOLBalance,
		UserSOLBalanceUSD:   userSOLBalanceUSD,
		UserTokenBalance:    userTokenBalance,
		UserTokenBalanceUSD: userTokenBalanceUSD,
		PnL:                 "🚀",
		TxID:                bundleId,
	}

	h.logger.Printf("✅ Created enhanced swap details with real token data")

	// Создаем позицию в портфолио после успешной покупки
	portfolioService := h.services.GetPortfolioService()
	if portfolioService != nil && calculatedTokenAmount > 0 {
		walletIDInt, err := strconv.ParseInt(defaultWallet.ID, 10, 64)
		if err == nil {
			// Получаем цену токена для entry price
			var entryPrice float64
			if priceFloat, _ := tokenMetrics.PriceUSD().Float64(); priceFloat > 0 {
				entryPrice = priceFloat
			}

			openReq := services.OpenPositionRequest{
				WalletID:     walletIDInt,
				TokenAddress: tokenAddress,
				TokenSymbol:  tokenSymbol,
				Amount:       calculatedTokenAmount,
				EntryPrice:   entryPrice,
			}

			position, err := portfolioService.OpenPosition(ctxToken, openReq)
			if err != nil {
				h.logger.Printf("⚠️ Failed to create position: %v", err)
			} else {
				h.logger.Printf("✅ Created position: ID=%d, Token=%s, Amount=%.6f", position.ID, tokenSymbol, calculatedTokenAmount)
			}
		}
	}

	h.finalizeSwap(query, true, swapDetails, "")
}

// executeSimpleSell - рабочая реализация продажи на основе executeSimpleSwap
func (h *TradingHandler) executeSimpleSell(ctx context.Context, userID int64, chatID int64, messageID int, tokenAddress string, sellPercentage float64) {
	h.logger.Printf("🚀 SIMPLE SELL: Starting with PROVEN working logic for user %d", userID)

	// Получаем пользователя и кошелек
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

	// Получаем приватный ключ
	privateKeyStr, err := h.services.GetWalletService().ExportPrivateKey(ctx, userID, defaultWallet.ID)
	if err != nil {
		h.logger.Printf("❌ Failed to get wallet private key: %v", err)
		sellDetails := h.createSellDetailsStub(tokenAddress, sellPercentage, "")
		h.finalizeSellMessage(chatID, messageID, false, sellDetails, "Failed to load wallet private key")
		return
	}

	// Парсим приватный ключ
	priv, err := solana.PrivateKeyFromBase58(privateKeyStr)
	if err != nil {
		h.logger.Printf("❌ Failed to parse private key: %v", err)
		sellDetails := h.createSellDetailsStub(tokenAddress, sellPercentage, "")
		h.finalizeSellMessage(chatID, messageID, false, sellDetails, "Invalid private key format")
		return
	}

	pub := priv.PublicKey()
	h.logger.Printf("✅ Using wallet: %s", pub.String())

	// Получаем настройки пользователя
	userSettings, err := h.services.GetSettingsService().GetUserSettings(ctx, userID)
	if err != nil {
		h.logger.Printf("❌ Failed to get user settings: %v", err)
		sellDetails := h.createSellDetailsStub(tokenAddress, sellPercentage, "")
		h.finalizeSellMessage(chatID, messageID, false, sellDetails, "Failed to load user settings")
		return
	}

	h.logger.Printf("📋 USER SETTINGS:")
	h.logger.Printf("  - Sell Slippage: %.0f%%", userSettings.SellSlippage)
	h.logger.Printf("  - Priority Fee: %d lamports", userSettings.PriorityFee)
	h.logger.Printf("  - Jito Tip: %d lamports", userSettings.JitoTip)

	// Инициализируем RPC клиент и blockhash
	rpcClient := rpc.New(simpleRpcURLs[0])
	go h.simpleRefreshBlockhash(ctx, rpcClient)
	time.Sleep(1 * time.Second) // Ждем получения blockhash

	// Определяем токены для свапа (ПРОДАЖА: Token → SOL)
	inputMint := tokenAddress                                   // Token
	outputMint := "So11111111111111111111111111111111111111112" // SOL

	h.logger.Printf("🔄 Swap: %s → SOL", tokenAddress)
	h.logger.Printf("💸 Percentage: %.0f%%", sellPercentage)

	// Получаем баланс токена для расчета суммы продажи
	tokenMintPubKey, err := solana.PublicKeyFromBase58(tokenAddress)
	if err != nil {
		h.logger.Printf("❌ Invalid token address: %v", err)
		sellDetails := h.createSellDetailsStub(tokenAddress, sellPercentage, "")
		h.finalizeSellMessage(chatID, messageID, false, sellDetails, "Invalid token address")
		return
	}

	// Получаем ATA адрес
	ata, _, err := solana.FindAssociatedTokenAddress(pub, tokenMintPubKey)
	if err != nil {
		h.logger.Printf("❌ Error finding ATA: %v", err)
		sellDetails := h.createSellDetailsStub(tokenAddress, sellPercentage, "")
		h.finalizeSellMessage(chatID, messageID, false, sellDetails, "ATA calculation error")
		return
	}

	// Получаем баланс токена
	tokenBalance, err := rpcClient.GetTokenAccountBalance(ctx, ata, rpc.CommitmentConfirmed)
	if err != nil {
		h.logger.Printf("❌ Failed to get token balance: %v", err)
		sellDetails := h.createSellDetailsStub(tokenAddress, sellPercentage, "")
		h.finalizeSellMessage(chatID, messageID, false, sellDetails, "Failed to get token balance. You may not own this token.")
		return
	}

	if tokenBalance.Value.UiAmount == nil || *tokenBalance.Value.UiAmount == 0 {
		h.logger.Printf("❌ No tokens to sell")
		sellDetails := h.createSellDetailsStub(tokenAddress, sellPercentage, "")
		h.finalizeSellMessage(chatID, messageID, false, sellDetails, "You don't have any tokens to sell")
		return
	}

	// Рассчитываем сумму для продажи
	totalBalanceStr := tokenBalance.Value.Amount
	totalBalance, err := strconv.ParseUint(totalBalanceStr, 10, 64)
	if err != nil {
		h.logger.Printf("❌ Failed to parse token balance: %v", err)
		sellDetails := h.createSellDetailsStub(tokenAddress, sellPercentage, "")
		h.finalizeSellMessage(chatID, messageID, false, sellDetails, "Failed to parse token balance")
		return
	}

	sellAmount := (totalBalance * uint64(sellPercentage)) / 100
	if sellAmount <= 0 {
		h.logger.Printf("❌ Sell amount too small")
		sellDetails := h.createSellDetailsStub(tokenAddress, sellPercentage, "")
		h.finalizeSellMessage(chatID, messageID, false, sellDetails, "Sell amount is too small")
		return
	}

	h.logger.Printf("📊 TOKEN BALANCE INFO:")
	h.logger.Printf("  - Total Balance: %s (%.6f UI)", totalBalanceStr, *tokenBalance.Value.UiAmount)
	h.logger.Printf("  - Sell Amount: %d tokens", sellAmount)

	// Получаем tip floor и рассчитываем tip
	tipFloor, err := h.simpleGetTipFloor()
	if err != nil {
		h.logger.Printf("⚠️ Failed to get tip floor, using default: %v", err)
		tipFloor = 0.002
	}

	// Используем настройки пользователя для tip
	var recommendedTip uint64
	if userSettings.JitoTip > 0 {
		recommendedTip = uint64(userSettings.JitoTip)
		h.logger.Printf("🎯 Using user tip setting: %d lamports", recommendedTip)
	} else {
		recommendedTip = uint64(tipFloor * 5 * 1e9) // x5 от 75th percentile
		minTip := uint64(100000)                    // минимум 0.0001 SOL
		maxTip := uint64(500000)                    // максимум 0.0005 SOL
		if recommendedTip < minTip {
			recommendedTip = minTip
		}
		if recommendedTip > maxTip {
			recommendedTip = maxTip
		}
		h.logger.Printf("🎯 Calculated tip: %d lamports (%.6f SOL)", recommendedTip, float64(recommendedTip)/1e9)
	}

	// Создаем swap транзакцию для продажи
	tx2, err := h.simpleGetSellSwapTransaction(ctx, inputMint, outputMint, pub, priv, sellAmount, userSettings)
	if err != nil {
		h.logger.Printf("❌ getSellSwap error: %v", err)
		sellDetails := h.createSellDetailsStub(tokenAddress, sellPercentage, "")
		h.finalizeSellMessage(chatID, messageID, false, sellDetails, "Sell transaction creation error")
		return
	}

	h.logger.Printf("✅ Sell swap transaction created")

	// Получаем tip accounts
	tipAccountsResponse, successfulEndpoint, err := h.simpleGetTipAccountsAcrossEndpoints()
	if err != nil {
		h.logger.Printf("❌ Failed to get tip accounts: %v", err)
		sellDetails := h.createSellDetailsStub(tokenAddress, sellPercentage, "")
		h.finalizeSellMessage(chatID, messageID, false, sellDetails, "Tip accounts unavailable")
		return
	}

	randomTipAccount := tipAccountsResponse[0]
	h.logger.Printf("✅ Tip account from %s: %s", successfulEndpoint, randomTipAccount)

	// Создаем tip транзакцию
	tipTx, err := h.simpleCreateTipTransaction(priv, recommendedTip, simpleBlockhash.Load().(solana.Hash), randomTipAccount)
	if err != nil {
		h.logger.Printf("❌ Tip transaction error: %v", err)
		sellDetails := h.createSellDetailsStub(tokenAddress, sellPercentage, "")
		h.finalizeSellMessage(chatID, messageID, false, sellDetails, "Tip transaction creation error")
		return
	}

	// Подготавливаем bundle (только Sell + Tip, без ATA так как токен уже есть)
	signed2, _ := tx2.MarshalBinary()
	signedTipTx, _ := tipTx.MarshalBinary()

	bundleRequest := [][]string{
		{base64.StdEncoding.EncodeToString(signed2), base64.StdEncoding.EncodeToString(signedTipTx)},
	}
	h.logger.Printf("📦 Bundle with 2 transactions: Sell + Tip")

	// Отправляем bundle
	h.logger.Printf("🚀 Sending sell bundle through multiple endpoints...")
	bundleId, usedEndpoint, err := h.simpleSendBundleAcrossEndpoints(bundleRequest)
	if err != nil {
		h.logger.Printf("❌ Failed to send bundle: %v", err)
		sellDetails := h.createSellDetailsStub(tokenAddress, sellPercentage, "")
		h.finalizeSellMessage(chatID, messageID, false, sellDetails, "Bundle submission failed")
		return
	}

	h.logger.Printf("✅ Sell bundle sent successfully via %s. Bundle ID: %s", usedEndpoint, bundleId)
	h.logger.Printf("💰 Tip amount: %.6f SOL", float64(recommendedTip)/1e9)

	// Обновляем позицию в БД через PnLTracker
	walletIDInt, _ := strconv.ParseInt(defaultWallet.ID, 10, 64)
	h.updatePositionAfterSell(ctx, userID, walletIDInt, tokenAddress, sellAmount, sellPercentage, bundleId)

	// Показываем пользователю что транзакция отправлена
	sellDetails := h.createSellDetailsStub(tokenAddress, sellPercentage, bundleId)
	h.finalizeSellMessage(chatID, messageID, true, sellDetails, "")
}

// Вспомогательные функции

func (h *TradingHandler) simpleRefreshBlockhash(ctx context.Context, rpcClient *rpc.Client) {
	for {
		if bh, err := rpcClient.GetLatestBlockhash(ctx, rpc.CommitmentProcessed); err == nil {
			simpleBlockhash.Store(bh.Value.Blockhash)
		}
		time.Sleep(400 * time.Millisecond)
	}
}

func (h *TradingHandler) simpleBuildATATransaction(ctx context.Context, payer solana.PrivateKey, owner, mint solana.PublicKey) (*solana.Transaction, error) {
	ix := associatedtokenaccount.NewCreateInstruction(payer.PublicKey(), owner, mint).Build()
	tx, err := solana.NewTransaction(
		[]solana.Instruction{ix},
		simpleBlockhash.Load().(solana.Hash),
		solana.TransactionPayer(payer.PublicKey()),
	)
	if err != nil {
		return nil, err
	}
	_, err = tx.Sign(func(key solana.PublicKey) *solana.PrivateKey {
		if key.Equals(payer.PublicKey()) {
			return &payer
		}
		return nil
	})
	return tx, err
}

func (h *TradingHandler) simpleGetTipFloor() (float64, error) {
	var tipFloor float64

	err := h.simpleRetryWithBackoff(func() error {
		h.simpleEnforceRateLimit()

		// Используем fasthttp правильно
		req := fasthttp.AcquireRequest()
		resp := fasthttp.AcquireResponse()
		defer fasthttp.ReleaseRequest(req)
		defer fasthttp.ReleaseResponse(resp)

		req.SetRequestURI("https://bundles.jito.wtf/api/v1/bundles/tip_floor")
		req.Header.SetMethod(fasthttp.MethodGet)

		err := simpleFastHttpCli.DoTimeout(req, resp, 15*time.Second)
		if err != nil {
			return err
		}

		if resp.StatusCode() == 429 {
			return fmt.Errorf("rate limited (429)")
		}

		body := resp.Body()
		var tipData []struct {
			LandedTips75thPercentile float64 `json:"landed_tips_75th_percentile"`
		}

		if err := json.Unmarshal(body, &tipData); err != nil {
			return err
		}

		if len(tipData) > 0 {
			tipFloor = tipData[0].LandedTips75thPercentile
		} else {
			tipFloor = 0.001
		}

		return nil
	}, 5, 3*time.Second, "getting tip floor")

	if err != nil {
		return 0.002, err
	}

	return tipFloor, nil
}

func (h *TradingHandler) simpleGetSwapTransaction(ctx context.Context, inputMint string, outputMint string, pub solana.PublicKey, priv solana.PrivateKey, amountLamports uint64, userSettings *entities.UserSettings) (*solana.Transaction, error) {
	quoteURL := fmt.Sprintf("https://quote-api.jup.ag/v6/quote?inputMint=%s&outputMint=%s&amount=%d&slippageBps=%d",
		inputMint, outputMint, amountLamports, int(userSettings.BuySlippage*100))

	// GET quote
	req := fasthttp.AcquireRequest()
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(resp)

	req.SetRequestURI(quoteURL)
	req.Header.SetMethod(fasthttp.MethodGet)

	err := simpleFastHttpCli.DoTimeout(req, resp, 15*time.Second)
	if err != nil {
		return nil, err
	}

	respBytes := resp.Body()
	var quoteResp map[string]interface{}
	_ = json.Unmarshal(respBytes, &quoteResp)

	swapReq := map[string]interface{}{
		"quoteResponse":                 quoteResp,
		"userPublicKey":                 pub.String(),
		"wrapAndUnwrapSol":              true,
		"slippageBps":                   int(userSettings.BuySlippage * 100),
		"computeUnitPriceMicroLamports": userSettings.PriorityFee,
		"computeUnitLimit":              1600000,
		"bundleOnly":                    true,
	}

	bodyData, _ := json.Marshal(swapReq)

	// POST swap
	swapReq2 := fasthttp.AcquireRequest()
	swapResp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(swapReq2)
	defer fasthttp.ReleaseResponse(swapResp)

	swapReq2.SetRequestURI("https://quote-api.jup.ag/v6/swap")
	swapReq2.Header.SetMethod(fasthttp.MethodPost)
	swapReq2.Header.SetContentType("application/json")
	swapReq2.SetBody(bodyData)

	err = simpleFastHttpCli.DoTimeout(swapReq2, swapResp, 15*time.Second)
	if err != nil {
		return nil, err
	}

	respBytes = swapResp.Body()
	var swapData struct {
		SwapTransaction string `json:"swapTransaction"`
	}
	_ = json.Unmarshal(respBytes, &swapData)

	var tx solana.Transaction
	_ = tx.UnmarshalBase64(swapData.SwapTransaction)
	_, _ = tx.Sign(func(p solana.PublicKey) *solana.PrivateKey {
		if p.Equals(pub) {
			return &priv
		}
		return nil
	})
	return &tx, nil
}

func (h *TradingHandler) simpleCreateTipTransaction(privateKey solana.PrivateKey, amount uint64, recentBlockhash solana.Hash, tipAddress string) (*solana.Transaction, error) {
	tipAccount, err := solana.PublicKeyFromBase58(tipAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to parse tip account: %v", err)
	}

	tx, err := solana.NewTransaction(
		[]solana.Instruction{
			system.NewTransferInstruction(
				amount,
				privateKey.PublicKey(),
				tipAccount,
			).Build(),
		},
		recentBlockhash,
		solana.TransactionPayer(privateKey.PublicKey()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create tip transaction: %v", err)
	}

	_, err = tx.Sign(func(key solana.PublicKey) *solana.PrivateKey {
		if privateKey.PublicKey().Equals(key) {
			return &privateKey
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to sign tip transaction: %v", err)
	}

	return tx, nil
}

func (h *TradingHandler) simpleGetTipAccountsAcrossEndpoints() ([]string, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	type tipResult struct {
		tipAccounts []string
		endpoint    string
		err         error
	}

	resultChan := make(chan tipResult, len(simpleJitoEndpoints))
	h.logger.Printf("🔄 Getting tip accounts from %d endpoints...", len(simpleJitoEndpoints))

	for i, endpoint := range simpleJitoEndpoints {
		go func(endpoint string, index int) {
			// Используем наш собственный Jito клиент
			jitoClient := jito.NewEnhancedJitoClientFromURL(endpoint)

			var tipAccounts []string
			var lastErr error

			for attempt := 0; attempt < 2; attempt++ {
				select {
				case <-ctx.Done():
					resultChan <- tipResult{nil, endpoint, fmt.Errorf("context canceled")}
					return
				default:
				}

				tipAccountsResponse, err := jitoClient.GetTipAccounts(ctx)
				if err != nil {
					lastErr = err
					continue
				}

				if len(tipAccountsResponse) > 0 {
					tipAccounts = tipAccountsResponse
					break
				}
				lastErr = fmt.Errorf("empty tip accounts")
			}

			resultChan <- tipResult{
				tipAccounts: tipAccounts,
				endpoint:    endpoint,
				err:         lastErr,
			}
		}(endpoint, i)
	}

	for i := 0; i < len(simpleJitoEndpoints); i++ {
		select {
		case res := <-resultChan:
			if res.err == nil && len(res.tipAccounts) > 0 {
				h.logger.Printf("✅ Got tip accounts from: %s", res.endpoint)
				cancel()
				return res.tipAccounts, res.endpoint, nil
			}
		case <-ctx.Done():
			return nil, "", fmt.Errorf("timeout getting tip accounts")
		}
	}

	return nil, "", fmt.Errorf("all endpoints failed for tip accounts")
}

func (h *TradingHandler) simpleSendBundleAcrossEndpoints(bundleRequest [][]string) (string, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	type result struct {
		bundleId string
		endpoint string
		err      error
	}

	resultChan := make(chan result, len(simpleJitoEndpoints))
	h.logger.Printf("🚀 Sending bundle through %d endpoints...", len(simpleJitoEndpoints))

	for i, endpoint := range simpleJitoEndpoints {
		go func(endpoint string, index int) {
			// Используем наш собственный Jito клиент
			jitoClient := jito.NewEnhancedJitoClientFromURL(endpoint)

			bundleId, err := h.simpleSendBundleWithRetryContext(ctx, jitoClient, bundleRequest, endpoint)
			resultChan <- result{
				bundleId: bundleId,
				endpoint: endpoint,
				err:      err,
			}
		}(endpoint, i)
	}

	for i := 0; i < len(simpleJitoEndpoints); i++ {
		select {
		case res := <-resultChan:
			if res.err == nil {
				h.logger.Printf("✅ Bundle sent via: %s", res.endpoint)
				cancel()
				return res.bundleId, res.endpoint, nil
			} else {
				h.logger.Printf("⚠️ Endpoint %s failed: %v", res.endpoint, res.err)
			}
		case <-ctx.Done():
			return "", "", fmt.Errorf("timeout sending bundle")
		}
	}

	return "", "", fmt.Errorf("all endpoints failed")
}

func (h *TradingHandler) simpleSendBundleWithRetryContext(ctx context.Context, jitoClient *jito.EnhancedJitoClient, bundleRequest [][]string, endpoint string) (string, error) {
	var bundleId string
	maxRetries := 3

	for attempt := 0; attempt < maxRetries; attempt++ {
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("context canceled: %v", ctx.Err())
		default:
		}

		// Конвертируем [][]string в []string для нашего API
		transactions := bundleRequest[0] // Берем первый bundle из массива
		bundleResult, err := jitoClient.SendBundle(ctx, transactions)
		if err != nil {
			if strings.Contains(fmt.Sprintf("%v", err), "rate limited") || strings.Contains(fmt.Sprintf("%v", err), "Too Many Requests") {
				if attempt < maxRetries-1 {
					continue
				}
				return "", fmt.Errorf("rate limited")
			}
			if attempt < maxRetries-1 {
				continue
			}
			return "", err
		}

		bundleId = bundleResult.BundleID
		if bundleId == "" {
			if attempt < maxRetries-1 {
				continue
			}
			return "", fmt.Errorf("empty bundle ID")
		}

		return bundleId, nil
	}

	return "", fmt.Errorf("all %d attempts failed for %s", maxRetries, endpoint)
}

func (h *TradingHandler) simpleEnforceRateLimit() {
	simpleLastRequestTime.Store(time.Now())
}

func (h *TradingHandler) simpleRetryWithBackoff(operation func() error, maxRetries int, baseDelay time.Duration, operationName string) error {
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			delay := time.Duration(float64(baseDelay) * math.Pow(2, float64(attempt-1)))
			if delay > 60*time.Second {
				delay = 60 * time.Second
			}
			time.Sleep(delay)
		}

		err := operation()
		if err == nil {
			return nil
		}

		lastErr = err
	}

	return fmt.Errorf("all %d attempts failed for %s: %v", maxRetries, operationName, lastErr)
}

func (h *TradingHandler) simpleGetSellSwapTransaction(ctx context.Context, inputMint string, outputMint string, pub solana.PublicKey, priv solana.PrivateKey, amountTokens uint64, userSettings *entities.UserSettings) (*solana.Transaction, error) {
	quoteURL := fmt.Sprintf("https://quote-api.jup.ag/v6/quote?inputMint=%s&outputMint=%s&amount=%d&slippageBps=%d",
		inputMint, outputMint, amountTokens, int(userSettings.SellSlippage*100))

	// GET quote
	req := fasthttp.AcquireRequest()
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(resp)

	req.SetRequestURI(quoteURL)
	req.Header.SetMethod(fasthttp.MethodGet)

	err := simpleFastHttpCli.DoTimeout(req, resp, 15*time.Second)
	if err != nil {
		return nil, err
	}

	respBytes := resp.Body()
	var quoteResp map[string]interface{}
	_ = json.Unmarshal(respBytes, &quoteResp)

	swapReq := map[string]interface{}{
		"quoteResponse":                 quoteResp,
		"userPublicKey":                 pub.String(),
		"wrapAndUnwrapSol":              true,
		"slippageBps":                   int(userSettings.SellSlippage * 100),
		"computeUnitPriceMicroLamports": userSettings.PriorityFee,
		"computeUnitLimit":              1600000,
		"bundleOnly":                    true,
	}

	bodyData, _ := json.Marshal(swapReq)

	// POST swap
	swapReq2 := fasthttp.AcquireRequest()
	swapResp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(swapReq2)
	defer fasthttp.ReleaseResponse(swapResp)

	swapReq2.SetRequestURI("https://quote-api.jup.ag/v6/swap")
	swapReq2.Header.SetMethod(fasthttp.MethodPost)
	swapReq2.Header.SetContentType("application/json")
	swapReq2.SetBody(bodyData)

	err = simpleFastHttpCli.DoTimeout(swapReq2, swapResp, 15*time.Second)
	if err != nil {
		return nil, err
	}

	respBytes = swapResp.Body()
	var swapData struct {
		SwapTransaction string `json:"swapTransaction"`
	}
	_ = json.Unmarshal(respBytes, &swapData)

	var tx solana.Transaction
	_ = tx.UnmarshalBase64(swapData.SwapTransaction)
	_, _ = tx.Sign(func(p solana.PublicKey) *solana.PrivateKey {
		if p.Equals(pub) {
			return &priv
		}
		return nil
	})
	return &tx, nil
}

func (h *TradingHandler) finalizeSellMessage(chatID int64, messageID int, success bool, sellDetails *SellDetails, errorMsg string) {
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
			tgbotapi.NewInlineKeyboardButtonData("🔄 Try Again", "sell"),
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

	edit := tgbotapi.NewEditMessageText(chatID, messageID, text)
	edit.ParseMode = "HTML"
	edit.ReplyMarkup = keyboard
	h.api.Send(edit)
}

// updatePositionAfterSell обновляет позицию в БД после успешной продажи
func (h *TradingHandler) updatePositionAfterSell(ctx context.Context, userID int64, walletID int64, tokenAddress string, sellAmountTokens uint64, sellPercentage float64, txID string) {
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
