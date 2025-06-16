package trading

import (
	"context"
	"encoding/base64"
	"fmt"
	"time"

	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"

	"cash-farmer/internal/infrastructure/blockchain/config"
	"cash-farmer/internal/infrastructure/blockchain/jito"
	"cash-farmer/internal/infrastructure/blockchain/jupiter"
	solanaService "cash-farmer/internal/infrastructure/blockchain/solana"
)

// TradingService главный сервис для торговых операций
type TradingService interface {
	// ExecuteSwapWithJito выполняет swap через Jupiter + отправка через Jito
	ExecuteSwapWithJito(ctx context.Context, params SwapRequest) (*TradingResult, error)

	// GetSwapQuote получает котировку для свапа
	GetSwapQuote(ctx context.Context, params QuoteParams) (*jupiter.QuoteResponse, error)

	// CheckBalances проверяет балансы SOL и токенов
	CheckBalances(ctx context.Context, userAddress solana.PublicKey, tokenMints []string) (*BalanceInfo, error)

	// GetTipAccounts получает список Jito tip accounts
	GetTipAccounts(ctx context.Context) ([]jito.TipAccount, error)

	// SubmitBundle отправляет бандл через Jito
	SubmitBundle(ctx context.Context, transactions [][]byte, tipAmount uint64) (*jito.BundleResult, error)
}

// SwapRequest параметры для выполнения свапа
type SwapRequest struct {
	UserPublicKey       string `json:"userPublicKey"`
	UserPrivateKey      string `json:"userPrivateKey"` // Добавлен для подписи транзакций
	InputTokenMint      string `json:"inputTokenMint"`
	OutputTokenMint     string `json:"outputTokenMint"`
	Amount              string `json:"amount"`
	SlippageBps         int    `json:"slippageBps"`
	PriorityFeeLamports uint64 `json:"priorityFeeLamports"`
	JitoTipLamports     uint64 `json:"jitoTipLamports"`
	MaxRetries          int    `json:"maxRetries"`
}

// QuoteParams параметры для получения котировки
type QuoteParams struct {
	InputTokenMint  string `json:"inputTokenMint"`
	OutputTokenMint string `json:"outputTokenMint"`
	Amount          string `json:"amount"`
	SlippageBps     int    `json:"slippageBps"`
}

// TradingResult результат торговой операции
type TradingResult struct {
	Status          string              `json:"status"`
	SwapResult      *jupiter.SwapResult `json:"swapResult,omitempty"`
	JitoResult      *jito.BundleResult  `json:"jitoResult,omitempty"`
	TransactionHash string              `json:"transactionHash,omitempty"`
	InputAmount     string              `json:"inputAmount"`
	OutputAmount    string              `json:"outputAmount"`
	PriceImpact     string              `json:"priceImpact"`
	Duration        time.Duration       `json:"duration"`
	ProcessedAt     time.Time           `json:"processedAt"`
	Error           string              `json:"error,omitempty"`
}

// BalanceInfo информация о балансах
type BalanceInfo struct {
	SOLBalance    uint64               `json:"solBalance"`
	TokenBalances map[string]TokenInfo `json:"tokenBalances"`
	ProcessedAt   time.Time            `json:"processedAt"`
}

// TokenInfo информация о токене
type TokenInfo struct {
	Mint     string `json:"mint"`
	Balance  string `json:"balance"`
	Decimals int    `json:"decimals"`
}

// tradingService реализация TradingService
type tradingService struct {
	config         *config.BlockchainConfig
	jitoService    jito.JitoService
	jupiterService jupiter.JupiterService
	solanaService  solanaService.SolanaService
}

// NewTradingService создает новый TradingService
func NewTradingService(cfg *config.BlockchainConfig) (TradingService, error) {
	jitoSvc := jito.NewJitoService(cfg)
	jupiterSvc := jupiter.NewJupiterService(cfg)

	solanaSvc, err := solanaService.NewSolanaService(cfg)
	if err != nil {
		return nil, fmt.Errorf("ошибка создания Solana service: %w", err)
	}

	return &tradingService{
		config:         cfg,
		jitoService:    jitoSvc,
		jupiterService: jupiterSvc,
		solanaService:  solanaSvc,
	}, nil
}

