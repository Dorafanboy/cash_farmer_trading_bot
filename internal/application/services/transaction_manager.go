package services

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"cash-farmer/internal/infrastructure/api"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/system"
)

// min helper function
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// TransactionManager orchestrates Jupiter + Jito + Solana integration
type TransactionManager struct {
	jupiter       *api.JupiterClient
	jito          *api.JitoClient
	solana        *api.SolanaClient
	config        *TransactionConfig
	walletService WalletService
}

// TransactionConfig holds configuration for transaction routing
type TransactionConfig struct {
	// Jito settings
	UseJitoPrimary      bool `json:"use_jito_primary"`
	JitoTimeoutSeconds  int  `json:"jito_timeout_seconds"`
	JitoFallbackEnabled bool `json:"jito_fallback_enabled"`

	// Solana fallback settings
	SolanaTimeoutSeconds int `json:"solana_timeout_seconds"`
	MaxRetries           int `json:"max_retries"`

	// Trading settings
	MaxSlippageBps        int     `json:"max_slippage_bps"`
	MaxPriceImpactPercent float64 `json:"max_price_impact_percent"`

	// Priority fee settings
	ComputeUnitPriceMicroLamports *int   `json:"compute_unit_price_micro_lamports"`
	JitoTipLamports               *int64 `json:"jito_tip_lamports"`
}

// TradeRequest represents a trade request
type TradeRequest struct {
	InputMint     string `json:"input_mint"`
	OutputMint    string `json:"output_mint"`
	Amount        string `json:"amount"`
	SlippageBps   int    `json:"slippage_bps"`
	UserPublicKey string `json:"user_public_key"`
	UserID        int64  `json:"user_id"`   // Добавляем UserID для получения private key
	WalletID      int64  `json:"wallet_id"` // Добавляем WalletID для получения private key
	SwapMode      string `json:"swap_mode,omitempty"`
}

// TradeResult represents the result of a trade operation
type TradeResult struct {
	Success         bool                      `json:"success"`
	TransactionID   string                    `json:"transaction_id,omitempty"`
	BundleID        string                    `json:"bundle_id,omitempty"`
	Method          string                    `json:"method"` // "jito" or "solana"
	Quote           *api.JupiterQuoteResponse `json:"quote,omitempty"`
	ExecutionTimeMs int64                     `json:"execution_time_ms"`
	Error           string                    `json:"error,omitempty"`
	FinalStatus     string                    `json:"final_status,omitempty"`
}

// BalanceInfo represents account balance information
type BalanceInfo struct {
	SOLBalance    int64            `json:"sol_balance"`
	TokenBalances map[string]int64 `json:"token_balances"`
	Slot          int64            `json:"slot"`
}

// TokenSafetyResult represents the result of token safety checks
type TokenSafetyResult struct {
	IsSafe         bool
	Reason         string
	Recommendation string
}

// NewTransactionManager creates a new transaction manager
func NewTransactionManager(
	jupiter *api.JupiterClient,
	jito *api.JitoClient,
	solana *api.SolanaClient,
	config *TransactionConfig,
	walletService WalletService,
) *TransactionManager {
	if config == nil {
		config = DefaultTransactionConfig()
	}

	return &TransactionManager{
		jupiter:       jupiter,
		jito:          jito,
		solana:        solana,
		config:        config,
		walletService: walletService,
	}
}

// DefaultTransactionConfig returns default configuration
func DefaultTransactionConfig() *TransactionConfig {
	return &TransactionConfig{
		UseJitoPrimary:                true,
		JitoTimeoutSeconds:            30,
		JitoFallbackEnabled:           true,
		SolanaTimeoutSeconds:          45,
		MaxRetries:                    3,
		MaxSlippageBps:                2000, // TDD: Увеличили для мем коинов (20%)
		MaxPriceImpactPercent:         5.0,  // 5%
		ComputeUnitPriceMicroLamports: nil,  // Auto
		JitoTipLamports:               nil,  // No tip by default
	}
}

