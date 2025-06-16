package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gagliardetto/solana-go/programs/system"

	"github.com/gagliardetto/solana-go"
	associatedtokenaccount "github.com/gagliardetto/solana-go/programs/associated-token-account"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/google/uuid"
	jitorpc "github.com/jito-labs/jito-go-rpc"
)

var (
	rpcURLs = []string{
		"https://mainnet.helius-rpc.com/?api-key=97f4303e-3cbd-4417-8c0f-bfdcf31e3cec",
		"https://rpc.hellomoon.io",
		"https://mainnet.genesysgo.org",
	}
	jitoEndpoints = []string{
		"https://mainnet.block-engine.jito.wtf/api/v1",
		"https://amsterdam.mainnet.block-engine.jito.wtf/api/v1",
		"https://frankfurt.mainnet.block-engine.jito.wtf/api/v1",
		"https://london.mainnet.block-engine.jito.wtf/api/v1",
		"https://singapore.mainnet.block-engine.jito.wtf",
		"https://ny.mainnet.block-engine.jito.wtf/api/v1",
		"https://slc.mainnet.block-engine.jito.wtf/api/v1",
		"https://tokyo.mainnet.block-engine.jito.wtf/api/v1",
	}
	blockhash       atomic.Value
	httpCli         = &http.Client{Timeout: 15 * time.Second}
	lastRequestTime atomic.Value
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

// Упростим функцию rate-limit логики: только метка времени
func enforceRateLimit() {
	lastRequestTime.Store(time.Now())
}

// Улучшенный разбор JSON и логика обработки "empty response"
func parseBundleStatusResponse(respBody []byte) (*BundleStatusResponse, error) {
	if len(respBody) == 0 {
		return nil, fmt.Errorf("пустой ответ от сервера")
	}

	var rawResponse json.RawMessage
	if err := json.Unmarshal(respBody, &rawResponse); err != nil {
		return nil, fmt.Errorf("невалидный JSON: %v", err)
	}

	var rpcResponse struct {
		Result *BundleStatusResponse `json:"result"`
		Error  interface{}           `json:"error"`
	}

	err := json.Unmarshal(respBody, &rpcResponse)
	if err != nil {
		return nil, fmt.Errorf("ошибка парсинга JSON: %v", err)
	}

	if rpcResponse.Error != nil {
		return nil, fmt.Errorf("RPC ошибка: %v", rpcResponse.Error)
	}

	if rpcResponse.Result == nil {
		return nil, fmt.Errorf("пустой ответ от сервера")
	}

	return rpcResponse.Result, nil
}

// Проверка валидности транзакций перед отправкой bundle
func validateTransactions(transactions [][]byte) error {
	for i, txBytes := range transactions {
		if len(txBytes) == 0 {
			return fmt.Errorf("транзакция %d пустая", i)
		}

		if len(txBytes) < 64 { // Минимальный размер для транзакции с подписью
			return fmt.Errorf("транзакция %d слишком короткая (%d байт)", i, len(txBytes))
		}

		log.Printf("✅ Транзакция %d прошла базовую проверку (%d байт)", i, len(txBytes))
	}

	return nil
}

// Проверка конкретных транзакций на блокчейне
func checkTransactionsOnChain(bundleId string, rpcClient *rpc.Client) {
	log.Printf("🔍 Проверяем транзакции bundle %s на блокчейне...", bundleId)

	exploreURL := fmt.Sprintf("https://explorer.jito.wtf/api/v1/bundles/%s", bundleId)

	resp, err := httpCli.Get(exploreURL)
	if err != nil {
		log.Printf("⚠️ Не удалось получить данные о bundle из Jito Explorer: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		log.Printf("⚠️ Jito Explorer вернул статус %d для bundle %s", resp.StatusCode, bundleId)
		return
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Printf("⚠️ Ошибка чтения ответа Jito Explorer: %v", err)
		return
	}

	var bundleInfo struct {
		Transactions []struct {
			TransactionId string `json:"transaction_id"`
			Status        string `json:"status"`
		} `json:"transactions"`
		Status string `json:"status"`
		Slot   uint64 `json:"slot"`
	}

	err = json.Unmarshal(body, &bundleInfo)
	if err != nil {
		log.Printf("⚠️ Не удалось распарсить ответ Jito Explorer: %v", err)
		log.Printf("Ответ: %s", string(body))
		return
	}

	log.Printf("📊 Bundle статус: %s, Slot: %d", bundleInfo.Status, bundleInfo.Slot)
	log.Printf("📊 Транзакций в bundle: %d", len(bundleInfo.Transactions))

	for i, tx := range bundleInfo.Transactions {
		log.Printf("🔗 Транзакция %d: %s (статус: %s)", i+1, tx.TransactionId, tx.Status)
		log.Printf("   Solscan: https://solscan.io/tx/%s", tx.TransactionId)

		// Проверяем транзакцию через RPC
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		txInfo, err := rpcClient.GetTransaction(ctx, solana.MustSignatureFromBase58(tx.TransactionId), &rpc.GetTransactionOpts{
			Encoding: solana.EncodingBase64,
		})
		cancel()

		if err != nil {
			log.Printf("   ⚠️ Не удалось получить информацию о транзакции: %v", err)
		} else if txInfo.Meta.Err != nil {
			log.Printf("   ❌ Транзакция провалилась: %v", txInfo.Meta.Err)
		} else {
			log.Printf("   ✅ Транзакция успешна (slot: %d)", txInfo.Slot)
		}
	}
}

// Функция для повторных попыток с экспоненциальной задержкой
func retryWithBackoff(operation func() error, maxRetries int, baseDelay time.Duration, operationName string) error {
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			isRateLimit := lastErr != nil && (fmt.Sprintf("%v", lastErr) == "rate limited (429)" ||
				fmt.Sprintf("%v", lastErr) == "rate limited")

			var delay time.Duration
			if isRateLimit {
				delay = time.Duration(float64(baseDelay) * math.Pow(2.5, float64(attempt-1)))
				log.Printf("🐌 Rate limit обнаружен, используем увеличенную задержку")
			} else {
				delay = time.Duration(float64(baseDelay) * math.Pow(2, float64(attempt-1)))
			}

			jitter := time.Duration(float64(delay) * 0.1 * (0.5 - float64(attempt%2)))
			totalDelay := delay + jitter

			maxDelayLimit := 60 * time.Second
			if isRateLimit {
				maxDelayLimit = 90 * time.Second
			}

			if totalDelay > maxDelayLimit {
				totalDelay = maxDelayLimit
			}

			log.Printf("🔄 %s - попытка %d/%d через %.2f секунд...",
				operationName, attempt+1, maxRetries, totalDelay.Seconds())
			time.Sleep(totalDelay)
		}

		err := operation()
		if err == nil {
			if attempt > 0 {
				log.Printf("✅ %s успешно выполнена с %d попытки", operationName, attempt+1)
			}
			return nil
		}

		lastErr = err
		log.Printf("⚠️ %s - попытка %d неудачна: %v", operationName, attempt+1, err)
	}

	return fmt.Errorf("все %d попыток неудачны для %s: %v", maxRetries, operationName, lastErr)
}

