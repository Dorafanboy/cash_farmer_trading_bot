package api

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"cash-farmer/internal/domain/valueobjects"

	"github.com/goccy/go-json"
	"github.com/valyala/fasthttp"
)

// JupiterClient handles communication with Jupiter Swap API
type JupiterClient struct {
	client  *fasthttp.Client
	baseURL string
}

// JupiterPriceResponse represents response from Jupiter price API
type JupiterPriceResponse struct {
	Data map[string]struct {
		Price float64 `json:"price"`
	} `json:"data"`
}

// JupiterQuoteRequest represents request to Jupiter quote API
type JupiterQuoteRequest struct {
	InputMint                  string `json:"inputMint"`
	OutputMint                 string `json:"outputMint"`
	Amount                     string `json:"amount"`
	SlippageBps                int    `json:"slippageBps"`
	SwapMode                   string `json:"swapMode,omitempty"`
	OnlyDirectRoutes           bool   `json:"onlyDirectRoutes,omitempty"`
	AsLegacyTransaction        bool   `json:"asLegacyTransaction,omitempty"`
	RestrictIntermediateTokens bool   `json:"restrictIntermediateTokens,omitempty"`
}

// JupiterQuoteResponse represents response from Jupiter quote API
type JupiterQuoteResponse struct {
	InputMint            string                 `json:"inputMint"`
	InAmount             string                 `json:"inAmount"`
	OutputMint           string                 `json:"outputMint"`
	OutAmount            string                 `json:"outAmount"`
	OtherAmountThreshold string                 `json:"otherAmountThreshold"`
	SwapMode             string                 `json:"swapMode"`
	SlippageBps          int                    `json:"slippageBps"`
	PlatformFee          *JupiterPlatformFee    `json:"platformFee,omitempty"`
	PriceImpactPct       string                 `json:"priceImpactPct"`
	RoutePlan            []JupiterRoutePlanStep `json:"routePlan"`
}

// JupiterPlatformFee represents platform fee structure
type JupiterPlatformFee struct {
	Amount string `json:"amount"`
	FeeBps int    `json:"feeBps"`
}

// JupiterRoutePlanStep represents a step in the route plan
type JupiterRoutePlanStep struct {
	SwapInfo JupiterSwapInfo `json:"swapInfo"`
	Percent  int             `json:"percent"`
}

// JupiterSwapInfo represents swap information
type JupiterSwapInfo struct {
	AmmKey     string `json:"ammKey"`
	Label      string `json:"label"`
	InputMint  string `json:"inputMint"`
	OutputMint string `json:"outputMint"`
	InAmount   string `json:"inAmount"`
	OutAmount  string `json:"outAmount"`
	FeeAmount  string `json:"feeAmount"`
	FeeMint    string `json:"feeMint"`
}

// JupiterSwapRequest represents request to Jupiter swap API (MINIMAL DOCS FORMAT)
type JupiterSwapRequest struct {
	QuoteResponse                 JupiterQuoteResponse `json:"quoteResponse"`
	UserPublicKey                 string               `json:"userPublicKey"`
	WrapAndUnwrapSol              bool                 `json:"wrapAndUnwrapSol,omitempty"`
	UseSharedAccounts             bool                 `json:"useSharedAccounts,omitempty"`
	FeeAccount                    string               `json:"feeAccount,omitempty"`
	ComputeUnitPriceMicroLamports *int                 `json:"computeUnitPriceMicroLamports,omitempty"`
	PrioritizationFeeLamports     interface{}          `json:"prioritizationFeeLamports,omitempty"`
	DynamicComputeUnitLimit       bool                 `json:"dynamicComputeUnitLimit,omitempty"`
	DynamicSlippage               bool                 `json:"dynamicSlippage,omitempty"`
	SkipUserAccountsRpcCalls      bool                 `json:"skipUserAccountsRpcCalls,omitempty"`
	AsLegacyTransaction           bool                 `json:"asLegacyTransaction,omitempty"`
	CreateTokenAccount            bool                 `json:"createTokenAccount,omitempty"`
}

// JupiterPrioritizationFee represents new 2025 prioritization fee structure
type JupiterPrioritizationFee struct {
	PriorityLevelWithMaxLamports *JupiterPriorityLevel `json:"priorityLevelWithMaxLamports,omitempty"`
	JitoTipLamports              *int64                `json:"jitoTipLamports,omitempty"`
}

// JupiterPriorityLevel represents priority level
type JupiterPriorityLevel struct {
	MaxLamports   int64  `json:"maxLamports"`
	PriorityLevel string `json:"priorityLevel"` // "medium", "high", "veryHigh"
}