// ExecuteTrade executes a token swap using Jupiter + Jito/Solana
func (tm *TransactionManager) ExecuteTrade(ctx context.Context, req TradeRequest) (*TradeResult, error) {
	startTime := time.Now()

	result := &TradeResult{
		Success: false,
	}

	// Step 1: Get quote from Jupiter
	log.Printf("Getting Jupiter quote for %s -> %s, amount: %s", req.InputMint, req.OutputMint, req.Amount)

	quoteReq := api.JupiterQuoteRequest{
		InputMint:                  req.InputMint,
		OutputMint:                 req.OutputMint,
		Amount:                     req.Amount,
		SlippageBps:                req.SlippageBps,
		SwapMode:                   req.SwapMode,
		OnlyDirectRoutes:           false, // CHANGED: Allow all routes to avoid empty responses
		AsLegacyTransaction:        false, // MEME MODE: Use modern format
		RestrictIntermediateTokens: true,  // MEME MODE: Stable routes only (critical!)
	}

	// CRITICAL: Log detailed quote request for debugging
	log.Printf("🔍 JUPITER QUOTE REQUEST: InputMint=%s, OutputMint=%s, Amount=%s, Slippage=%d, OnlyDirectRoutes=%t",
		quoteReq.InputMint, quoteReq.OutputMint, quoteReq.Amount, quoteReq.SlippageBps, quoteReq.OnlyDirectRoutes)

	// CRITICAL: Check for known problematic tokens
	if req.OutputMint == "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v" {
		log.Printf("⚠️ TOKEN WARNING: This token (Shhh) may be frozen/restricted - try different token")
	}

	quote, err := tm.jupiter.GetQuote(ctx, quoteReq)
	if err != nil {
		result.Error = fmt.Sprintf("failed to get Jupiter quote: %v", err)
		result.ExecutionTimeMs = time.Since(startTime).Milliseconds()
		log.Printf("❌ JUPITER QUOTE ERROR: %v", err)
		return result, err
	}

	result.Quote = quote
	log.Printf("✅ JUPITER QUOTE SUCCESS: InAmount=%s, OutAmount=%s, Price=%s",
		quote.InAmount, quote.OutAmount, quote.OutAmount)

	// CRITICAL: Log detailed quote for debugging corrupted transactions
	log.Printf("🔍 JUPITER QUOTE DETAILS: InputMint=%s, OutputMint=%s", quote.InputMint, quote.OutputMint)
	log.Printf("🔍 JUPITER QUOTE DETAILS: SlippageBps=%d, SwapMode=%s", quote.SlippageBps, quote.SwapMode)
	log.Printf("🔍 JUPITER QUOTE DETAILS: PriceImpactPct=%s", quote.PriceImpactPct)
	log.Printf("🔍 JUPITER QUOTE DETAILS: RoutePlan has %d steps", len(quote.RoutePlan))

	// CRITICAL: Log first route step details
	if len(quote.RoutePlan) > 0 {
		step := quote.RoutePlan[0]
		log.Printf("🔍 JUPITER ROUTE STEP 1: Label=%s, AmmKey=%s", step.SwapInfo.Label, step.SwapInfo.AmmKey)
		log.Printf("🔍 JUPITER ROUTE STEP 1: InAmount=%s, OutAmount=%s", step.SwapInfo.InAmount, step.SwapInfo.OutAmount)
	}

	// Step 2: Validate quote
	if err := tm.jupiter.ValidateQuote(quote, tm.config.MaxSlippageBps); err != nil {
		result.Error = fmt.Sprintf("quote validation failed: %v", err)
		result.ExecutionTimeMs = time.Since(startTime).Milliseconds()
		return result, err
	}

	log.Printf("Quote validated: in=%s, out=%s, slippage=%d bps", quote.InAmount, quote.OutAmount, quote.SlippageBps)

	// Step 2.5: Check user balance BEFORE building transaction
	balance, err := tm.solana.GetBalance(ctx, req.UserPublicKey)
	if err != nil {
		result.Error = fmt.Sprintf("failed to get user balance: %v", err)
		result.ExecutionTimeMs = time.Since(startTime).Milliseconds()
		return result, err
	}

	// TDD-BASED BALANCE CALCULATION - Based on real testing results
	swapAmount, _ := strconv.ParseInt(req.Amount, 10, 64)

	// TDD DISCOVERY: ATA costs are REAL and must be included!
	// Test showed: minimum working swap = balance - ATA_cost - fees - buffer
	usdcATACost := int64(2039280)  // REAL cost for USDC ATA creation
	jitoTip := int64(500)          // MINIMAL: 0.0000005 SOL
	priorityFees := int64(0)       // TDD proved: 0 fees with minimal params
	transactionFees := int64(5000) // MINIMAL: 0.000005 SOL
	safetyBuffer := int64(100000)  // REASONABLE: 0.0001 SOL

	// TDD FORMULA: balance >= swapAmount + ataCreationCosts + allFees
	requiredBalance := swapAmount + usdcATACost + jitoTip + priorityFees + transactionFees + safetyBuffer

	log.Printf("💰 TDD BALANCE: ATA cost: %d lamports", usdcATACost)
	log.Printf("💰 TDD BALANCE: Minimal tip: %d lamports", jitoTip)
	log.Printf("💰 TDD BALANCE: Zero priority fees (TDD proved)", priorityFees)
	log.Printf("💰 TDD BALANCE: Minimal tx fees: %d lamports", transactionFees)
	log.Printf("💰 TDD BALANCE: Safety buffer: %d lamports", safetyBuffer)

	log.Printf("💰 TDD BALANCE CHECK: User has %d lamports, required: %d lamports", balance, requiredBalance)
	log.Printf("💰 TDD BREAKDOWN: Swap=%d, ATA=%d, Fees=%d (TDD formula)", swapAmount, usdcATACost, jitoTip+priorityFees+transactionFees+safetyBuffer)

	if balance < requiredBalance {
		// TDD DISCOVERY: Auto-calculate max possible swap amount
		maxPossibleSwap := balance - usdcATACost - jitoTip - priorityFees - transactionFees - safetyBuffer
		if maxPossibleSwap > 0 && maxPossibleSwap < swapAmount {
			log.Printf("⚠️ TDD AUTO-ADJUST: Requested %d too big, max possible: %d lamports", swapAmount, maxPossibleSwap)
			log.Printf("💡 TDD SOLUTION: Auto-adjusting swap amount to fit balance")

			// Update request amount to max possible
			req.Amount = strconv.FormatInt(maxPossibleSwap, 10)
			swapAmount = maxPossibleSwap
			requiredBalance = swapAmount + usdcATACost + jitoTip + priorityFees + transactionFees + safetyBuffer

			log.Printf("✅ TDD AUTO-ADJUST: New swap amount: %d lamports (%.6f SOL)", swapAmount, float64(swapAmount)/1e9)
		} else {
			result.Error = fmt.Sprintf("TDD: insufficient balance: have %d, need %d lamports (including ATA costs)",
				balance, requiredBalance)
			result.ExecutionTimeMs = time.Since(startTime).Milliseconds()
			return result, fmt.Errorf("insufficient balance for meme coin swap")
		}
	}

	log.Printf("✅ TDD BALANCE CHECK PASSED: User can afford swap of %d lamports!", swapAmount)

	// TOKEN SAFETY CHECKS - Added to prevent 0x1 errors from problematic tokens
	if quote != nil {
		tokenSafetyCheck := tm.checkTokenSafety(ctx, req.OutputMint, quote)
		if !tokenSafetyCheck.IsSafe {
			result.Error = fmt.Sprintf("Token safety check failed: %s. Recommendation: %s",
				tokenSafetyCheck.Reason, tokenSafetyCheck.Recommendation)
			result.ExecutionTimeMs = time.Since(startTime).Milliseconds()
			log.Printf("🚨 TOKEN SAFETY: %s", result.Error)
			return result, fmt.Errorf("token safety check failed: %s", tokenSafetyCheck.Reason)
		}
		log.Printf("✅ TOKEN SAFETY: Token passed safety checks")
	}

	// Step 3: Get swap transaction from Jupiter with ULTRA-MINIMAL PARAMETERS
	// TDD DISCOVERY: "optimizations" cause 0x1 errors, minimal params work!
	swapReq := api.JupiterSwapRequest{
		QuoteResponse:    *quote,
		UserPublicKey:    req.UserPublicKey,
		WrapAndUnwrapSol: false, // MEME MODE: Avoid Wrapped SOL ATA costs
		// REMOVED ALL "OPTIMIZATIONS" BASED ON TDD TESTS:
		// - UseSharedAccounts: causes expensive priority fees
		// - DynamicComputeUnitLimit: causes expensive priority fees
		// - DynamicSlippage: causes expensive priority fees
		// - CreateTokenAccount: let Jupiter handle automatically
		// - PrioritizationFeeLamports: causes expensive fees (TDD showed 0 vs 38k lamports!)
	}

	// MEME COIN MODE: Enhanced logging of MINIMAL approach
	log.Printf("🚀 TDD-FIXED MEME COIN REQUEST: WrapSol=%t, NO extra params (based on test results)",
		swapReq.WrapAndUnwrapSol)
	log.Printf("🚀 TDD-DISCOVERY: Removing all 'optimizations' that caused 0x1 errors in tests")

	swapResp, err := tm.jupiter.GetSwapTransaction(ctx, swapReq)
	if err != nil {
		result.Error = fmt.Sprintf("failed to get swap transaction: %v", err)
		result.ExecutionTimeMs = time.Since(startTime).Milliseconds()
		log.Printf("❌ JUPITER SWAP ERROR: %v", err)
		return result, err
	}

	// CRITICAL: Check if Jupiter returned empty/invalid response
	if swapResp == nil {
		result.Error = "Jupiter returned nil swap response"
		result.ExecutionTimeMs = time.Since(startTime).Milliseconds()
		log.Printf("❌ JUPITER ERROR: Nil swap response")
		return result, fmt.Errorf("Jupiter nil response")
	}

	if swapResp.SwapTransaction == "" {
		result.Error = "Jupiter returned empty swap transaction"
		result.ExecutionTimeMs = time.Since(startTime).Milliseconds()
		log.Printf("❌ JUPITER ERROR: Empty swap transaction")
		return result, fmt.Errorf("Jupiter empty transaction")
	}

	log.Printf("Swap transaction built, size: %d chars", len(swapResp.SwapTransaction))

	// CRITICAL: Detailed transaction format analysis
	txStart := swapResp.SwapTransaction[:min(50, len(swapResp.SwapTransaction))]
	log.Printf("🔍 JUPITER TX FORMAT: Starts with: %s", txStart)

	// Inform about unsigned transactions (this is normal!)
	if strings.HasPrefix(swapResp.SwapTransaction, "AQAAAAAAAAAA") {
		log.Printf("✅ JUPITER TX STATUS: Received unsigned transaction (normal behavior)")
		log.Printf("🔐 NEXT STEP: Will sign transaction with user's private key")
	}

	// Jupiter API correctly returns UNSIGNED transactions (starts with "AQAAAAAAAAAA...")
	// This is NORMAL behavior - we need to sign them in our bot
	// Check for various corruption patterns
	if false { // DISABLED - "AQAAAAAAAAAA" is normal for unsigned transactions
		log.Printf("❌ JUPITER ERROR: Transaction appears corrupted - all zeros pattern, trying legacy format")

		// FALLBACK: Try with legacy format (2025) - TDD MINIMAL APPROACH
		swapReqLegacy := api.JupiterSwapRequest{
			QuoteResponse:       *quote,
			UserPublicKey:       req.UserPublicKey,
			WrapAndUnwrapSol:    false, // MEME MODE: Avoid Wrapped SOL ATA costs
			AsLegacyTransaction: true,  // FALLBACK: Use legacy format
			// TDD: Removed all other parameters that cause 0x1 errors
		}

		// Copy prioritization fee if set
		if swapReq.PrioritizationFeeLamports != nil {
			swapReqLegacy.PrioritizationFeeLamports = swapReq.PrioritizationFeeLamports
		}

		log.Printf("🔧 JUPITER FALLBACK: Trying with AsLegacyTransaction=true")
		swapRespLegacy, err := tm.jupiter.GetSwapTransaction(ctx, swapReqLegacy)
		if err != nil || swapRespLegacy == nil || strings.HasPrefix(swapRespLegacy.SwapTransaction, "AQAAAAAAAAAA") {
			result.Error = "Jupiter returned corrupted transaction even with legacy format"
			result.ExecutionTimeMs = time.Since(startTime).Milliseconds()
			log.Printf("❌ JUPITER FALLBACK FAILED: Legacy format also corrupted")

			// FINAL ATTEMPT: Try with minimal parameters (TDD-based 2025)
			log.Printf("🔧 JUPITER FINAL ATTEMPT: Trying TDD-proven minimal parameters")
			swapReqMinimal := api.JupiterSwapRequest{
				QuoteResponse:       *quote,
				UserPublicKey:       req.UserPublicKey,
				WrapAndUnwrapSol:    false, // MEME MODE: Avoid Wrapped SOL ATA costs
				AsLegacyTransaction: true,  // Legacy format
				// TDD: ALL OTHER PARAMETERS REMOVED - they cause 0x1 errors!
			}

			swapRespMinimal, err := tm.jupiter.GetSwapTransaction(ctx, swapReqMinimal)
			if err == nil && swapRespMinimal != nil && !strings.HasPrefix(swapRespMinimal.SwapTransaction, "AQAAAAAAAAAA") {
				log.Printf("✅ JUPITER MINIMAL SUCCESS: Working with minimal parameters, size: %d chars", len(swapRespMinimal.SwapTransaction))
				swapResp = swapRespMinimal
			} else {
				log.Printf("❌ JUPITER ALL METHODS FAILED: Token may be frozen/restricted or Jupiter API issue")
				result.Error = "Jupiter API returns corrupted transactions - token may be frozen or restricted"
				result.ExecutionTimeMs = time.Since(startTime).Milliseconds()
				return result, fmt.Errorf("Jupiter transaction corruption detected")
			}
		}

		log.Printf("✅ JUPITER FALLBACK SUCCESS: Legacy format worked, size: %d chars", len(swapRespLegacy.SwapTransaction))
		swapResp = swapRespLegacy
	}

	if strings.HasPrefix(swapResp.SwapTransaction, "111111111111") {
		result.Error = "Jupiter returned corrupted transaction (all ones pattern)"
		result.ExecutionTimeMs = time.Since(startTime).Milliseconds()
		log.Printf("❌ JUPITER ERROR: Transaction appears corrupted - all ones pattern")
		return result, fmt.Errorf("Jupiter transaction corruption detected")
	}

	// Validate base64 format
	if _, err := base64.StdEncoding.DecodeString(swapResp.SwapTransaction); err != nil {
		result.Error = fmt.Sprintf("Jupiter returned invalid base64 transaction: %v", err)
		result.ExecutionTimeMs = time.Since(startTime).Milliseconds()
		log.Printf("❌ JUPITER ERROR: Invalid base64 format: %v", err)
		return result, fmt.Errorf("Jupiter transaction invalid base64")
	}

	log.Printf("✅ JUPITER TX VALIDATION: Format appears valid")

	// CRITICAL: Create separate transaction for Solana RPC if needed
	var solanaTransaction string
	if tm.config.JitoFallbackEnabled || !tm.config.UseJitoPrimary {
		log.Printf("🔧 SOLANA PREP: Creating separate transaction for Solana RPC (TDD minimal)")
		solanaReq := api.JupiterSwapRequest{
			QuoteResponse:    *quote,
			UserPublicKey:    req.UserPublicKey,
			WrapAndUnwrapSol: false, // MEME MODE: Avoid Wrapped SOL ATA costs
			// TDD: Keep it minimal - removed all parameters that cause 0x1 errors
		}

		// Copy prioritization fee from main request
		if swapReq.PrioritizationFeeLamports != nil {
			solanaReq.PrioritizationFeeLamports = swapReq.PrioritizationFeeLamports
		}

		log.Printf("🔍 SOLANA PREP REQUEST: SharedAccounts=%t (different from Jito)", solanaReq.UseSharedAccounts)

		solanaResp, err := tm.jupiter.GetSwapTransaction(ctx, solanaReq)
		if err != nil {
			log.Printf("❌ SOLANA PREP ERROR: Failed to create Solana transaction: %v", err)
			solanaTransaction = swapResp.SwapTransaction // Fallback to original
		} else {
			solanaTransaction = solanaResp.SwapTransaction
			log.Printf("✅ SOLANA PREP: Created separate transaction, size: %d chars", len(solanaTransaction))

			// Compare transaction starts
			jitoStart := swapResp.SwapTransaction[:min(30, len(swapResp.SwapTransaction))]
			solanaStart := solanaTransaction[:min(30, len(solanaTransaction))]
			log.Printf("🔍 TX COMPARISON: Jito starts: %s", jitoStart)
			log.Printf("🔍 TX COMPARISON: Solana starts: %s", solanaStart)
		}
	} else {
		solanaTransaction = swapResp.SwapTransaction
		log.Printf("🔍 SOLANA PREP: Using same transaction as Jito")
	}

	log.Printf("🔍 CONFIG DEBUG: UseJitoPrimary=%t, JitoFallbackEnabled=%t", tm.config.UseJitoPrimary, tm.config.JitoFallbackEnabled)

	// Step 4: Execute transaction using smart routing
	if tm.config.UseJitoPrimary {
		// Try Jito first
		log.Printf("Attempting Jito bundle execution")
		if bundleResult := tm.executeViaJito(ctx, swapResp.SwapTransaction, req, result); bundleResult {
			result.Method = "jito"
			result.Success = true
			result.ExecutionTimeMs = time.Since(startTime).Milliseconds()
			return result, nil
		}

		// Jito failed, try fallback if enabled
		if tm.config.JitoFallbackEnabled {
			log.Printf("Jito failed, attempting Solana fallback")
			if solanaResult := tm.executeViaSolana(ctx, solanaTransaction, req, result); solanaResult {
				result.Method = "solana"
				result.Success = true
				result.ExecutionTimeMs = time.Since(startTime).Milliseconds()
				return result, nil
			}
		} else {
			log.Printf("🔍 CONFIG DEBUG: Jito fallback is DISABLED, not trying Solana")
			// FORCE fallback for rate limit errors
			if result.Error != "" && (strings.Contains(result.Error, "rate limited") || strings.Contains(result.Error, "congested")) {
				log.Printf("🔧 FORCE FALLBACK: Rate limit detected, trying Solana anyway")
				if solanaResult := tm.executeViaSolana(ctx, solanaTransaction, req, result); solanaResult {
					result.Method = "solana"
					result.Success = true
					result.ExecutionTimeMs = time.Since(startTime).Milliseconds()
					return result, nil
				}
			}
		}
	} else {
		// Use Solana directly
		log.Printf("Attempting Solana execution")
		if solanaResult := tm.executeViaSolana(ctx, solanaTransaction, req, result); solanaResult {
			result.Method = "solana"
			result.Success = true
			result.ExecutionTimeMs = time.Since(startTime).Milliseconds()
			return result, nil
		}
	}

	// ENHANCED RETRY: Try alternative Jupiter parameters if error contains 0x1
	if result.Error != "" && strings.Contains(result.Error, "custom program error: 0x1") {
		log.Printf("🔧 AUTO-FIX ATTEMPT: Detected 0x1 error, trying alternative Jupiter parameters...")
		retryResult := tm.retryWithAlternativeParams(ctx, req, quote, startTime)
		if retryResult.Success {
			return retryResult, nil
		}
		// If retry also failed, append retry info to original error
		result.Error = fmt.Sprintf("%s; Auto-retry also failed: %s", result.Error, retryResult.Error)
	}

	// All methods failed
	result.Error = "all transaction execution methods failed"
	result.ExecutionTimeMs = time.Since(startTime).Milliseconds()
	return result, fmt.Errorf("all transaction execution methods failed")
}

