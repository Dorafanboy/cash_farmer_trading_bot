package valueobjects

import (
	"fmt"
	"time"

	"github.com/shopspring/decimal"
)

type TokenMetrics struct {
	address   SolanaTokenAddress
	name      string
	symbol    string
	priceUSD  decimal.Decimal
	liquidity decimal.Decimal
	marketCap decimal.Decimal
	timestamp time.Time
}

func NewTokenMetrics(
	address SolanaTokenAddress,
	name, symbol string,
	priceUSD, liquidity, marketCap decimal.Decimal,
) *TokenMetrics {
	return &TokenMetrics{
		address:   address,
		name:      name,
		symbol:    symbol,
		priceUSD:  priceUSD,
		liquidity: liquidity,
		marketCap: marketCap,
		timestamp: time.Now(),
	}
}

func NewTokenMetricsFromFloats(
	address SolanaTokenAddress,
	name, symbol string,
	price, liquidity, marketCap float64,
) (*TokenMetrics, error) {
	if price < 0 {
		return nil, fmt.Errorf("price cannot be negative: %f", price)
	}
	if liquidity < 0 {
		return nil, fmt.Errorf("liquidity cannot be negative: %f", liquidity)
	}
	if marketCap < 0 {
		return nil, fmt.Errorf("market cap cannot be negative: %f", marketCap)
	}

	return &TokenMetrics{
		address:   address,
		name:      name,
		symbol:    symbol,
		priceUSD:  decimal.NewFromFloat(price),
		liquidity: decimal.NewFromFloat(liquidity),
		marketCap: decimal.NewFromFloat(marketCap),
		timestamp: time.Now(),
	}, nil
}

func (t *TokenMetrics) Address() SolanaTokenAddress {
	return t.address
}

func (t *TokenMetrics) Name() string {
	return t.name
}

func (t *TokenMetrics) Symbol() string {
	return t.symbol
}

func (t *TokenMetrics) PriceUSD() decimal.Decimal {
	return t.priceUSD
}

func (t *TokenMetrics) Liquidity() decimal.Decimal {
	return t.liquidity
}

func (t *TokenMetrics) MarketCap() decimal.Decimal {
	return t.marketCap
}

func (t *TokenMetrics) Timestamp() time.Time {
	return t.timestamp
}

func (t *TokenMetrics) FormattedPrice() string {
	// Convert to float64 and format nicely without excessive decimal places
	priceFloat, _ := t.priceUSD.Float64()

	if priceFloat >= 1000 {
		return fmt.Sprintf("$%.2f", priceFloat)
	} else if priceFloat >= 1 {
		return fmt.Sprintf("$%.6f", priceFloat)
	} else if priceFloat >= 0.0001 {
		return fmt.Sprintf("$%.6f", priceFloat)
	} else {
		return fmt.Sprintf("$%.8f", priceFloat)
	}
}

func (t *TokenMetrics) FormattedLiquidity() string {
	// Convert to float64 first to avoid big.Int formatting issues
	liquidityFloat, _ := t.liquidity.Float64()

	if liquidityFloat >= 1000000000 {
		billions := liquidityFloat / 1000000000
		return fmt.Sprintf("$%.2fB", billions)
	}
	if liquidityFloat >= 1000000 {
		millions := liquidityFloat / 1000000
		return fmt.Sprintf("$%.2fM", millions)
	}
	if liquidityFloat >= 1000 {
		thousands := liquidityFloat / 1000
		return fmt.Sprintf("$%.2fK", thousands)
	}
	return fmt.Sprintf("$%.2f", liquidityFloat)
}

func (t *TokenMetrics) FormattedMarketCap() string {
	// Convert to float64 first to avoid big.Int formatting issues
	marketCapFloat, _ := t.marketCap.Float64()

	if marketCapFloat >= 1000000000 {
		billions := marketCapFloat / 1000000000
		return fmt.Sprintf("$%.2fB", billions)
	}
	if marketCapFloat >= 1000000 {
		millions := marketCapFloat / 1000000
		return fmt.Sprintf("$%.2fM", millions)
	}
	if marketCapFloat >= 1000 {
		thousands := marketCapFloat / 1000
		return fmt.Sprintf("$%.2fK", thousands)
	}
	return fmt.Sprintf("$%.2f", marketCapFloat)
}

func (t *TokenMetrics) String() string {
	return fmt.Sprintf(
		"Token %s: Price %s, Liquidity %s, Market Cap %s",
		t.address.String(),
		t.FormattedPrice(),
		t.FormattedLiquidity(),
		t.FormattedMarketCap(),
	)
}
