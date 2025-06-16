package jupiter

import (
	"context"
	"fmt"
	"time"

	"cash-farmer/internal/infrastructure/blockchain/common"
	"cash-farmer/internal/infrastructure/blockchain/config"

	"github.com/goccy/go-json"
	"github.com/valyala/fasthttp"
)

// JupiterService интерфейс для работы с Jupiter API
type JupiterService interface {
	// GetQuote получает котировку для свапа
	GetQuote(ctx context.Context, params QuoteRequest) (*QuoteResponse, error)

	// GetSwapTransaction получает транзакцию для свапа
	GetSwapTransaction(ctx context.Context, request SwapRequest) (*SwapResponse, error)

	// ExecuteSwap выполняет полный цикл: quote + swap transaction
	ExecuteSwap(ctx context.Context, params SwapParams) (*SwapResult, error)

	// GetQuoteWithRetry получает котировку с retry логикой
	GetQuoteWithRetry(ctx context.Context, params QuoteRequest) (*QuoteResponse, error)
}

// jupiterService реализация JupiterService
type jupiterService struct {
	config     *config.BlockchainConfig
	httpClient *fasthttp.Client
	baseURL    string
}

// NewJupiterService создает новый Jupiter service
func NewJupiterService(cfg *config.BlockchainConfig) JupiterService {
	return &jupiterService{
		config:  cfg,
		baseURL: cfg.JupiterBaseURL,
		httpClient: &fasthttp.Client{
			ReadTimeout:  cfg.RequestTimeout,
			WriteTimeout: cfg.RequestTimeout,
		},
	}
}

// GetQuote получает котировку для свапа
func (j *jupiterService) GetQuote(ctx context.Context, params QuoteRequest) (*QuoteResponse, error) {
	url := CreateQuoteURL(j.baseURL, params)

	req := fasthttp.AcquireRequest()
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(resp)

	req.SetRequestURI(url)
	req.Header.SetMethod(fasthttp.MethodGet)
	req.Header.SetContentType("application/json")

	err := j.httpClient.DoTimeout(req, resp, j.config.RequestTimeout)
	if err != nil {
		return nil, fmt.Errorf("ошибка HTTP запроса: %w", err)
	}

	if resp.StatusCode() != fasthttp.StatusOK {
		return nil, fmt.Errorf("Jupiter Quote API ошибка %d: %s", resp.StatusCode(), string(resp.Body()))
	}

	// Копируем тело ответа
	body := make([]byte, len(resp.Body()))
	copy(body, resp.Body())

	quote, err := ParseQuoteResponse(body)
	if err != nil {
		return nil, fmt.Errorf("ошибка парсинга котировки: %w", err)
	}

	if err := quote.ValidateQuoteResponse(); err != nil {
		return nil, fmt.Errorf("некорректная котировка: %w", err)
	}

	return quote, nil
}

// GetQuoteWithRetry получает котировку с retry логикой
func (j *jupiterService) GetQuoteWithRetry(ctx context.Context, params QuoteRequest) (*QuoteResponse, error) {
	retryConfig := common.DefaultRetryConfig()

	return common.WithRetryAndResult(ctx, retryConfig, func() (*QuoteResponse, error) {
		return j.GetQuote(ctx, params)
	})
}

// GetSwapTransaction получает транзакцию для свапа
func (j *jupiterService) GetSwapTransaction(ctx context.Context, request SwapRequest) (*SwapResponse, error) {
	url := j.baseURL + "/swap"

	requestBody, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("ошибка маршалинга запроса: %w", err)
	}

	req := fasthttp.AcquireRequest()
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(resp)

	req.SetRequestURI(url)
	req.Header.SetMethod(fasthttp.MethodPost)
	req.Header.SetContentType("application/json")
	req.SetBody(requestBody)

	err = j.httpClient.DoTimeout(req, resp, j.config.RequestTimeout)
	if err != nil {
		return nil, fmt.Errorf("ошибка HTTP запроса: %w", err)
	}

	if resp.StatusCode() != fasthttp.StatusOK {
		return nil, fmt.Errorf("Jupiter Swap API ошибка %d: %s", resp.StatusCode(), string(resp.Body()))
	}

	// Копируем тело ответа
	body := make([]byte, len(resp.Body()))
	copy(body, resp.Body())

	swapResponse, err := ParseSwapResponse(body)
	if err != nil {
		return nil, fmt.Errorf("ошибка парсинга swap response: %w", err)
	}

	return swapResponse, nil
}