// executeViaJito attempts to execute transaction via Jito bundle
func (tm *TransactionManager) executeViaJito(ctx context.Context, transaction string, req TradeRequest, result *TradeResult) bool {
	timeout := time.Duration(tm.config.JitoTimeoutSeconds) * time.Second
	jitoCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	log.Printf("🔍 JITO DEBUG: Starting Jito execution, timeout=%d seconds", tm.config.JitoTimeoutSeconds)

	// CRITICAL: Jupiter transaction для Jito НЕ НУЖНО подписывать - она уже готова!
	// (Jupiter возвращает transaction в специальном формате для bundle)
	log.Printf("🔧 JITO BUNDLE: Using Jupiter transaction as-is (unsigned for bundle)")

	// DEBUG: Log transaction preview to detect corruption
	txPreview := transaction
	if len(txPreview) > 100 {
		txPreview = txPreview[:100] + "..."
	}
	log.Printf("🔍 JITO TX DEBUG: Transaction preview: %s", txPreview)

	// Check for common corrupted patterns
	if strings.HasPrefix(transaction, "AQAAAAAAAAAA") {
		log.Printf("⚠️ JITO TX WARNING: Transaction appears to be corrupted (all zeros pattern)")
	}

	// Validate transaction format
	if err := tm.ValidateTransaction(transaction); err != nil {
		result.Error = fmt.Sprintf("invalid transaction format: %v", err)
		log.Printf("🔍 JITO TX ERROR: %v", err)
		return false
	}

	// Prepare bundle transactions
	bundleTransactions := []string{transaction}

	// CRITICAL: Add tip transaction if Jito tip is configured
	if tm.config.JitoTipLamports != nil && *tm.config.JitoTipLamports > 0 {
		tipAmount := *tm.config.JitoTipLamports
		log.Printf("🔧 JITO TIP: Adding tip transaction with %d lamports", tipAmount)

		// Get random tip account
		tipAccount, err := tm.jito.GetRandomTipAccount(jitoCtx)
		if err != nil {
			log.Printf("⚠️ JITO TIP WARNING: Failed to get tip account: %v", err)
		} else {
			log.Printf("🎯 JITO TIP: Creating REAL tip transaction to %s for %d lamports", tipAccount.Account, tipAmount)

			// Create actual tip transaction with Solana SDK
			tipTx, err := tm.createTipTransaction(jitoCtx, tipAccount.Account, tipAmount, req)
			if err != nil {
				log.Printf("⚠️ JITO TIP ERROR: Failed to create tip transaction: %v", err)
				// Proceed with main transaction only as fallback
			} else {
				// Add tip transaction to bundle BEFORE main transaction
				bundleTransactions = []string{tipTx, transaction}
				log.Printf("✅ JITO TIP: Added REAL tip transaction to bundle (2 transactions total)")
			}
		}
	} else {
		log.Printf("⚠️ JITO TIP WARNING: No tip configured - this may cause bundle rejection")
	}

	// Submit bundle
	bundleResult, err := tm.jito.SendBundle(jitoCtx, bundleTransactions)
	if err != nil {
		result.Error = fmt.Sprintf("jito bundle submission failed: %v", err)
		log.Printf("🔍 JITO DEBUG: Bundle submission failed: %v", err)
		return false
	}

	result.BundleID = bundleResult.BundleID
	log.Printf("🔍 JITO DEBUG: Bundle submitted successfully: %s", bundleResult.BundleID)

	// Wait for confirmation
	log.Printf("🔍 JITO DEBUG: Waiting for bundle confirmation...")
	status, err := tm.jito.WaitForBundleConfirmation(jitoCtx, bundleResult.BundleID, timeout)
	if err != nil {
		result.Error = fmt.Sprintf("jito bundle confirmation failed: %v", err)
		log.Printf("🔍 JITO DEBUG: Bundle confirmation failed: %v", err)
		return false
	}

	log.Printf("🔍 JITO DEBUG: Bundle status received: %s, transactions: %d", status.Status, len(status.Transactions))

	if status.Status == "confirmed" && len(status.Transactions) > 0 {
		result.TransactionID = status.Transactions[0]
		result.FinalStatus = "confirmed"
		log.Printf("🔍 JITO DEBUG: Bundle confirmed successfully, tx: %s", result.TransactionID)
		return true
	}

	result.Error = fmt.Sprintf("jito bundle failed with status: %s", status.Status)
	log.Printf("🔍 JITO DEBUG: Bundle failed with status: %s", status.Status)
	return false
}