// JupiterSwapResponse represents response from Jupiter swap API
type JupiterSwapResponse struct {
	SwapTransaction           string                  `json:"swapTransaction"`
	LastValidBlockHeight      int64                   `json:"lastValidBlockHeight"`
	PrioritizationFeeLamports int64                   `json:"prioritizationFeeLamports,omitempty"`
	ComputeUnitLimit          int64                   `json:"computeUnitLimit,omitempty"`
	DynamicComputeUnitLimit   *int64                  `json:"dynamicComputeUnitLimit,omitempty"`
	SimulationError           *JupiterSimulationError `json:"simulationError,omitempty"`
}

type JupiterSimulationError struct {
	ErrorCode string `json:"errorCode"`
	Error     string `json:"error"`
}

// NewJupiterClient creates a new Jupiter API client
func NewJupiterClient(baseURL string) *JupiterClient {
	return &JupiterClient{
		client: &fasthttp.Client{
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
		},
		baseURL: baseURL,
	}
}

// GetSolPrice fetches current SOL price in USD using the working Jupiter price API v2
func (j *JupiterClient) GetSolPrice(ctx context.Context) (*valueobjects.SolPrice, error) {
	// 🔥 FIX: Use the correct Jupiter price API v2 endpoint
	solMintAddress := "So11111111111111111111111111111111111111112"
	url := fmt.Sprintf("https://price.jup.ag/v4/price?ids=%s", solMintAddress)

	req := fasthttp.AcquireRequest()
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(resp)

	req.SetRequestURI(url)
	req.Header.SetMethod(fasthttp.MethodGet)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "cash-farmer-bot/1.0")

	err := j.client.DoTimeout(req, resp, 30*time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}

	if resp.StatusCode() != fasthttp.StatusOK {
		return nil, fmt.Errorf("API request failed with status %d", resp.StatusCode())
	}

	var priceResp JupiterPriceResponse
	if err := json.Unmarshal(resp.Body(), &priceResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	solData, exists := priceResp.Data[solMintAddress]
	if !exists {
		return nil, fmt.Errorf("SOL price not found in response")
	}

	solPrice, err := valueobjects.NewSolPriceFromFloat(solData.Price)
	if err != nil {
		return nil, fmt.Errorf("failed to create SOL price: %w", err)
	}

	return solPrice, nil
}

// GetQuote fetches a quote for token swap from Jupiter API
func (j *JupiterClient) GetQuote(ctx context.Context, req JupiterQuoteRequest) (*JupiterQuoteResponse, error) {
	url := "https://lite-api.jup.ag/swap/v1/quote"

	params := fmt.Sprintf("?inputMint=%s&outputMint=%s&amount=%s&slippageBps=%d",
		req.InputMint, req.OutputMint, req.Amount, req.SlippageBps)

	if req.SwapMode != "" {
		params += "&swapMode=" + req.SwapMode
	}
	if req.OnlyDirectRoutes {
		params += "&onlyDirectRoutes=true"
	}
	if req.AsLegacyTransaction {
		params += "&asLegacyTransaction=true"
	}
	if req.RestrictIntermediateTokens {
		params += "&restrictIntermediateTokens=true"
	}

	url += params

	httpReq := fasthttp.AcquireRequest()
	httpResp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(httpReq)
	defer fasthttp.ReleaseResponse(httpResp)

	httpReq.SetRequestURI(url)
	httpReq.Header.SetMethod(fasthttp.MethodGet)
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("User-Agent", "cash-farmer-bot/1.0")

	err := j.client.DoTimeout(httpReq, httpResp, 30*time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}

	if httpResp.StatusCode() != fasthttp.StatusOK {
		return nil, fmt.Errorf("jupiter quote API request failed with status %d", httpResp.StatusCode())
	}

	var quoteResp JupiterQuoteResponse
	if err := json.Unmarshal(httpResp.Body(), &quoteResp); err != nil {
		return nil, fmt.Errorf("failed to decode quote response: %w", err)
	}

	return &quoteResp, nil
}

