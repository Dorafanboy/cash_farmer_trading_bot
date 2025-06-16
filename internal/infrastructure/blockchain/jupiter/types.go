package jupiter

import (
	"fmt"
	"time"

	"github.com/goccy/go-json"
)

// QuoteRequest параметры для получения котировки
type QuoteRequest struct {
	InputMint           string `json:"inputMint"`
	OutputMint          string `json:"outputMint"`
	Amount              string `json:"amount"`
	SlippageBps         int    `json:"slippageBps,omitempty"`
	SwapMode            string `json:"swapMode,omitempty"`
	DexFilter           string `json:"dexFilter,omitempty"`
	ExcludeDexes        string `json:"excludeDexes,omitempty"`
	OnlyDirectRoutes    bool   `json:"onlyDirectRoutes,omitempty"`
	AsLegacyTransaction bool   `json:"asLegacyTransaction,omitempty"`
	PlatformFeeBps      int    `json:"platformFeeBps,omitempty"`
	MaxAccounts         int    `json:"maxAccounts,omitempty"`
}

// QuoteResponse ответ от Jupiter Quote API
type QuoteResponse struct {
	InputMint            string          `json:"inputMint"`
	InAmount             string          `json:"inAmount"`
	OutputMint           string          `json:"outputMint"`
	OutAmount            string          `json:"outAmount"`
	OtherAmountThreshold string          `json:"otherAmountThreshold"`
	SwapMode             string          `json:"swapMode"`
	SlippageBps          int             `json:"slippageBps"`
	PlatformFee          *PlatformFee    `json:"platformFee,omitempty"`
	PriceImpactPct       string          `json:"priceImpactPct"`
	RoutePlan            []RoutePlanStep `json:"routePlan"`
	ContextSlot          int64           `json:"contextSlot,omitempty"`
	TimeTaken            float64         `json:"timeTaken,omitempty"`
}

// PlatformFee информация о комиссии платформы
type PlatformFee struct {
	Amount string `json:"amount"`
	FeeBps int    `json:"feeBps"`
}

// RoutePlanStep шаг в плане маршрута
type RoutePlanStep struct {
	SwapInfo SwapInfo `json:"swapInfo"`
	Percent  int      `json:"percent"`
}

// SwapInfo информация о свапе
type SwapInfo struct {
	AmmKey     string `json:"ammKey"`
	Label      string `json:"label"`
	InputMint  string `json:"inputMint"`
	OutputMint string `json:"outputMint"`
	InAmount   string `json:"inAmount"`
	OutAmount  string `json:"outAmount"`
	FeeAmount  string `json:"feeAmount"`
	FeeMint    string `json:"feeMint"`
}

// SwapRequest параметры для выполнения свапа
type SwapRequest struct {
	QuoteResponse                 QuoteResponse             `json:"quoteResponse"`
	UserPublicKey                 string                    `json:"userPublicKey"`
	WrapAndUnwrapSol              bool                      `json:"wrapAndUnwrapSol"`
	UseSharedAccounts             bool                      `json:"useSharedAccounts,omitempty"`
	FeeAccount                    string                    `json:"feeAccount,omitempty"`
	TrackingAccount               string                    `json:"trackingAccount,omitempty"`
	ComputeUnitPriceMicroLamports *int64                    `json:"computeUnitPriceMicroLamports,omitempty"`
	PriorityLevelWithMaxLamports  *PriorityLevelMaxLamports `json:"priorityLevelWithMaxLamports,omitempty"`
	AsLegacyTransaction           bool                      `json:"asLegacyTransaction,omitempty"`
	UseTokenLedger                bool                      `json:"useTokenLedger,omitempty"`
	DestinationTokenAccount       string                    `json:"destinationTokenAccount,omitempty"`
}

// PriorityLevelMaxLamports конфигурация priority fees
type PriorityLevelMaxLamports struct {
	PriorityLevel string `json:"priorityLevel"` // "min", "low", "medium", "high", "veryHigh", "unsafeMax"
	MaxLamports   int64  `json:"maxLamports"`
}

// SwapResponse ответ от Jupiter Swap API
type SwapResponse struct {
	SwapTransaction      string               `json:"swapTransaction"`
	LastValidBlockHeight int64                `json:"lastValidBlockHeight"`
	PriorityFeeEstimate  *PriorityFeeEstimate `json:"priorityFeeEstimate,omitempty"`
	ComputeUnitEstimate  *ComputeUnitEstimate `json:"computeUnitEstimate,omitempty"`
}

// PriorityFeeEstimate оценка priority fees
type PriorityFeeEstimate struct {
	PriorityFeeEstimate       float64            `json:"priorityFeeEstimate"`
	PriorityFeeEstimateByTier map[string]float64 `json:"priorityFeeEstimateByTier"`
}

// ComputeUnitEstimate оценка compute units
type ComputeUnitEstimate struct {
	ComputeUnitEstimate int64 `json:"computeUnitEstimate"`
}

// JupiterError ошибка от Jupiter API
type JupiterError struct {
	ErrorCode  string `json:"error"`
	Message    string `json:"message,omitempty"`
	StatusCode int    `json:"statusCode,omitempty"`
}

func (e *JupiterError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("Jupiter API ошибка: %s - %s", e.ErrorCode, e.Message)
	}
	return fmt.Sprintf("Jupiter API ошибка: %s", e.ErrorCode)
}