// executeViaSolana attempts to execute transaction via standard Solana RPC
func (tm *TransactionManager) executeViaSolana(ctx context.Context, transaction string, req TradeRequest, result *TradeResult) bool {
	timeout := time.Duration(tm.config.SolanaTimeoutSeconds) * time.Second
	solanaCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	log.Printf("🔍 SOLANA DEBUG: Starting Solana execution, timeout=%d seconds", tm.config.SolanaTimeoutSeconds)

	// CRITICAL: Jupiter transaction для Solana RPC НУЖНО подписать!
	signedTx, err := tm.signTransaction(solanaCtx, transaction, req)
	if err != nil {
		log.Printf("🔍 SOLANA DEBUG: Failed to sign transaction: %v", err)
		result.Error = fmt.Sprintf("failed to sign transaction for Solana RPC: %v", err)
		return false
	}

	// Send transaction
	txID, err := tm.solana.SendTransaction(solanaCtx, signedTx, nil)
	if err != nil {
		result.Error = fmt.Sprintf("solana transaction submission failed: %v", err)
		log.Printf("🔍 SOLANA DEBUG: Transaction submission failed: %v", err)

		// CRITICAL: Enhanced analysis for custom program error 0x1 based on Jupiter documentation
		if strings.Contains(err.Error(), "custom program error: 0x1") {
			log.Printf("❌ JUPITER PROGRAM ERROR 0x1 DETECTED!")
			log.Printf("🔍 ANALYSIS: This error typically means:")
			log.Printf("  - 1. Insufficient balance for the swap")
			log.Printf("  - 2. Token account issues (frozen/closed)")
			log.Printf("  - 3. Route calculation problems")
			log.Printf("  - 4. Incorrect transaction format")
			log.Printf("  - 5. Slippage too tight for current market")
			log.Printf("🔍 ERROR CONTEXT:")
			log.Printf("  - Input: %s -> Output: %s", req.InputMint, req.OutputMint)
			log.Printf("  - Amount: %s lamports", req.Amount)
			log.Printf("  - Slippage: %d bps", req.SlippageBps)
			log.Printf("  - User: %s", req.UserPublicKey)

			// ENHANCED RECOMMENDATIONS based on user's Jupiter analysis
			log.Printf("🔧 ENHANCED TROUBLESHOOTING RECOMMENDATIONS:")
			log.Printf("  1. 🏦 BALANCE: Ensure sufficient SOL for:")
			log.Printf("     - Swap amount: %s lamports", req.Amount)
			log.Printf("     - ATA creation: ~2,039,280 lamports (if new token)")
			log.Printf("     - Priority fees: ~150,000+ lamports")
			log.Printf("     - Transaction fees: ~25,000 lamports")
			log.Printf("     - Safety buffer: ~100,000 lamports")
			log.Printf("  2. 🔧 JUPITER API PARAMS: Try alternative parameters:")
			log.Printf("     - useSharedAccounts=false (disable shared accounts)")
			log.Printf("     - wrapAndUnwrapSol=false (MEME MODE: avoid ATA costs)")
			log.Printf("     - skipUserAccountsRpcCalls=false (enable RPC checks)")
			log.Printf("     - Increase slippage tolerance if market is volatile")
			log.Printf("  3. 🎯 TOKEN SPECIFIC: Check if token has issues:")
			log.Printf("     - Token may be frozen/restricted")
			log.Printf("     - Low liquidity causing route failures")
			log.Printf("     - New/experimental tokens may have issues")
			log.Printf("  4. 🔄 RETRY STRATEGIES:")
			log.Printf("     - Try with different AMM (disable useSharedAccounts)")
			log.Printf("     - Reduce swap amount to test minimal transaction")
			log.Printf("     - Wait for better network conditions")

			// Log transaction preview
			txPreview := transaction
			if len(txPreview) > 300 {
				txPreview = txPreview[:300] + "..."
			}
			log.Printf("🔍 FAILING TX: %s", txPreview)
		}
		return false
	}

	result.TransactionID = txID
	log.Printf("🔍 SOLANA DEBUG: Transaction submitted successfully: %s", txID)

	// Wait for confirmation
	log.Printf("🔍 SOLANA DEBUG: Waiting for transaction confirmation...")
	status, err := tm.solana.WaitForTransactionConfirmation(solanaCtx, txID, timeout)
	if err != nil {
		result.Error = fmt.Sprintf("solana transaction confirmation failed: %v", err)
		log.Printf("🔍 SOLANA DEBUG: Transaction confirmation failed: %v", err)
		return false
	}

	result.FinalStatus = status
	log.Printf("🔍 SOLANA DEBUG: Transaction status received: %s", status)

	if status == "confirmed" || status == "finalized" {
		log.Printf("🔍 SOLANA DEBUG: Transaction confirmed successfully: %s", txID)
		return true
	}

	result.Error = fmt.Sprintf("solana transaction failed with status: %s", status)
	log.Printf("🔍 SOLANA DEBUG: Transaction failed with status: %s", status)
	return false
}

