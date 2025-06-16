package save2

//
//import (
//	"bytes"
//	"context"
//	"encoding/base64"
//	"encoding/json"
//	"fmt"
//	associatedtokenaccount "github.com/gagliardetto/solana-go/programs/associated-token-account"
//	_ "github.com/gagliardetto/solana-go/programs/token"
//	"io/ioutil"
//	"log"
//	"net/http"
//	"time"
//
//	"github.com/gagliardetto/solana-go"
//	"github.com/gagliardetto/solana-go/rpc"
//	"github.com/google/uuid"
//)
//
//func EnsureATAExists(ctx context.Context, rpcClient *rpc.Client, owner solana.PublicKey, mint solana.PublicKey, payer solana.PrivateKey) error {
//	ata, _, err := solana.FindAssociatedTokenAddress(owner, mint)
//	if err != nil {
//		return fmt.Errorf("не удалось найти ATA: %w", err)
//	}
//
//	info, err := rpcClient.GetAccountInfo(ctx, ata)
//	if err == nil && info.Value != nil {
//		return nil // ✅ уже существует
//	}
//
//	blockhash, err := rpcClient.GetLatestBlockhash(ctx, rpc.CommitmentProcessed)
//	if err != nil {
//		return fmt.Errorf("не удалось получить blockhash: %w", err)
//	}
//
//	ix := associatedtokenaccount.NewCreateInstruction(
//		payer.PublicKey(),
//		owner,
//		mint,
//	).Build()
//
//	tx, err := solana.NewTransaction(
//		[]solana.Instruction{ix},
//		blockhash.Value.Blockhash,
//		solana.TransactionPayer(payer.PublicKey()),
//	)
//	if err != nil {
//		return fmt.Errorf("не удалось создать транзакцию: %w", err)
//	}
//
//	_, err = tx.Sign(func(key solana.PublicKey) *solana.PrivateKey {
//		if key.Equals(payer.PublicKey()) {
//			return &payer
//		}
//		return nil
//	})
//	if err != nil {
//		return fmt.Errorf("подпись не удалась: %w", err)
//	}
//
//	sig, err := rpcClient.SendTransactionWithOpts(ctx, tx, rpc.TransactionOpts{
//		SkipPreflight: true,
//	})
//	if err != nil {
//		return fmt.Errorf("не удалось отправить ATA транзакцию: %w", err)
//	}
//
//	log.Printf("✅ ATA создан: %s", sig.String())
//	return nil
//}
//
//func main() {
//	ctx := context.Background()
//
//	keyStr := ""
//	priv, err := solana.PrivateKeyFromBase58(keyStr)
//	if err != nil {
//		log.Fatalf("Неверный приватный ключ: %v", err)
//	}
//	pub := priv.PublicKey()
//
//	quoteURL := "https://quote-api.jup.ag/v6/quote" +
//		"?inputMint=So11111111111111111111111111111111111111112" +
//		"&outputMint=EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v" +
//		"&amount=1000000" +
//		"&slippageBps=50"
//
//	resp, err := http.Get(quoteURL)
//	if err != nil {
//		log.Fatalf("Quote request error: %v", err)
//	}
//	defer resp.Body.Close()
//	respBytes, err := ioutil.ReadAll(resp.Body)
//	if err != nil {
//		log.Fatalf("Ошибка чтения quote-ответа: %v", err)
//	}
//
//	log.Printf("📄 Ответ Jupiter quote: %s", string(respBytes))
//
//	var quoteResp map[string]interface{}
//	if err := json.Unmarshal(respBytes, &quoteResp); err != nil {
//		log.Fatalf("Ошибка парсинга quote-ответа: %v", err)
//	}
//
//	rpcClient := rpc.New("https://mainnet.helius-rpc.com/?api-key=97f4303e-3cbd-4417-8c0f-bfdcf31e3cec")
//
//	// Достаём outputMint из ответа Jupiter
//	outputMintStr, ok := quoteResp["outputMint"].(string)
//	if !ok {
//		log.Fatal("❌ Не удалось извлечь outputMint из quote-ответа")
//	}
//	outputMint := solana.MustPublicKeyFromBase58(outputMintStr)
//
//	// Проверка: если outputMint НЕ является SOL — создаём ATA
//	if outputMintStr != "So11111111111111111111111111111111111111112" {
//		if err := EnsureATAExists(ctx, rpcClient, pub, outputMint, priv); err != nil {
//			log.Fatalf("❌ Не удалось обеспечить ATA: %v", err)
//		}
//	}
//
//	balResp, err := rpcClient.GetBalance(ctx, pub, rpc.CommitmentFinalized)
//	if err != nil {
//		log.Fatalf("❌ Не удалось получить баланс: %v", err)
//	}
//	log.Printf("💰 Баланс: %.6f SOL", float64(balResp.Value)/1e9)
//
//	//if balResp.Value < 2000000 {
//	//	log.Fatalf("❌ Недостаточно SOL для swap (требуется > 0.002 SOL)")
//	//}
//
//	swapReq := map[string]interface{}{
//		"quoteResponse":                 quoteResp,
//		"userPublicKey":                 pub.String(),
//		"wrapAndUnwrapSol":              true,
//		"slippageBps":                   50,
//		"dynamicComputeUnitLimit":       false,
//		"computeUnitPriceMicroLamports": 200000,
//		//"prioritizationFeeLamports": 200000,
//	}
//	bodyData, _ := json.Marshal(swapReq)
//
//	swapRequest, _ := http.NewRequest("POST", "https://quote-api.jup.ag/v6/swap", bytes.NewBuffer(bodyData))
//	swapRequest.Header.Set("Content-Type", "application/json")
//	swapResp, err := http.DefaultClient.Do(swapRequest.WithContext(ctx))
//	if err != nil {
//		log.Fatalf("Ошибка запроса swap: %v", err)
//	}
//	defer swapResp.Body.Close()
//
//	respBytes, _ = ioutil.ReadAll(swapResp.Body)
//	var swapData struct {
//		SwapTransaction string `json:"swapTransaction"`
//	}
//	if err := json.Unmarshal(respBytes, &swapData); err != nil {
//		log.Fatalf("Ошибка парсинга swap-ответа: %v", err)
//	}
//	if swapData.SwapTransaction == "" {
//		log.Fatalf("Пустая swap-транзакция от Jupiter")
//	}
//
//	var tx solana.Transaction
//	if err := tx.UnmarshalBase64(swapData.SwapTransaction); err != nil {
//		log.Fatalf("Ошибка парсинга base64 транзакции: %v", err)
//	}
//
//	_, err = tx.Sign(func(p solana.PublicKey) *solana.PrivateKey {
//		if p.Equals(pub) {
//			return &priv
//		}
//		return nil
//	})
//	if err != nil {
//		log.Fatalf("Ошибка подписи: %v", err)
//	}
//
//	signedBytes, err := tx.MarshalBinary()
//	if err != nil {
//		log.Fatalf("Ошибка сериализации: %v", err)
//	}
//	signedTxB64 := base64.StdEncoding.EncodeToString(signedBytes)
//
//	log.Printf("📦 TX Base64: %s", signedTxB64[:64])
//
//	res, err := sendTxnJito("https://amsterdam.mainnet.block-engine.jito.wtf", uuid.New().String(), signedTxB64)
//	if err != nil {
//		log.Printf("❌ Jito ошибка: %v – fallback через Helius...", err)
//		//_ := uint(3)
//		sig, err := rpcClient.SendTransactionWithOpts(ctx, &tx, rpc.TransactionOpts{
//			SkipPreflight: true,
//			MaxRetries:    nil,
//		})
//		if err != nil {
//			log.Fatalf("❌ Ошибка при fallback через Helius: %v", err)
//		}
//		log.Printf("✅ Отправлено через Helius. Signature: %s", sig.String())
//		return
//	}
//
//	var bundleResult map[string]interface{}
//	if err := json.Unmarshal(res, &bundleResult); err != nil {
//		log.Fatalf("Ошибка разбора ответа Jito: %v", err)
//	}
//	log.Printf("✅ Отправлено через Jito. Ответ: %+v\n", bundleResult)
//
//	if len(tx.Signatures) == 0 {
//		log.Fatal("❌ У транзакции нет сигнатур")
//	}
//
//	signature := tx.Signatures[0].String()
//	log.Printf("🔍 Проверяем статус транзакции: %s", signature)
//
//	// Проверяем статус
//	sig, err := solana.SignatureFromBase58(signature)
//	if err != nil {
//		log.Fatalf("❌ Ошибка преобразования сигнатуры: %v", err)
//	}
//
//	statusResp, err := rpcClient.GetSignatureStatuses(ctx, true, sig)
//	if err != nil || statusResp == nil || len(statusResp.Value) == 0 || statusResp.Value[0] == nil {
//		log.Printf("❓ Статус транзакции не найден")
//	} else {
//		confirmation := statusResp.Value[0].ConfirmationStatus
//		if confirmation != "" {
//			log.Printf("✅ Транзакция подтверждена (%s). Solscan: https://solscan.io/tx/%s", confirmation, signature)
//		} else {
//			log.Printf("⚠️ Транзакция пока не подтверждена. Solscan: https://solscan.io/tx/%s", signature)
//		}
//	}
//
//	for i := 0; i < 5; i++ {
//		time.Sleep(3 * time.Second)
//		statusResp, err := rpcClient.GetSignatureStatuses(ctx, true, sig)
//		if err == nil && statusResp != nil && len(statusResp.Value) > 0 && statusResp.Value[0] != nil {
//			confirmation := statusResp.Value[0].ConfirmationStatus
//			if confirmation != "" {
//				log.Printf("✅ Транзакция подтверждена (%s). Solscan: https://solscan.io/tx/%s", confirmation, sig.String())
//				return
//			}
//		}
//		log.Printf("⏳ Попытка %d: транзакция ещё не подтверждена...", i+1)
//	}
//	log.Printf("⚠️ Транзакция не подтверждена после 5 попыток. Solscan: https://solscan.io/tx/%s", sig.String())
//}
//
//func sendTxnJito(jitoUrl, uuid, signedTxB64 string) ([]byte, error) {
//	body := map[string]interface{}{
//		"jsonrpc": "2.0",
//		"id":      1,
//		"method":  "sendTransaction",
//		"params": []interface{}{
//			signedTxB64,
//			map[string]string{"encoding": "base64"},
//		},
//	}
//	payload, _ := json.Marshal(body)
//
//	u := fmt.Sprintf("%s/api/v1/transactions", jitoUrl)
//	req, err := http.NewRequest("POST", u, bytes.NewBuffer(payload))
//	if err != nil {
//		return nil, err
//	}
//	req.Header.Set("Content-Type", "application/json")
//	req.Header.Set("x-jito-auth", uuid)
//
//	resp, err := http.DefaultClient.Do(req)
//	if err != nil {
//		return nil, err
//	}
//	defer resp.Body.Close()
//
//	if resp.StatusCode != http.StatusOK {
//		return nil, fmt.Errorf("Jito status: %s", resp.Status)
//	}
//
//	return ioutil.ReadAll(resp.Body)
//}