// SwapResult результат выполнения свапа
type SwapResult struct {
	QuoteResponse   *QuoteResponse `json:"quote_response"`
	SwapTransaction string         `json:"swap_transaction"`
	TransactionHash string         `json:"transaction_hash,omitempty"`
	Status          SwapStatus     `json:"status"`
	Error           string         `json:"error,omitempty"`
	PriceImpact     string         `json:"price_impact"`
	InputAmount     string         `json:"input_amount"`
	OutputAmount    string         `json:"output_amount"`
	SlippageBps     int            `json:"slippage_bps"`
	ProcessedAt     time.Time      `json:"processed_at"`
	Duration        time.Duration  `json:"duration"`
}

// SwapStatus статус свапа
type SwapStatus string

const (
	SwapStatusQuoted    SwapStatus = "quoted"
	SwapStatusPrepared  SwapStatus = "prepared"
	SwapStatusSubmitted SwapStatus = "submitted"
	SwapStatusConfirmed SwapStatus = "confirmed"
	SwapStatusFailed    SwapStatus = "failed"
)

// TokenInfo информация о токене
type TokenInfo struct {
	Address  string `json:"address"`
	Symbol   string `json:"symbol"`
	Name     string `json:"name"`
	Decimals int    `json:"decimals"`
	LogoURI  string `json:"logoURI,omitempty"`
}

// SwapParams параметры для создания свапа (входные от телеграм бота)
type SwapParams struct {
	UserPublicKey       string `json:"user_public_key"`
	InputTokenMint      string `json:"input_token_mint"`      // Адрес входного токена
	OutputTokenMint     string `json:"output_token_mint"`     // Адрес выходного токена
	Amount              string `json:"amount"`                // Количество для свапа
	SlippageBps         int    `json:"slippage_bps"`          // Slippage в basis points
	PriorityFeeLamports int64  `json:"priority_fee_lamports"` // Priority fee
	JitoTipLamports     int64  `json:"jito_tip_lamports"`     // Jito tip
	SwapMode            string `json:"swap_mode"`             // "ExactIn" или "ExactOut"
	MaxAccounts         int    `json:"max_accounts"`          // Максимум аккаунтов в транзакции
}

// DefaultSwapParams возвращает параметры свапа по умолчанию
func DefaultSwapParams() SwapParams {
	return SwapParams{
		SlippageBps:         1500,  // 1.5%
		PriorityFeeLamports: 10000, // 0.00001 SOL
		JitoTipLamports:     10000, // 0.00001 SOL
		SwapMode:            "ExactIn",
		MaxAccounts:         64,
	}
}

// CreateQuoteURL создает URL для получения котировки
func CreateQuoteURL(baseURL string, params QuoteRequest) string {
	url := fmt.Sprintf("%s/quote?inputMint=%s&outputMint=%s&amount=%s",
		baseURL, params.InputMint, params.OutputMint, params.Amount)

	if params.SlippageBps > 0 {
		url += fmt.Sprintf("&slippageBps=%d", params.SlippageBps)
	}

	if params.SwapMode != "" {
		url += fmt.Sprintf("&swapMode=%s", params.SwapMode)
	}

	if params.OnlyDirectRoutes {
		url += "&onlyDirectRoutes=true"
	}

	if params.AsLegacyTransaction {
		url += "&asLegacyTransaction=true"
	}

	if params.MaxAccounts > 0 {
		url += fmt.Sprintf("&maxAccounts=%d", params.MaxAccounts)
	}

	return url
}

// ParseQuoteResponse парсит ответ от Quote API
func ParseQuoteResponse(data []byte) (*QuoteResponse, error) {
	var quote QuoteResponse
	if err := json.Unmarshal(data, &quote); err != nil {
		// Пробуем распарсить как ошибку
		var jupiterErr JupiterError
		if errParseErr := json.Unmarshal(data, &jupiterErr); errParseErr == nil {
			return nil, &jupiterErr
		}
		return nil, fmt.Errorf("ошибка парсинга quote response: %w", err)
	}
	return &quote, nil
}

// ParseSwapResponse парсит ответ от Swap API
func ParseSwapResponse(data []byte) (*SwapResponse, error) {
	var swap SwapResponse
	if err := json.Unmarshal(data, &swap); err != nil {
		// Пробуем распарсить как ошибку
		var jupiterErr JupiterError
		if errParseErr := json.Unmarshal(data, &jupiterErr); errParseErr == nil {
			return nil, &jupiterErr
		}
		return nil, fmt.Errorf("ошибка парсинга swap response: %w", err)
	}
	return &swap, nil
}

// ValidateQuoteResponse проверяет корректность котировки
func (q *QuoteResponse) ValidateQuoteResponse() error {
	if q.InputMint == "" {
		return fmt.Errorf("inputMint не может быть пустым")
	}

	if q.OutputMint == "" {
		return fmt.Errorf("outputMint не может быть пустым")
	}

	if q.InAmount == "" || q.InAmount == "0" {
		return fmt.Errorf("inAmount должен быть больше 0")
	}

	if q.OutAmount == "" || q.OutAmount == "0" {
		return fmt.Errorf("outAmount должен быть больше 0")
	}

	return nil
}

// GetPriceImpactFloat возвращает price impact как float64
func (q *QuoteResponse) GetPriceImpactFloat() (float64, error) {
	if q.PriceImpactPct == "" {
		return 0, nil
	}

	var impact float64
	if err := json.Unmarshal([]byte(q.PriceImpactPct), &impact); err != nil {
		return 0, fmt.Errorf("ошибка парсинга price impact: %w", err)
	}

	return impact, nil
}