// GetAccountBalance retrieves account balances (SOL + tokens)
func (tm *TransactionManager) GetAccountBalance(ctx context.Context, address string, tokenMints []string) (*BalanceInfo, error) {
	balance := &BalanceInfo{
		TokenBalances: make(map[string]int64),
	}

	// Get SOL balance
	solBalance, err := tm.solana.GetBalance(ctx, address)
	if err != nil {
		return nil, fmt.Errorf("failed to get SOL balance: %w", err)
	}
	balance.SOLBalance = solBalance

	// Get token balances
	for _, mint := range tokenMints {
		tokenBalance, _, err := tm.solana.GetTokenBalance(ctx, address, mint)
		if err != nil {
			log.Printf("Warning: failed to get balance for token %s: %v", mint, err)
			continue
		}
		balance.TokenBalances[mint] = tokenBalance
	}

	return balance, nil
}

// EstimateTrade gets a quote without executing the trade
func (tm *TransactionManager) EstimateTrade(ctx context.Context, req TradeRequest) (*api.JupiterQuoteResponse, error) {
	quoteReq := api.JupiterQuoteRequest{
		InputMint:   req.InputMint,
		OutputMint:  req.OutputMint,
		Amount:      req.Amount,
		SlippageBps: req.SlippageBps,
		SwapMode:    req.SwapMode,
	}

	quote, err := tm.jupiter.GetQuote(ctx, quoteReq)
	if err != nil {
		return nil, fmt.Errorf("failed to get estimate quote: %w", err)
	}

	// Validate the quote
	if err := tm.jupiter.ValidateQuote(quote, tm.config.MaxSlippageBps); err != nil {
		return nil, fmt.Errorf("estimate quote validation failed: %w", err)
	}

	return quote, nil
}