// ExecuteSwapWithJito выполняет полный цикл: Jupiter swap + Jito bundle
func (t *tradingService) ExecuteSwapWithJito(ctx context.Context, params SwapRequest) (*TradingResult, error) {
	startTime := time.Now()

	result := &TradingResult{
		Status:      "processing",
		InputAmount: params.Amount,
		ProcessedAt: time.Now(),
	}

	// Валидация параметров
	if err := t.validateSwapRequest(params); err != nil {
		result.Status = "failed"
		result.Error = fmt.Sprintf("некорректные параметры: %v", err)
		result.Duration = time.Since(startTime)
		return result, err
	}

	// Шаг 1: Создаем параметры для Jupiter
	swapParams := jupiter.SwapParams{
		UserPublicKey:       params.UserPublicKey,
		InputTokenMint:      params.InputTokenMint,
		OutputTokenMint:     params.OutputTokenMint,
		Amount:              params.Amount,
		SlippageBps:         params.SlippageBps,
		PriorityFeeLamports: int64(params.PriorityFeeLamports),
		JitoTipLamports:     int64(params.JitoTipLamports),
		SwapMode:            "ExactIn",
		MaxAccounts:         64,
	}

	// Шаг 2: Получаем swap transaction от Jupiter
	swapResult, err := t.jupiterService.ExecuteSwap(ctx, swapParams)
	if err != nil {
		result.Status = "failed"
		result.Error = fmt.Sprintf("ошибка Jupiter swap: %v", err)
		result.Duration = time.Since(startTime)
		return result, err
	}

	result.SwapResult = swapResult
	result.OutputAmount = swapResult.OutputAmount
	result.PriceImpact = swapResult.PriceImpact

	// Шаг 3: Подписываем транзакцию приватным ключом пользователя
	signedTransaction, err := t.signTransaction(swapResult.SwapTransaction, params.UserPrivateKey)
	if err != nil {
		result.Status = "failed"
		result.Error = fmt.Sprintf("ошибка подписи транзакции: %v", err)
		result.Duration = time.Since(startTime)
		return result, err
	}

	// Шаг 4: Если есть Jito tip, отправляем через Jito
	if params.JitoTipLamports > 0 {
		// Отправляем подписанную транзакцию через Jito
		jitoResult, err := t.submitViaJito(ctx, []string{signedTransaction}, params.JitoTipLamports)
		if err != nil {
			// Fallback: отправляем подписанную транзакцию через Solana RPC (декодируем base64)
			txBytes, decodeErr := base64.StdEncoding.DecodeString(signedTransaction)
			if decodeErr != nil {
				result.Status = "failed"
				result.Error = fmt.Sprintf("ошибка декодирования подписанной транзакции для fallback: %v", decodeErr)
				result.Duration = time.Since(startTime)
				return result, decodeErr
			}
			signature, fallbackErr := t.fallbackToSolanaRPC(ctx, txBytes)
			if fallbackErr != nil {
				result.Status = "failed"
				result.Error = fmt.Sprintf("ошибка Jito и fallback: Jito=%v, Fallback=%v", err, fallbackErr)
				result.Duration = time.Since(startTime)
				return result, fallbackErr
			}

			result.Status = "completed_fallback"
			result.TransactionHash = signature.String()
		} else {
			result.Status = "completed_jito"
			result.JitoResult = jitoResult
			result.TransactionHash = jitoResult.BundleID
		}
	} else {
		// Шаг 5: Отправляем подписанную транзакцию через обычный RPC (декодируем base64)
		txBytes, decodeErr := base64.StdEncoding.DecodeString(signedTransaction)
		if decodeErr != nil {
			result.Status = "failed"
			result.Error = fmt.Sprintf("ошибка декодирования подписанной транзакции: %v", decodeErr)
			result.Duration = time.Since(startTime)
			return result, decodeErr
		}
		signature, err := t.fallbackToSolanaRPC(ctx, txBytes)
		if err != nil {
			result.Status = "failed"
			result.Error = fmt.Sprintf("ошибка отправки транзакции: %v", err)
			result.Duration = time.Since(startTime)
			return result, err
		}

		result.Status = "completed_rpc"
		result.TransactionHash = signature.String()
	}

	result.Duration = time.Since(startTime)

	fmt.Printf("🎯 Trading completed (%s) за %v: %s → %s\n",
		result.Status, result.Duration, params.InputTokenMint, params.OutputTokenMint)

	return result, nil
}