// ExecuteSwap выполняет полный цикл: quote + swap transaction
func (j *jupiterService) ExecuteSwap(ctx context.Context, params SwapParams) (*SwapResult, error) {
	startTime := time.Now()

	result := &SwapResult{
		ProcessedAt: time.Now(),
		SlippageBps: params.SlippageBps,
		InputAmount: params.Amount,
		Status:      SwapStatusQuoted,
	}

	// Шаг 1: Получаем котировку
	quoteRequest := QuoteRequest{
		InputMint:           params.InputTokenMint,
		OutputMint:          params.OutputTokenMint,
		Amount:              params.Amount,
		SlippageBps:         params.SlippageBps,
		SwapMode:            params.SwapMode,
		OnlyDirectRoutes:    false,
		AsLegacyTransaction: false,
		MaxAccounts:         params.MaxAccounts,
	}

	quote, err := j.GetQuoteWithRetry(ctx, quoteRequest)
	if err != nil {
		result.Status = SwapStatusFailed
		result.Error = fmt.Sprintf("ошибка получения котировки: %v", err)
		result.Duration = time.Since(startTime)
		return result, err
	}

	result.QuoteResponse = quote
	result.OutputAmount = quote.OutAmount
	result.PriceImpact = quote.PriceImpactPct

	// Шаг 2: Создаем swap request с оптимизированными параметрами
	swapRequest := SwapRequest{
		QuoteResponse:       *quote,
		UserPublicKey:       params.UserPublicKey,
		WrapAndUnwrapSol:    false, // ОПТИМИЗАЦИЯ: избегаем создания Wrapped SOL ATA
		UseSharedAccounts:   true,  // ОПТИМИЗАЦИЯ: используем shared accounts
		AsLegacyTransaction: false,
	}

	// Настройка priority fees в зависимости от параметров
	if params.PriorityFeeLamports > 0 {
		microLamports := params.PriorityFeeLamports * 1000000 // конвертируем в micro lamports
		swapRequest.ComputeUnitPriceMicroLamports = &microLamports
	} else {
		// Используем динамические priority levels для оптимизации
		swapRequest.PriorityLevelWithMaxLamports = &PriorityLevelMaxLamports{
			PriorityLevel: "medium", // Оптимальный баланс скорости и стоимости
			MaxLamports:   100000,   // Максимум 0.0001 SOL
		}
	}

	result.Status = SwapStatusPrepared

	// Шаг 3: Получаем swap transaction
	swapResponse, err := j.GetSwapTransaction(ctx, swapRequest)
	if err != nil {
		result.Status = SwapStatusFailed
		result.Error = fmt.Sprintf("ошибка получения swap transaction: %v", err)
		result.Duration = time.Since(startTime)
		return result, err
	}

	result.SwapTransaction = swapResponse.SwapTransaction
	result.Status = SwapStatusPrepared
	result.Duration = time.Since(startTime)

	fmt.Printf("🎯 Jupiter swap prepared за %v: %s → %s (impact: %s)\n",
		result.Duration,
		j.formatTokenAmount(quote.InAmount, params.InputTokenMint),
		j.formatTokenAmount(quote.OutAmount, params.OutputTokenMint),
		quote.PriceImpactPct)

	return result, nil
}

// formatTokenAmount форматирует количество токенов для вывода
func (j *jupiterService) formatTokenAmount(amount, mint string) string {
	if mint == "So11111111111111111111111111111111111111112" { // SOL
		return amount + " SOL"
	}
	return amount + " tokens"
}

// CreateOptimizedSwapParams создает оптимизированные параметры свапа
func (j *jupiterService) CreateOptimizedSwapParams(
	userPublicKey, inputMint, outputMint, amount string,
	slippageBps int) SwapParams {

	params := DefaultSwapParams()
	params.UserPublicKey = userPublicKey
	params.InputTokenMint = inputMint
	params.OutputTokenMint = outputMint
	params.Amount = amount
	params.SlippageBps = slippageBps

	// Оптимизации для cost efficiency (из наших предыдущих исследований)
	params.PriorityFeeLamports = 50000 // Средний priority fee вместо высокого
	params.JitoTipLamports = 10000     // Минимальный Jito tip
	params.MaxAccounts = 64            // Лимит аккаунтов для предсказуемости

	return params
}

// GetOptimizedQuote получает котировку с оптимизированными параметрами
func (j *jupiterService) GetOptimizedQuote(ctx context.Context,
	inputMint, outputMint, amount string, slippageBps int) (*QuoteResponse, error) {

	quoteRequest := QuoteRequest{
		InputMint:           inputMint,
		OutputMint:          outputMint,
		Amount:              amount,
		SlippageBps:         slippageBps,
		SwapMode:            "ExactIn",
		OnlyDirectRoutes:    false,
		AsLegacyTransaction: false,
		MaxAccounts:         64,
	}

	return j.GetQuoteWithRetry(ctx, quoteRequest)
}

// ValidateSwapParams проверяет корректность параметров свапа
func ValidateSwapParams(params SwapParams) error {
	if params.UserPublicKey == "" {
		return fmt.Errorf("userPublicKey не может быть пустым")
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