// GetSwapTransaction builds a swap transaction from Jupiter API
func (j *JupiterClient) GetSwapTransaction(ctx context.Context, req JupiterSwapRequest) (*JupiterSwapResponse, error) {
	url := "https://lite-api.jup.ag/swap/v1/swap"

	fmt.Printf("🔍 JUPITER SWAP API REQUEST: URL=%s\n", url)
	fmt.Printf("🔍 JUPITER SWAP API REQUEST: UserPublicKey=%s\n", req.UserPublicKey)
	fmt.Printf("🔍 JUPITER SWAP API REQUEST: WrapAndUnwrapSol=%t\n", req.WrapAndUnwrapSol)
	fmt.Printf("🔍 JUPITER SWAP API REQUEST: UseSharedAccounts=%t\n", req.UseSharedAccounts)
	fmt.Printf("🔍 JUPITER SWAP API REQUEST: AsLegacyTransaction=%t\n", req.AsLegacyTransaction)
	if req.ComputeUnitPriceMicroLamports != nil {
		fmt.Printf("🔍 JUPITER SWAP API REQUEST: ComputeUnitPrice=%d\n", *req.ComputeUnitPriceMicroLamports)
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal swap request: %w", err)
	}

	fmt.Printf("🔍 JUPITER SWAP API REQUEST: Body size=%d bytes\n", len(reqBody))

	quoteJSON, _ := json.Marshal(req.QuoteResponse)
	if len(quoteJSON) > 500 {
		fmt.Printf("🔍 JUPITER QUOTE JSON: %s...\n", string(quoteJSON[:500]))
	} else {
		fmt.Printf("🔍 JUPITER QUOTE JSON: %s\n", string(quoteJSON))
	}

	if len(reqBody) > 1000 {
		fmt.Printf("🔍 JUPITER REQUEST BODY: %s...\n", string(reqBody[:1000]))
	} else {
		fmt.Printf("🔍 JUPITER REQUEST BODY: %s\n", string(reqBody))
	}

	httpReq := fasthttp.AcquireRequest()
	httpResp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(httpReq)
	defer fasthttp.ReleaseResponse(httpResp)

	httpReq.SetRequestURI(url)
	httpReq.Header.SetMethod(fasthttp.MethodPost)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("User-Agent", "cash-farmer-bot/1.0")
	httpReq.SetBody(reqBody)

	err = j.client.DoTimeout(httpReq, httpResp, 30*time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}

	if httpResp.StatusCode() != fasthttp.StatusOK {
		fmt.Printf("❌ JUPITER SWAP API ERROR: Status=%d\n", httpResp.StatusCode())
		// Read error response body
		fmt.Printf("❌ JUPITER SWAP API ERROR BODY: %s\n", string(httpResp.Body()))
		return nil, fmt.Errorf("jupiter swap API request failed with status %d", httpResp.StatusCode())
	}

	// CRITICAL: Read raw response body first
	bodyBytes := httpResp.Body()

	// Truncate raw response if too long
	if len(bodyBytes) > 2000 {
		fmt.Printf("🔍 JUPITER RAW RESPONSE: %s...\n", string(bodyBytes[:2000]))
	} else {
		fmt.Printf("🔍 JUPITER RAW RESPONSE: %s\n", string(bodyBytes))
	}

	if strings.Contains(string(bodyBytes), "simulationError") {
		fmt.Printf("⚠️ JUPITER WARNING: Response contains simulationError - transaction may be invalid\n")
	}

	var swapResp JupiterSwapResponse
	if err := json.Unmarshal(bodyBytes, &swapResp); err != nil {
		fmt.Printf("❌ JUPITER SWAP API ERROR: Failed to decode response: %v\n", err)
		return nil, fmt.Errorf("failed to decode swap response: %w", err)
	}

	if swapResp.SimulationError != nil {
		fmt.Printf("⚠️ JUPITER SIMULATION ERROR: Code=%s, Message=%s\n",
			swapResp.SimulationError.ErrorCode, swapResp.SimulationError.Error)
		fmt.Printf("⚠️ JUPITER WARNING: Transaction may be invalid due to simulation error\n")

		if strings.Contains(swapResp.SimulationError.Error, "custom program error: 0x1") {
			fmt.Printf("🚨 JUPITER API DETECTED 0x1 ERROR DURING SIMULATION!\n")
			fmt.Printf("🔍 This indicates the transaction will fail with InsufficientFunds\n")
			fmt.Printf("🔍 Possible causes:\n")
			fmt.Printf("   - Token is frozen/restricted\n")
			fmt.Printf("   - Insufficient SOL for ATA creation\n")
			fmt.Printf("   - Route calculation issues\n")
			fmt.Printf("   - Slippage too tight\n")
			fmt.Printf("💡 RECOMMENDATION: Use alternative Jupiter parameters or different token\n")
		}
	}

	fmt.Printf("✅ JUPITER SWAP API SUCCESS: Transaction size=%d chars\n", len(swapResp.SwapTransaction))
	txStart := swapResp.SwapTransaction
	if len(txStart) > 50 {
		txStart = txStart[:50] + "..."
	}
	fmt.Printf("✅ JUPITER SWAP API SUCCESS: Transaction starts: %s\n", txStart)

	return &swapResp, nil
}

// ValidateQuote performs basic validation on the quote response
func (j *JupiterClient) ValidateQuote(quote *JupiterQuoteResponse, maxSlippageBps int) error {
	if quote == nil {
		return fmt.Errorf("quote is nil")
	}

	if quote.SlippageBps > maxSlippageBps {
		return fmt.Errorf("quote slippage %d bps exceeds maximum %d bps", quote.SlippageBps, maxSlippageBps)
	}

	priceImpact, err := strconv.ParseFloat(quote.PriceImpactPct, 64)
	if err != nil {
		return fmt.Errorf("failed to parse price impact: %w", err)
	}

	if priceImpact > 5.0 {
		return fmt.Errorf("high price impact: %.2f%%", priceImpact)
	}

	if quote.InAmount == "" || quote.OutAmount == "" {
		return fmt.Errorf("invalid quote: missing amounts")
	}

	return nil
}