// submitViaJito отправляет транзакции через Jito (принимает уже готовые base64 строки)
func (t *tradingService) submitViaJito(ctx context.Context, transactionStrings []string, tipAmount uint64) (*jito.BundleResult, error) {
	fmt.Printf("🚀 Начинаем отправку через Jito: %d транзакций, tip=%d lamports\n", len(transactionStrings), tipAmount)

	// Получаем tip accounts
	tipAccounts, err := t.jitoService.GetTipAccounts(ctx)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения tip accounts: %w", err)
	}

	if len(tipAccounts) == 0 {
		return nil, fmt.Errorf("нет доступных tip accounts")
	}

	fmt.Printf("✅ Получили %d tip accounts\n", len(tipAccounts))

	// transactionStrings уже в base64 формате от Jupiter, передаем напрямую
	fmt.Printf("📦 Отправляем бандл через Jito с %d транзакциями...\n", len(transactionStrings))
	result, err := t.jitoService.SendBundle(ctx, transactionStrings)
	if err != nil {
		return nil, fmt.Errorf("ошибка отправки бандла через Jito: %w", err)
	}

	if len(result.SuccessfulResults) > 0 {
		fmt.Printf("✅ Jito бандл успешно отправлен: %s\n", result.FastestEndpoint)
		return &result.SuccessfulResults[0], nil
	}

	return nil, fmt.Errorf("Jito бандл не был успешно отправлен")
}

// fallbackToSolanaRPC отправляет транзакцию через обычный Solana RPC
func (t *tradingService) fallbackToSolanaRPC(ctx context.Context, transaction []byte) (solana.Signature, error) {
	return t.solanaService.SendTransaction(ctx, transaction)
}

// GetSwapQuote получает котировку для свапа
func (t *tradingService) GetSwapQuote(ctx context.Context, params QuoteParams) (*jupiter.QuoteResponse, error) {
	quoteRequest := jupiter.QuoteRequest{
		InputMint:           params.InputTokenMint,
		OutputMint:          params.OutputTokenMint,
		Amount:              params.Amount,
		SlippageBps:         params.SlippageBps,
		SwapMode:            "ExactIn",
		OnlyDirectRoutes:    false,
		AsLegacyTransaction: false,
		MaxAccounts:         64,
	}

	return t.jupiterService.GetQuoteWithRetry(ctx, quoteRequest)
}

// CheckBalances проверяет балансы пользователя
func (t *tradingService) CheckBalances(ctx context.Context, userAddress solana.PublicKey, tokenMints []string) (*BalanceInfo, error) {
	balanceInfo := &BalanceInfo{
		TokenBalances: make(map[string]TokenInfo),
		ProcessedAt:   time.Now(),
	}

	// Получаем баланс SOL
	solBalance, err := t.solanaService.GetBalance(ctx, userAddress)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения SOL баланса: %w", err)
	}
	balanceInfo.SOLBalance = solBalance

	// Получаем балансы токенов
	for _, mintStr := range tokenMints {
		mint, err := solana.PublicKeyFromBase58(mintStr)
		if err != nil {
			continue // пропускаем некорректные mint адреса
		}

		// Получаем ATA адрес
		ata, err := t.solanaService.GetAssociatedTokenAddress(userAddress, mint)
		if err != nil {
			continue
		}

		// Проверяем существование ATA
		exists, err := t.solanaService.CheckATAExists(ctx, ata)
		if err != nil || !exists {
			balanceInfo.TokenBalances[mintStr] = TokenInfo{
				Mint:     mintStr,
				Balance:  "0",
				Decimals: 9, // default
			}
			continue
		}

		// Получаем баланс токена
		tokenBalance, err := t.solanaService.GetTokenBalance(ctx, ata)
		if err != nil {
			continue
		}

		balanceInfo.TokenBalances[mintStr] = TokenInfo{
			Mint:     mintStr,
			Balance:  tokenBalance.Amount,
			Decimals: int(tokenBalance.Decimals),
		}
	}

	return balanceInfo, nil
}