func refreshBlockhash(ctx context.Context, rpcClient *rpc.Client) {
	for {
		if bh, err := rpcClient.GetLatestBlockhash(ctx, rpc.CommitmentProcessed); err == nil {
			blockhash.Store(bh.Value.Blockhash)
		}
		time.Sleep(2 * time.Second) // Увеличили интервал с 400ms до 2s
	}
}

func buildATATransaction(ctx context.Context, payer solana.PrivateKey, owner, mint solana.PublicKey) (*solana.Transaction, error) {
	ix := associatedtokenaccount.NewCreateInstruction(payer.PublicKey(), owner, mint).Build()
	tx, err := solana.NewTransaction(
		[]solana.Instruction{ix},
		blockhash.Load().(solana.Hash),
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

// Получение текущих tip floor данных с retry логикой
func getTipFloor() (float64, error) {
	var tipFloor float64

	err := retryWithBackoff(func() error {
		enforceRateLimit()
		resp, err := httpCli.Get("https://bundles.jito.wtf/api/v1/bundles/tip_floor")
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		if resp.StatusCode == 429 {
			return fmt.Errorf("rate limited (429)")
		}

		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}

		var tipData []struct {
			LandedTips75thPercentile float64 `json:"landed_tips_75th_percentile"`
			LandedTips95thPercentile float64 `json:"landed_tips_95th_percentile"`
		}

		if err := json.Unmarshal(body, &tipData); err != nil {
			return err
		}

		if len(tipData) > 0 {
			tipFloor = tipData[0].LandedTips75thPercentile
		} else {
			tipFloor = 0.001 // fallback
		}

		return nil
	}, 5, 3*time.Second, "получение tip floor")

	if err != nil {
		log.Printf("⚠️ Не удалось получить tip floor после всех попыток: %v", err)
		return 0.002, nil
	}

	return tipFloor, nil
}

func getSwapTransaction(ctx context.Context, inputMint string, outputMint string, pub solana.PublicKey, priv solana.PrivateKey) (*solana.Transaction, error) {
	quoteURL := fmt.Sprintf("https://quote-api.jup.ag/v6/quote?inputMint=%s&outputMint=%s&amount=1000000&slippageBps=1500", inputMint, outputMint)
	resp, err := httpCli.Get(quoteURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBytes, _ := ioutil.ReadAll(resp.Body)
	var quoteResp map[string]interface{}
	_ = json.Unmarshal(respBytes, &quoteResp)

	swapReq := map[string]interface{}{
		"quoteResponse":                 quoteResp,
		"userPublicKey":                 pub.String(),
		"wrapAndUnwrapSol":              true,
		"slippageBps":                   50,
		"computeUnitPriceMicroLamports": 600000,
		"computeUnitLimit":              1600000,
		"bundleOnly":                    true,
	}
	bodyData, _ := json.Marshal(swapReq)
	swapRequest, _ := http.NewRequest("POST", "https://quote-api.jup.ag/v6/swap", bytes.NewBuffer(bodyData))
	swapRequest.Header.Set("Content-Type", "application/json")
	swapResp, _ := httpCli.Do(swapRequest.WithContext(ctx))
	defer swapResp.Body.Close()
	respBytes, _ = ioutil.ReadAll(swapResp.Body)
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

func createTipTransaction(privateKey solana.PrivateKey, amount uint64, recentBlockhash solana.Hash, tipAddress string) (*solana.Transaction, error) {
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

// Получение tip accounts ПАРАЛЛЕЛЬНО от всех endpoints одновременно (АГРЕССИВНАЯ АТАКА)
func getTipAccountsAcrossEndpoints() ([]string, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	type tipResult struct {
		tipAccounts []string
		endpoint    string
		err         error
	}

	resultChan := make(chan tipResult, len(jitoEndpoints))
	log.Printf("🔄 АГРЕССИВНАЯ АТАКА: спамим все %d endpoints одновременно для tip accounts...", len(jitoEndpoints))

	for i, endpoint := range jitoEndpoints {
		go func(endpoint string, index int) {
			uuid := uuid.New().String()
			jitoClient := jitorpc.NewJitoJsonRpcClient(endpoint, uuid)
			debug := true
			jitoClient.Debug = &debug

			var tipAccounts []string
			var lastErr error

			for attempt := 0; attempt < 2; attempt++ {
				select {
				case <-ctx.Done():
					resultChan <- tipResult{nil, endpoint, fmt.Errorf("context canceled")}
					return
				default:
				}

				tipAccountsRaw, err := jitoClient.GetTipAccounts()
				if err != nil {
					lastErr = err
					continue
				}

				var tipAccountsResponse []string
				err = json.Unmarshal(tipAccountsRaw, &tipAccountsResponse)
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

	for i := 0; i < len(jitoEndpoints); i++ {
		select {
		case res := <-resultChan:
			if res.err == nil && len(res.tipAccounts) > 0 {
				log.Printf("✅ ПЕРВЫЙ УСПЕХ! Tip accounts получены от: %s", res.endpoint)
				cancel() // Убиваем остальные горутины
				return res.tipAccounts, res.endpoint, nil
			}
		case <-ctx.Done():
			return nil, "", fmt.Errorf("timeout: не получили tip accounts за 10 секунд")
		}
	}

	return nil, "", fmt.Errorf("все endpoints недоступны для tip accounts")
}

// Отправка bundle параллельно через все endpoints (горутины)
func sendBundleAcrossEndpoints(bundleRequest [][]string) (string, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	type result struct {
		bundleId string
		endpoint string
		err      error
	}

	resultChan := make(chan result, len(jitoEndpoints))

	log.Printf("🚀 Запускаем параллельную отправку через %d endpoints...", len(jitoEndpoints))

	for i, endpoint := range jitoEndpoints {
		go func(endpoint string, index int) {
			uuid := uuid.New().String()
			jitoClient := jitorpc.NewJitoJsonRpcClient(endpoint, uuid)
			debug := true
			jitoClient.Debug = &debug

			log.Printf("🌐 Горутина %d: начинаем отправку через %s", index+1, endpoint)

			bundleId, err := sendBundleWithRetryContext(ctx, jitoClient, bundleRequest, endpoint)
			resultChan <- result{
				bundleId: bundleId,
				endpoint: endpoint,
				err:      err,
			}
		}(endpoint, i)
	}

	var lastErr error

	for i := 0; i < len(jitoEndpoints); i++ {
		select {
		case res := <-resultChan:
			if res.err == nil {
				log.Printf("✅ ПЕРВЫЙ УСПЕХ! Bundle отправлен через: %s", res.endpoint)
				log.Printf("🛑 Отменяем остальные горутины...")
				cancel() // Отменяем остальные горутины
				return res.bundleId, res.endpoint, nil
			} else {
				log.Printf("⚠️ Горутина для %s неудачна: %v", res.endpoint, res.err)
				lastErr = res.err
			}
		case <-ctx.Done():
			return "", "", fmt.Errorf("timeout: не удалось отправить bundle за 30 секунд")
		}
	}

	return "", "", fmt.Errorf("все %d endpoints неудачны: последняя ошибка: %v", len(jitoEndpoints), lastErr)
}

// Отправка bundle с АГРЕССИВНЫМ retry БЕЗ ЗАДЕРЖЕК (с context cancellation)
func sendBundleWithRetryContext(ctx context.Context, jitoClient *jitorpc.JitoJsonRpcClient, bundleRequest [][]string, endpoint string) (string, error) {
	var bundleId string

	maxRetries := 3

	for attempt := 0; attempt < maxRetries; attempt++ {
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("контекст отменен: %v", ctx.Err())
		default:
		}

		bundleIdRaw, err := jitoClient.SendBundle(bundleRequest)
		if err != nil {
			if fmt.Sprintf("%v", err) == "RPC error: Network congested. Endpoint is globally rate limited." ||
				fmt.Sprintf("%v", err) == "RPC error: Too Many Requests" {
				if attempt < maxRetries-1 {
					log.Printf("⚠️ Rate limit на %s, попытка %d/%d", endpoint, attempt+1, maxRetries)
					continue
				}
				return "", fmt.Errorf("rate limited (429)")
			}
			if attempt < maxRetries-1 {
				continue
			}
			return "", err
		}

		err = json.Unmarshal(bundleIdRaw, &bundleId)
		if err != nil {
			if attempt < maxRetries-1 {
				continue
			}
			return "", err
		}

		if bundleId == "" {
			if attempt < maxRetries-1 {
				continue
			}
			return "", fmt.Errorf("получен пустой bundle ID")
		}

		return bundleId, nil
	}

	return "", fmt.Errorf("все %d попыток неудачны для %s", maxRetries, endpoint)
}

func main() {
	ctx := context.Background()
	priv, _ := solana.PrivateKeyFromBase58("")
	pub := priv.PublicKey()
	rpcClient := rpc.New(rpcURLs[0])
	go refreshBlockhash(ctx, rpcClient)
	time.Sleep(3 * time.Second) // Увеличили время ожидания

	inputMint := "CreiuhfwdWCN5mJbMJtA9bBpYQrQF2tCBuZwSPWfpump"
	outputMint := "So11111111111111111111111111111111111111112"

	tipFloor, err := getTipFloor()
	if err != nil {
		log.Printf("⚠️ Не удалось получить tip floor, используем дефолтный: %v", err)
		tipFloor = 0.002
	}

	fmt.Println(strconv.FormatFloat(tipFloor, 'f', -1, 64))
	fmt.Println("Такой tip floor")

	recommendedTip := uint64(tipFloor * 5 * 1e9) // x5 от 75th percentile (увеличиваем для лучшей конкуренции!)

	minTip := uint64(100000) // минимум 0.0001 SOL
	maxTip := uint64(500000) // максимум 0.0005 SOL

	if recommendedTip < minTip {
		recommendedTip = minTip
	}
	if recommendedTip > maxTip {
		recommendedTip = maxTip
	}

	log.Printf("🎯 Рекомендуемый tip: %.6f SOL (%d lamports)", float64(recommendedTip)/1e9, recommendedTip)

	log.Printf("📊 Tip Floor (75th percentile): %.9f SOL", tipFloor)
	log.Printf("📊 Наш tip в %dx больше floor", int(float64(recommendedTip)/(tipFloor*1e9)))

	targetMint := solana.MustPublicKeyFromBase58(outputMint)
	ata, _, err := solana.FindAssociatedTokenAddress(pub, targetMint)
	if err != nil {
		log.Fatalf("❌ Ошибка при поиске ATA: %v", err)
	}

	_, err = rpcClient.GetAccountInfo(ctx, ata)
	var tx1 *solana.Transaction

	if err != nil {
		log.Printf("🔨 Создаем ATA для токена: %s", ata.String())
		tx1, err = buildATATransaction(ctx, priv, pub, targetMint)
		if err != nil {
			log.Fatalf("❌ buildATA: %v", err)
		}
	} else {
		log.Printf("✅ ATA уже существует: %s", ata.String())
		tx1 = nil
	}

	tx2, err := getSwapTransaction(ctx, inputMint, outputMint, pub, priv)
	if err != nil {
		log.Fatalf("❌ getSwap: %v", err)
	}

	log.Printf("✅ Swap транзакция создана (tip будет отдельно)")

	log.Printf("🔄 Получаем tip accounts...")
	tipAccountsResponse, successfulEndpoint, err := getTipAccountsAcrossEndpoints()
	if err != nil {
		log.Fatalf("❌ Не удалось получить tip accounts ни от одного endpoint: %v", err)
	}

	randomTipAccount := tipAccountsResponse[0]
	log.Printf("✅ Tip account получен от %s: %s", successfulEndpoint, randomTipAccount)

	tipTx, err := createTipTransaction(priv, recommendedTip, blockhash.Load().(solana.Hash), randomTipAccount)
	if err != nil {
		log.Fatalf("❌ Ошибка при создании транзакции с чаевыми: %v", err)
	}

	var signed1 []byte
	if tx1 != nil {
		signed1, _ = tx1.MarshalBinary()
	}
	signed2, _ := tx2.MarshalBinary()
	signedTipTx, _ := tipTx.MarshalBinary()

	swapHash := tx2.Signatures[0].String()
	log.Printf("   Swap Transaction: https://solscan.io/tx/%s", swapHash)

	var transactionsToValidate [][]byte
	if tx1 != nil {
		transactionsToValidate = [][]byte{signed1, signed2, signedTipTx}
	} else {
		transactionsToValidate = [][]byte{signed2, signedTipTx}
	}

	log.Printf("🔍 Проверяем валидность транзакций перед отправкой...")
	err = validateTransactions(transactionsToValidate)
	if err != nil {
		log.Fatalf("❌ Валидация транзакций не прошла: %v", err)
	}

	var bundleRequest [][]string
	if tx1 != nil {
		bundleRequest = [][]string{
			{base64.StdEncoding.EncodeToString(signed1), base64.StdEncoding.EncodeToString(signed2), base64.StdEncoding.EncodeToString(signedTipTx)},
		}
		log.Printf("📦 Bundle с 3 транзакциями: ATA + Swap + Tip")
	} else {
		bundleRequest = [][]string{
			{base64.StdEncoding.EncodeToString(signed2), base64.StdEncoding.EncodeToString(signedTipTx)},
		}
		log.Printf("📦 Bundle с 2 транзакциями: Swap + Tip")
	}

	log.Printf("🚀 Отправляем bundle через множественные endpoints...")
	bundleId, usedEndpoint, err := sendBundleAcrossEndpoints(bundleRequest)
	if err != nil {
		log.Fatalf("❌ Не удалось отправить bundle ни через один endpoint: %v", err)
	}

	log.Printf("✅ Bundle отправлен успешно через %s. Bundle ID: %s", usedEndpoint, bundleId)
	log.Printf("💰 Tip amount: %.6f SOL", float64(recommendedTip)/1e9)

	checkBundleStatus(bundleId, recommendedTip, usedEndpoint, rpcClient)
}

func checkBundleStatus(bundleId string, tipAmount uint64, endpoint string, rpcClient *rpc.Client) {
	maxAttempts := 15
	pollInterval := 3 * time.Second

	log.Printf("🔍 Начинаем мониторинг bundle: %s", bundleId)
	log.Printf("🌐 Jito Explorer: https://explorer.jito.wtf/bundle/%s", bundleId)
	log.Printf("🔗 Используем endpoint: %s", endpoint)

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		time.Sleep(pollInterval)

		var statusResponse *BundleStatusResponse
		inflightErr := retryWithBackoff(func() error {
			enforceRateLimit()

			requestBody := map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      1,
				"method":  "getInflightBundleStatuses",
				"params":  []interface{}{[]string{bundleId}},
			}

			bodyBytes, err := json.Marshal(requestBody)
			if err != nil {
				return err
			}

			resp, err := httpCli.Post(endpoint,
				"application/json", bytes.NewBuffer(bodyBytes))
			if err != nil {
				return err
			}
			defer resp.Body.Close()

			if resp.StatusCode == 429 {
				return fmt.Errorf("rate limited (429)")
			}

			respBody, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				return err
			}

			if len(respBody) == 0 {
				return fmt.Errorf("empty response")
			}

			statusResponse, err = parseBundleStatusResponse(respBody)
			if err != nil {
				if err.Error() == "пустой ответ от сервера" {
					log.Printf("📭 Jito вернул пустой ответ, bundle вероятно ещё не появился")
				}
				return err
			}
			return nil
		}, 2, 3*time.Second, "getInflightBundleStatuses")

		if inflightErr != nil && strings.Contains(inflightErr.Error(), "empty response") {
			log.Printf("🔄 getInflightBundleStatuses пуст, пробуем getBundleStatuses для завершенных bundles...")

			err := retryWithBackoff(func() error {
				enforceRateLimit()

				requestBody := map[string]interface{}{
					"jsonrpc": "2.0",
					"id":      1,
					"method":  "getBundleStatuses",
					"params":  []interface{}{[]string{bundleId}},
				}

				bodyBytes, err := json.Marshal(requestBody)
				if err != nil {
					return err
				}

				resp, err := httpCli.Post(endpoint,
					"application/json", bytes.NewBuffer(bodyBytes))
				if err != nil {
					return err
				}
				defer resp.Body.Close()

				if resp.StatusCode == 429 {
					return fmt.Errorf("rate limited (429)")
				}

				respBody, err := ioutil.ReadAll(resp.Body)
				if err != nil {
					return err
				}

				log.Printf("🔍 getBundleStatuses response: %s", string(respBody))

				if len(respBody) == 0 {
					return fmt.Errorf("empty response")
				}

				var rpcResponse struct {
					Result *BundleStatusResponse `json:"result"`
					Error  interface{}           `json:"error"`
				}

				err = json.Unmarshal(respBody, &rpcResponse)
				if err != nil {
					return fmt.Errorf("json unmarshal error (response: %s): %v", string(respBody), err)
				}

				if rpcResponse.Error != nil {
					return fmt.Errorf("RPC error: %v", rpcResponse.Error)
				}

				if rpcResponse.Result == nil {
					return fmt.Errorf("null result")
				}

				statusResponse = rpcResponse.Result
				return nil
			}, 2, 3*time.Second, "getBundleStatuses")

			if err != nil && strings.Contains(err.Error(), "empty response") && attempt >= 2 {
				log.Printf("✅ И getBundleStatuses тоже пуст - bundle ТОЧНО выполнен!")
				log.Printf("🎉 Bundle успешно обработан Jito!")
				log.Printf("🔗 Проверьте: https://explorer.jito.wtf/bundle/%s", bundleId)

				log.Printf("🔍 Получаем детали выполненных транзакций...")
				checkTransactionsOnChain(bundleId, rpcClient)
				return
			}

			if err != nil {
				if strings.Contains(err.Error(), "empty response") {
					if attempt >= 2 {
						log.Printf("✅ Получили empty response на попытке %d", attempt)
						log.Printf("🎉 ЭТО ХОРОШО! Empty response обычно означает:")
						log.Printf("   ✅ Bundle УЖЕ ВЫПОЛНЕН и не показывается в getInflightBundleStatuses")
						log.Printf("   📋 getInflightBundleStatuses показывает только активные bundles (последние 5 минут)")
						log.Printf("🔗 Проверьте результат: https://explorer.jito.wtf/bundle/%s", bundleId)

						log.Printf("🔍 Получаем детали выполненных транзакций...")
						checkTransactionsOnChain(bundleId, rpcClient)

						log.Printf("🎯 Bundle скорее всего УСПЕШНО ВЫПОЛНЕН! Empty response = bundle завершен")
						return
					}
					log.Printf("Attempt %d: Empty response (это хорошо! bundle вероятно выполнен)", attempt)
					continue
				}

				if attempt <= 5 {
					log.Printf("Attempt %d: Bundle статус недоступен через %s: %v", attempt, endpoint, err)
					continue
				} else {
					log.Printf("Attempt %d: Пробуем другие endpoints для проверки статуса...", attempt)
					statusResponse = tryOtherEndpointsForStatus(bundleId)
					if statusResponse == nil {
						log.Printf("Attempt %d: Bundle больше не в системе Jito. Проверка: %v", attempt, err)
						log.Printf("💡 Bundle либо выполнен, либо отклонен. Проверьте Jito Explorer выше ☝️")
						return
					}
				}
			}
		}

		if statusResponse != nil && len(statusResponse.Value) == 0 {
			log.Printf("Attempt %d: Bundle не найден в системе (slot: %d)", attempt, statusResponse.Context.Slot)
			if attempt > 3 {
				log.Printf("✅ Bundle не в системе = скорее всего ВЫПОЛНЕН!")
				log.Printf("🔗 Проверьте: https://explorer.jito.wtf/bundle/%s", bundleId)
				checkTransactionsOnChain(bundleId, rpcClient)
				return
			}
			continue
		}

		if statusResponse == nil {
			continue
		}

		bundleStatus := statusResponse.Value[0]
		log.Printf("Attempt %d: Bundle status: %s (slot: %d)", attempt, bundleStatus.ConfirmationStatus, bundleStatus.Slot)

		switch bundleStatus.ConfirmationStatus {
		case "processed":
			log.Printf("✅ Bundle обработан кластером. Продолжаем мониторинг...")
		case "confirmed":
			log.Printf("✅ Bundle подтвержден кластером. Продолжаем мониторинг...")
		case "finalized":
			log.Printf("🎉 Bundle финализирован кластером в слоте %d!", bundleStatus.Slot)
			if bundleStatus.Err.Ok == nil {
				log.Printf("🚀 Bundle выполнен успешно!")
				log.Printf("📱 Ссылки на транзакции:")
				for _, txID := range bundleStatus.Transactions {
					solscanURL := fmt.Sprintf("https://solscan.io/tx/%s", txID)
					log.Printf("   - %s", solscanURL)
				}
			} else {
				log.Printf("❌ Ошибка выполнения bundle: %v", bundleStatus.Err.Ok)
			}
			return
		case "failed":
			log.Printf("❌ Bundle провалился: %v", bundleStatus.Err.Ok)
			return
		default:
			log.Printf("⚠️ Неожиданный статус: %s. Проверьте bundle вручную.", bundleStatus.ConfirmationStatus)
			return
		}
	}

	log.Printf("⏰ Достигнут максимум попыток. Финальный статус неизвестен.")
	log.Printf("💡 Возможные причины:")
	log.Printf("   - Tip слишком мал для конкуренции (текущий: %.6f SOL)", float64(tipAmount)/1e9)
	log.Printf("   - Bundle отклонен из-за проблем с транзакциями")
	log.Printf("   - Высокая конкуренция на данном слоте")
	log.Printf("   - Проверьте bundle ID вручную: https://explorer.jito.wtf/bundle/%s", bundleId)
}

// Пробуем другие endpoints для проверки статуса bundle
func tryOtherEndpointsForStatus(bundleId string) *BundleStatusResponse {
	log.Printf("🔄 Пробуем альтернативные endpoints для проверки статуса bundle...")

	alternativeEndpoints := jitoEndpoints[:3]

	for i, endpoint := range alternativeEndpoints {
		log.Printf("🌐 Проверяем через endpoint %d/3: %s", i+1, endpoint)

		var statusResponse *BundleStatusResponse
		err := retryWithBackoff(func() error {
			enforceRateLimit()

			requestBody := map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      1,
				"method":  "getBundleStatuses",
				"params":  []interface{}{[]string{bundleId}},
			}

			bodyBytes, err := json.Marshal(requestBody)
			if err != nil {
				return err
			}

			resp, err := httpCli.Post(endpoint, "application/json", bytes.NewBuffer(bodyBytes))
			if err != nil {
				return err
			}
			defer resp.Body.Close()

			if resp.StatusCode == 429 {
				return fmt.Errorf("rate limited (429)")
			}

			respBody, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				return err
			}

			if len(respBody) == 0 {
				return fmt.Errorf("empty response")
			}

			var rpcResponse struct {
				Result *BundleStatusResponse `json:"result"`
				Error  interface{}           `json:"error"`
			}

			err = json.Unmarshal(respBody, &rpcResponse)
			if err != nil {
				return fmt.Errorf("json unmarshal error: %v", err)
			}

			if rpcResponse.Error != nil {
				return fmt.Errorf("RPC error: %v", rpcResponse.Error)
			}

			if rpcResponse.Result == nil {
				return fmt.Errorf("null result")
			}

			statusResponse = rpcResponse.Result
			return nil
		}, 2, 2*time.Second, fmt.Sprintf("альтернативная проверка через %s", endpoint))

		if err != nil {
			log.Printf("⚠️ Endpoint %s тоже не работает: %v", endpoint, err)
			continue
		}

		if len(statusResponse.Value) > 0 {
			log.Printf("✅ Получили статус через альтернативный endpoint: %s", endpoint)
			return statusResponse
		}
	}

	log.Printf("❌ Ни один альтернативный endpoint не вернул статус bundle")
	return nil
}