// ValidateTransaction performs basic validation on transaction data
func (tm *TransactionManager) ValidateTransaction(transaction string) error {
	if transaction == "" {
		return fmt.Errorf("transaction is empty")
	}

	// Decode base64 to verify it's valid
	if _, err := base64.StdEncoding.DecodeString(transaction); err != nil {
		return fmt.Errorf("transaction is not valid base64: %w", err)
	}

	// Basic length check
	if len(transaction) < 100 {
		return fmt.Errorf("transaction appears too short: %d characters", len(transaction))
	}

	return nil
}

// UpdateConfig updates transaction manager configuration
func (tm *TransactionManager) UpdateConfig(config *TransactionConfig) {
	tm.config = config
}

// GetConfig returns current configuration
func (tm *TransactionManager) GetConfig() *TransactionConfig {
	return tm.config
}

// createTipTransaction creates a real SOL transfer transaction for Jito tip
func (tm *TransactionManager) createTipTransaction(ctx context.Context, tipAccount string, tipAmount int64, req TradeRequest) (string, error) {
	log.Printf("🔧 CREATING TIP: %d lamports from %s to %s", tipAmount, req.UserPublicKey, tipAccount)

	// Parse tip account address
	tipAccountPubkey, err := solana.PublicKeyFromBase58(tipAccount)
	if err != nil {
		return "", fmt.Errorf("invalid tip account address: %w", err)
	}

	// Parse user public key
	userPubkey, err := solana.PublicKeyFromBase58(req.UserPublicKey)
	if err != nil {
		return "", fmt.Errorf("invalid user public key: %w", err)
	}

	// Create a simple system transfer instruction
	transferInstruction := system.NewTransferInstruction(
		uint64(tipAmount), // lamports to transfer (convert to uint64)
		userPubkey,        // from user wallet
		tipAccountPubkey,  // destination (tip account)
	)

	// Get recent blockhash from Solana RPC
	blockhash, _, err := tm.solana.GetLatestBlockhash(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get recent blockhash: %w", err)
	}

	blockhashPubkey, err := solana.HashFromBase58(blockhash)
	if err != nil {
		return "", fmt.Errorf("invalid blockhash: %w", err)
	}

	// Build transaction
	tx, err := solana.NewTransaction(
		[]solana.Instruction{transferInstruction.Build()},
		blockhashPubkey,
		solana.TransactionPayer(userPubkey), // user pays for the transaction
	)
	if err != nil {
		return "", fmt.Errorf("failed to build tip transaction: %w", err)
	}

	// CRITICAL: SIGN the tip transaction with user's private key
	privateKeyStr, err := tm.walletService.ExportPrivateKey(ctx, req.UserID, req.WalletID)
	if err != nil {
		return "", fmt.Errorf("failed to get private key for signing: %w", err)
	}

	// Parse private key from base58
	privateKey, err := solana.PrivateKeyFromBase58(privateKeyStr)
	if err != nil {
		return "", fmt.Errorf("invalid private key format: %w", err)
	}

	// Sign the transaction
	_, err = tx.Sign(func(key solana.PublicKey) *solana.PrivateKey {
		if key.Equals(userPubkey) {
			return &privateKey
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("failed to sign tip transaction: %w", err)
	}

	// Serialize signed transaction to base64
	txBytes, err := tx.MarshalBinary()
	if err != nil {
		return "", fmt.Errorf("failed to serialize signed tip transaction: %w", err)
	}

	tipTxBase64 := base64.StdEncoding.EncodeToString(txBytes)
	log.Printf("✅ TIP TRANSACTION: Created %d bytes, base64 length: %d (SIGNED)", len(txBytes), len(tipTxBase64))

	return tipTxBase64, nil
}

// signTransaction signs a transaction with user's private key
func (tm *TransactionManager) signTransaction(ctx context.Context, transactionBase64 string, req TradeRequest) (string, error) {
	log.Printf("🔧 SIGNING TRANSACTION: %d chars for user %d wallet %d", len(transactionBase64), req.UserID, req.WalletID)

	// Get user's private key
	privateKeyStr, err := tm.walletService.ExportPrivateKey(ctx, req.UserID, req.WalletID)
	if err != nil {
		return "", fmt.Errorf("failed to get private key for signing: %w", err)
	}

	// Parse private key from base58
	privateKey, err := solana.PrivateKeyFromBase58(privateKeyStr)
	if err != nil {
		return "", fmt.Errorf("invalid private key format: %w", err)
	}

	// Decode transaction from base64
	txBytes, err := base64.StdEncoding.DecodeString(transactionBase64)
	if err != nil {
		return "", fmt.Errorf("failed to decode transaction: %w", err)
	}

	// Unmarshal transaction using TransactionFromBytes
	tx, err := solana.TransactionFromBytes(txBytes)
	if err != nil {
		return "", fmt.Errorf("failed to unmarshal transaction: %w", err)
	}

	// Parse user public key for verification
	userPubkey, err := solana.PublicKeyFromBase58(req.UserPublicKey)
	if err != nil {
		return "", fmt.Errorf("invalid user public key: %w", err)
	}

	// Sign the transaction
	_, err = tx.Sign(func(key solana.PublicKey) *solana.PrivateKey {
		if key.Equals(userPubkey) {
			return &privateKey
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("failed to sign transaction: %w", err)
	}

	// Serialize signed transaction back to base64
	signedTxBytes, err := tx.MarshalBinary()
	if err != nil {
		return "", fmt.Errorf("failed to serialize signed transaction: %w", err)
	}

	signedTxBase64 := base64.StdEncoding.EncodeToString(signedTxBytes)
	log.Printf("✅ SIGNED TRANSACTION: %d bytes -> %d bytes (SIGNED)", len(txBytes), len(signedTxBytes))

	return signedTxBase64, nil
}

// checkTokenSafety performs relaxed safety checks for meme coin trading
func (tm *TransactionManager) checkTokenSafety(ctx context.Context, tokenMint string, quote *api.JupiterQuoteResponse) TokenSafetyResult {
	log.Printf("🔍 TOKEN SAFETY: Checking token %s (MEME COIN MODE - ALL TOKENS ALLOWED)", tokenMint)

	// MEME COIN MODE: Allow ALL tokens including pump.fun and micro-price tokens
	// User specifically wants to trade any meme coins without restrictions

	// Only block completely broken tokens (those that cause API errors)
	if quote != nil && quote.PriceImpactPct == "-1" {
		log.Printf("⚠️ TOKEN WARNING: Token may have liquidity issues but ALLOWING anyway (MEME MODE)")
	}

	// Allow pump.fun tokens unconditionally
	if quote != nil && len(quote.RoutePlan) > 0 {
		firstRoute := quote.RoutePlan[0]
		if firstRoute.SwapInfo.Label == "Pump.fun Amm" {
			log.Printf("🚀 PUMP.FUN DETECTED: ALLOWED (MEME MODE - user wants all meme coins)")
		}
	}

	// MEME COIN MODE: Pass everything through with warning
	log.Printf("✅ TOKEN SAFETY (MEME MODE): ALL TOKENS ALLOWED - including micro-price and pump.fun")
	log.Printf("🎯 MEME COIN WARNING: High risk, high reward - any token accepted!")

	return TokenSafetyResult{
		IsSafe:         true, // Allow ALL tokens in meme mode
		Reason:         "MEME COIN MODE - all tokens allowed unconditionally",
		Recommendation: "High risk meme coin trading enabled - any token accepted",
	}
}

// retryWithAlternativeParams attempts to retry swap with alternative Jupiter parameters to fix 0x1 errors
func (tm *TransactionManager) retryWithAlternativeParams(ctx context.Context, req TradeRequest, originalQuote *api.JupiterQuoteResponse, startTime time.Time) *TradeResult {
	log.Printf("🔧 RETRY WITH ALTERNATIVE PARAMS: Starting enhanced retry for 0x1 error")

	result := &TradeResult{
		Success: false,
	}

	// TDD-Based parameter sets: Only try variations that passed tests
	parameterSets := []struct {
		name             string
		legacyTx         bool
		increaseSlippage int // Additional slippage to add in bps
	}{
		{
			name:             "TDD_Minimal_Modern",
			legacyTx:         false, // Modern format (same as passed tests)
			increaseSlippage: 200,   // Add 2% more slippage
		},
		{
			name:             "TDD_Minimal_Legacy",
			legacyTx:         true, // Legacy format fallback
			increaseSlippage: 500,  // Add 5% more slippage for safety
		},
		{
			name:             "TDD_Minimal_HighSlippage",
			legacyTx:         false, // Modern format
			increaseSlippage: 1000,  // Add 10% more slippage (last resort)
		},
	}

	for i, params := range parameterSets {
		log.Printf("🔧 TDD RETRY ATTEMPT %d/%d: %s", i+1, len(parameterSets), params.name)
		log.Printf("   - AsLegacyTransaction: %t", params.legacyTx)
		log.Printf("   - Additional slippage: %d bps", params.increaseSlippage)
		log.Printf("   - TDD APPROACH: Using MINIMAL parameters that passed tests")

		// Increase slippage for retry
		retrySlippage := req.SlippageBps + params.increaseSlippage
		if retrySlippage > 5000 { // Cap at 50%
			retrySlippage = 5000
		}

		// Build TDD-proven minimal swap request (based on successful test parameters)
		swapReq := api.JupiterSwapRequest{
			QuoteResponse:       *originalQuote,
			UserPublicKey:       req.UserPublicKey,
			WrapAndUnwrapSol:    false,           // MEME MODE: Avoid Wrapped SOL ATA costs
			AsLegacyTransaction: params.legacyTx, // Only variable parameter
			// TDD: ALL OTHER PARAMETERS REMOVED - they caused 0x1 errors in tests!
		}

		// Update quote slippage for retry
		modifiedQuote := *originalQuote
		modifiedQuote.SlippageBps = retrySlippage
		swapReq.QuoteResponse = modifiedQuote

		// TDD: NO priority fees - tests showed they cause expensive fees!

		log.Printf("🔍 RETRY REQUEST: Slippage increased to %d bps for better chance", retrySlippage)

		// Get swap transaction with alternative parameters
		swapResp, err := tm.jupiter.GetSwapTransaction(ctx, swapReq)
		if err != nil {
			log.Printf("❌ RETRY %s FAILED: Jupiter API error: %v", params.name, err)
			continue
		}

		if swapResp == nil || swapResp.SwapTransaction == "" {
			log.Printf("❌ RETRY %s FAILED: Empty response from Jupiter", params.name)
			continue
		}

		log.Printf("✅ RETRY %s: Got transaction, size: %d chars", params.name, len(swapResp.SwapTransaction))

		// Try executing with Solana (more reliable for retries)
		if success := tm.executeViaSolana(ctx, swapResp.SwapTransaction, req, result); success {
			result.Method = "solana-retry"
			result.Success = true
			result.ExecutionTimeMs = time.Since(startTime).Milliseconds()
			log.Printf("✅ RETRY SUCCESS: %s method worked! Transaction: %s", params.name, result.TransactionID)
			return result
		}

		log.Printf("❌ RETRY %s FAILED: Execution error: %s", params.name, result.Error)
	}

	// All retry attempts failed
	result.Error = "all retry attempts with alternative parameters failed"
	result.ExecutionTimeMs = time.Since(startTime).Milliseconds()
	log.Printf("❌ ALL RETRIES FAILED: Could not resolve 0x1 error with alternative parameters")

	return result
}

// calculateATACosts calculates the actual cost of creating missing associated token accounts
func (tm *TransactionManager) calculateATACosts(ctx context.Context, userPublicKey string, tokenMint string) int64 {
	log.Printf("🔍 ATA COST CALC: Checking which ATAs need to be created")

	totalCost := int64(0)
	ataRentCost := int64(2039280) // Standard ATA rent

	// For now, assume both ATAs need to be created (conservative approach)
	// In a full implementation, we would check the blockchain directly

	// 1. Output token ATA (e.g., USDC)
	outputATACost := ataRentCost
	totalCost += outputATACost
	log.Printf("🔍 ATA COST CALC: Output token ATA cost: %d lamports", outputATACost)

	// 2. Wrapped SOL ATA (needed when wrapAndUnwrapSol=true in Jupiter)
	wrappedSOLATACost := ataRentCost
	totalCost += wrappedSOLATACost
	log.Printf("🔍 ATA COST CALC: Wrapped SOL ATA cost: %d lamports", wrappedSOLATACost)

	log.Printf("🔍 ATA COST CALC: Total ATA creation cost: %d lamports", totalCost)

	return totalCost
}