// GetTipAccounts получает список Jito tip accounts
func (t *tradingService) GetTipAccounts(ctx context.Context) ([]jito.TipAccount, error) {
	return t.jitoService.GetTipAccounts(ctx)
}

// SubmitBundle отправляет бандл через Jito (конвертирует [][]byte в []string)
func (t *tradingService) SubmitBundle(ctx context.Context, transactions [][]byte, tipAmount uint64) (*jito.BundleResult, error) {
	// Конвертируем [][]byte в []string (base64)
	transactionStrings := make([]string, len(transactions))
	for i, tx := range transactions {
		transactionStrings[i] = base64.StdEncoding.EncodeToString(tx)
	}

	return t.submitViaJito(ctx, transactionStrings, tipAmount)
}

// validateSwapRequest валидирует параметры запроса
func (t *tradingService) validateSwapRequest(params SwapRequest) error {
	if params.UserPublicKey == "" {
		return fmt.Errorf("userPublicKey не может быть пустым")
	}

	if !solanaService.IsValidPublicKey(params.UserPublicKey) {
		return fmt.Errorf("некорректный userPublicKey")
	}

	if params.InputTokenMint == "" {
		return fmt.Errorf("inputTokenMint не может быть пустым")
	}

	if params.OutputTokenMint == "" {
		return fmt.Errorf("outputTokenMint не может быть пустым")
	}

	if params.Amount == "" || params.Amount == "0" {
		return fmt.Errorf("amount должен быть больше 0")
	}

	if params.SlippageBps < 0 || params.SlippageBps > 10000 {
		return fmt.Errorf("slippageBps должен быть от 0 до 10000")
	}

	return nil
}

// CreateDefaultSwapRequest создает запрос с дефолтными параметрами
func CreateDefaultSwapRequest(userPubKey, inputMint, outputMint, amount string) SwapRequest {
	return SwapRequest{
		UserPublicKey:       userPubKey,
		InputTokenMint:      inputMint,
		OutputTokenMint:     outputMint,
		Amount:              amount,
		SlippageBps:         100,   // 1% slippage
		PriorityFeeLamports: 50000, // 0.00005 SOL
		JitoTipLamports:     10000, // 0.00001 SOL
		MaxRetries:          3,
	}
}

// FormatTradingResult форматирует результат для вывода
func FormatTradingResult(result *TradingResult) string {
	return fmt.Sprintf(
		"Status: %s | Duration: %v | Input: %s | Output: %s | Impact: %s | Hash: %s",
		result.Status,
		result.Duration,
		result.InputAmount,
		result.OutputAmount,
		result.PriceImpact,
		result.TransactionHash,
	)
}

// signTransaction подписывает транзакцию от Jupiter приватным ключом пользователя
func (t *tradingService) signTransaction(base64Transaction, privateKeyStr string) (string, error) {
	// Декодируем транзакцию из base64
	txBytes, err := base64.StdEncoding.DecodeString(base64Transaction)
	if err != nil {
		return "", fmt.Errorf("ошибка декодирования транзакции: %w", err)
	}

	// Парсим приватный ключ
	privateKey, err := solana.PrivateKeyFromBase58(privateKeyStr)
	if err != nil {
		return "", fmt.Errorf("ошибка парсинга приватного ключа: %w", err)
	}

	// Десериализуем транзакцию
	tx, err := solana.TransactionFromDecoder(bin.NewBinDecoder(txBytes))
	if err != nil {
		return "", fmt.Errorf("ошибка десериализации транзакции: %w", err)
	}

	// Подписываем транзакцию
	_, err = tx.Sign(func(key solana.PublicKey) *solana.PrivateKey {
		if key.Equals(privateKey.PublicKey()) {
			return &privateKey
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("ошибка подписи транзакции: %w", err)
	}

	// Сериализуем обратно в байты
	signedTxBytes, err := tx.MarshalBinary()
	if err != nil {
		return "", fmt.Errorf("ошибка сериализации подписанной транзакции: %w", err)
	}

	// Кодируем в base64
	return base64.StdEncoding.EncodeToString(signedTxBytes), nil
}
