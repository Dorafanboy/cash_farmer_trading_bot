package api

import (
	"context"
	"fmt"
	"time"

	"cash-farmer/internal/domain/valueobjects"

	"github.com/goccy/go-json"
	"github.com/valyala/fasthttp"
	"golang.org/x/time/rate"
)

// DexscreenerClient handles communication with Dexscreener API
type DexscreenerClient struct {
	client      *fasthttp.Client
	baseURL     string
	rateLimiter *rate.Limiter
}

// DexscreenerTokenResponse represents response from Dexscreener token API
type DexscreenerTokenResponse struct {
	Pairs []DexscreenerPair `json:"pairs"`
}

// DexscreenerPair represents a token pair from Dexscreener
type DexscreenerPair struct {
	ChainID     string `json:"chainId"`
	PairAddress string `json:"pairAddress"`
	BaseToken   struct {
		Address string `json:"address"`
		Name    string `json:"name"`
		Symbol  string `json:"symbol"`
	} `json:"baseToken"`
	QuoteToken struct {
		Address string `json:"address"`
		Name    string `json:"name"`
		Symbol  string `json:"symbol"`
	} `json:"quoteToken"`
	PriceUSD  string `json:"priceUsd"`
	Liquidity struct {
		USD float64 `json:"usd"`
	} `json:"liquidity"`
	FDV       float64 `json:"fdv"`
	MarketCap float64 `json:"marketCap"`
}

// NewDexscreenerClient creates a new Dexscreener API client
func NewDexscreenerClient(baseURL string) *DexscreenerClient {
	return &DexscreenerClient{
		client: &fasthttp.Client{
			ReadTimeout:  15 * time.Second,
			WriteTimeout: 15 * time.Second,
		},
		baseURL:     baseURL,
		rateLimiter: rate.NewLimiter(rate.Every(2*time.Second), 1),
	}
}

// GetTokenMetrics fetches token metrics from Dexscreener API
func (d *DexscreenerClient) GetTokenMetrics(
	ctx context.Context,
	address valueobjects.SolanaTokenAddress,
) (*valueobjects.TokenMetrics, error) {
	if err := d.rateLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limiter error: %w", err)
	}

	url := fmt.Sprintf("https://api.dexscreener.com/tokens/v1/solana/%s", address.Value())
	fmt.Printf("🔍 DEXSCREENER API: Making request to: %s\n", url)

	req := fasthttp.AcquireRequest()
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(resp)

	req.SetRequestURI(url)
	req.Header.SetMethod(fasthttp.MethodGet)

	err := d.client.DoTimeout(req, resp, 15*time.Second)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}

	statusCode := resp.StatusCode()
	fmt.Printf("🔍 DEXSCREENER API: Response status: %d\n", statusCode)

	if statusCode != 200 {
		return nil, fmt.Errorf("API request failed with status %d", statusCode)
	}

	var tokenPairs []DexscreenerPair
	if err := json.Unmarshal(resp.Body(), &tokenPairs); err != nil {
		fmt.Printf("🚨 DEXSCREENER API ERROR: Failed to decode response: %v\n", err)
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	fmt.Printf("🔍 DEXSCREENER API: Found %d pairs\n", len(tokenPairs))

	if len(tokenPairs) == 0 {
		return nil, fmt.Errorf("no trading pairs found for token %s", address.ShortString())
	}

	var bestPair *DexscreenerPair
	for i := range tokenPairs {
		pair := &tokenPairs[i]
		fmt.Printf("🔍 DEXSCREENER PAIR: chainId=%s, price=%s, liquidity=%.2f, marketCap=%.2f\n",
			pair.ChainID, pair.PriceUSD, pair.Liquidity.USD, pair.MarketCap)

		if pair.ChainID == "solana" {
			if bestPair == nil || pair.Liquidity.USD > bestPair.Liquidity.USD {
				bestPair = pair
			}
		}
	}

	if bestPair == nil {
		return nil, fmt.Errorf("no Solana pairs found for token %s", address.ShortString())
	}

	fmt.Printf("🔍 DEXSCREENER BEST PAIR: symbol=%s, name=%s, price=%s, liquidity=%.2f, marketCap=%.2f\n",
		bestPair.BaseToken.Symbol, bestPair.BaseToken.Name, bestPair.PriceUSD, bestPair.Liquidity.USD, bestPair.MarketCap)

	priceFloat := 0.0
	if bestPair.PriceUSD != "" {
		_, err := fmt.Sscanf(bestPair.PriceUSD, "%f", &priceFloat)
		if err != nil {
			return nil, fmt.Errorf("failed to parse price: %w", err)
		}
	}

	marketCapValue := bestPair.MarketCap

	if marketCapValue == 0 {
		marketCapValue = bestPair.FDV
		isSOLToken := address.Value() == "So11111111111111111111111111111111111111112"

		if isSOLToken && marketCapValue == 0 {
			solCirculatingSupply := 400000000.0
			marketCapValue = priceFloat * solCirculatingSupply
			fmt.Printf("🔄 SOL SPECIAL HANDLING: Calculated market cap = $%.2fB\n", marketCapValue/1000000000)
		}
	}

	tokenMetrics, err := valueobjects.NewTokenMetricsFromFloats(
		address,
		bestPair.BaseToken.Name,
		bestPair.BaseToken.Symbol,
		priceFloat,
		bestPair.Liquidity.USD,
		marketCapValue,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create token metrics: %w", err)
	}

	fmt.Printf("✅ DEXSCREENER METRICS: Created TokenMetrics successfully\n")
	return tokenMetrics, nil
}
