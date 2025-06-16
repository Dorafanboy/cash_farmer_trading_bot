package solana

import (
	"context"
	"fmt"
	"regexp"
	"sync"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"

	"cash-farmer/internal/infrastructure/blockchain/common"
	"cash-farmer/internal/infrastructure/blockchain/config"
)

// SolanaService интерфейс для работы с Solana RPC
type SolanaService interface {
	// GetLatestBlockhash получает последний blockhash
	GetLatestBlockhash(ctx context.Context) (*rpc.GetLatestBlockhashResult, error)

	// SendTransaction отправляет транзакцию
	SendTransaction(ctx context.Context, transaction []byte) (solana.Signature, error)

	// GetTransaction получает информацию о транзакции
	GetTransaction(ctx context.Context, signature solana.Signature) (*rpc.GetTransactionResult, error)

	// GetBalance получает баланс аккаунта
	GetBalance(ctx context.Context, account solana.PublicKey) (uint64, error)

	// GetTokenBalance получает баланс токена
	GetTokenBalance(ctx context.Context, tokenAccount solana.PublicKey) (*rpc.UiTokenAmount, error)

	// GetAssociatedTokenAddress получает адрес ATA
	GetAssociatedTokenAddress(owner, mint solana.PublicKey) (solana.PublicKey, error)

	// CheckATAExists проверяет существование ATA
	CheckATAExists(ctx context.Context, ata solana.PublicKey) (bool, error)

	// DiagnoseNetworkConnectivity проверяет состояние сетевых соединений
	DiagnoseNetworkConnectivity(ctx context.Context) map[string]error
}

// solanaService реализация SolanaService
type solanaService struct {
	config        *config.BlockchainConfig
	clients       []*rpc.Client
	currentClient int
	mu            sync.RWMutex
	blockhash     *CachedBlockhash
}

// CachedBlockhash кешированный blockhash
type CachedBlockhash struct {
	Hash      solana.Hash
	Height    uint64
	Timestamp time.Time
	mu        sync.RWMutex
}

// NewSolanaService создает новый Solana service
func NewSolanaService(cfg *config.BlockchainConfig) (SolanaService, error) {
	if len(cfg.SolanaRPCURLs) == 0 {
		return nil, fmt.Errorf("нет доступных Solana RPC URLs")
	}

	var clients []*rpc.Client
	for _, url := range cfg.SolanaRPCURLs {
		client := rpc.New(url)
		clients = append(clients, client)
	}

	service := &solanaService{
		config:    cfg,
		clients:   clients,
		blockhash: &CachedBlockhash{},
	}

	// Запускаем горутину для обновления blockhash
	go service.startBlockhashUpdater()

	return service, nil
}

// startBlockhashUpdater запускает обновление blockhash в фоне
func (s *solanaService) startBlockhashUpdater() {
	ticker := time.NewTicker(s.config.BlockhashRefresh)
	defer ticker.Stop()

	// Выполняем первое обновление сразу
	s.updateBlockhash()

	consecutiveErrors := 0
	maxConsecutiveErrors := 5

	for {
		select {
		case <-ticker.C:
			if err := s.updateBlockhashWithRetry(); err != nil {
				consecutiveErrors++
				if consecutiveErrors >= maxConsecutiveErrors {
					fmt.Printf("⚠️ Критично: %d последовательных ошибок обновления blockhash. Увеличиваем интервал.\n", consecutiveErrors)
					// Временно увеличиваем интервал при проблемах
					ticker.Reset(s.config.BlockhashRefresh * 3)
				}
			} else {
				if consecutiveErrors > 0 {
					fmt.Printf("✅ Blockhash обновление восстановлено после %d ошибок\n", consecutiveErrors)
					consecutiveErrors = 0
					// Возвращаем нормальный интервал
					ticker.Reset(s.config.BlockhashRefresh)
				}
			}
		}
	}
}

// updateBlockhashWithRetry обновляет blockhash с retry логикой
func (s *solanaService) updateBlockhashWithRetry() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := s.getLatestBlockhashFromAnyClient(ctx)
	if err != nil {
		fmt.Printf("⚠️ Ошибка обновления blockhash: %v\n", err)
		return err
	}

	s.blockhash.mu.Lock()
	s.blockhash.Hash = result.Value.Blockhash
	s.blockhash.Height = result.Value.LastValidBlockHeight
	s.blockhash.Timestamp = time.Now()
	s.blockhash.mu.Unlock()

	//fmt.Printf("🔄 Blockhash обновлен: %s (height: %d)\n",
	//	result.Value.Blockhash.String(), result.Value.LastValidBlockHeight)
	return nil
}

// updateBlockhash обновляет кешированный blockhash (deprecated - используем updateBlockhashWithRetry)
func (s *solanaService) updateBlockhash() {
	s.updateBlockhashWithRetry()
}

// GetLatestBlockhash получает последний blockhash (с кешированием)
func (s *solanaService) GetLatestBlockhash(ctx context.Context) (*rpc.GetLatestBlockhashResult, error) {
	s.blockhash.mu.RLock()

	// Проверяем свежесть кеша (не старше 10 секунд)
	if time.Since(s.blockhash.Timestamp) < 10*time.Second && !s.blockhash.Hash.IsZero() {
		// Возвращаем кешированный результат
		s.blockhash.mu.RUnlock()
		// Просто получаем свежий blockhash вместо использования кеша
		return s.getLatestBlockhashFromAnyClient(ctx)
	}
	s.blockhash.mu.RUnlock()

	// Кеш устарел, получаем свежий blockhash
	return s.getLatestBlockhashFromAnyClient(ctx)
}

// getLatestBlockhashFromAnyClient получает blockhash от любого доступного клиента
func (s *solanaService) getLatestBlockhashFromAnyClient(ctx context.Context) (*rpc.GetLatestBlockhashResult, error) {
	return common.WithParallelRetry(ctx, s.getClientURLs(), func(url string) (*rpc.GetLatestBlockhashResult, error) {
		client := s.getClientByURL(url)
		if client == nil {
			return nil, fmt.Errorf("клиент не найден для URL: %s", url)
		}

		// Увеличиваем таймаут для каждого индивидуального запроса
		requestCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		defer cancel()

		result, err := client.GetLatestBlockhash(requestCtx, rpc.CommitmentFinalized)
		if err != nil {
			return nil, fmt.Errorf("ошибка получения blockhash от %s: %w", url, err)
		}

		return result, nil
	})
}

// SendTransaction отправляет транзакцию
func (s *solanaService) SendTransaction(ctx context.Context, transaction []byte) (solana.Signature, error) {
	retryConfig := common.DefaultRetryConfig()

	return common.WithRetryAndResult(ctx, retryConfig, func() (solana.Signature, error) {
		client := s.getNextClient()

		signature, err := client.SendRawTransactionWithOpts(
			ctx,
			transaction,
			rpc.TransactionOpts{
				SkipPreflight:       false,
				PreflightCommitment: rpc.CommitmentProcessed,
				MaxRetries:          &[]uint{3}[0],
			},
		)

		if err != nil {
			return solana.Signature{}, fmt.Errorf("ошибка отправки транзакции: %w", err)
		}

		return signature, nil
	})
}

// GetTransaction получает информацию о транзакции
func (s *solanaService) GetTransaction(ctx context.Context, signature solana.Signature) (*rpc.GetTransactionResult, error) {
	client := s.getNextClient()

	result, err := client.GetTransaction(
		ctx,
		signature,
		&rpc.GetTransactionOpts{
			Commitment:                     rpc.CommitmentConfirmed,
			MaxSupportedTransactionVersion: &[]uint64{0}[0],
		},
	)

	if err != nil {
		return nil, fmt.Errorf("ошибка получения транзакции: %w", err)
	}

	return result, nil
}

// GetBalance получает баланс аккаунта
func (s *solanaService) GetBalance(ctx context.Context, account solana.PublicKey) (uint64, error) {
	client := s.getNextClient()

	balance, err := client.GetBalance(ctx, account, rpc.CommitmentConfirmed)
	if err != nil {
		return 0, fmt.Errorf("ошибка получения баланса: %w", err)
	}

	return balance.Value, nil
}

// GetTokenBalance получает баланс токена
func (s *solanaService) GetTokenBalance(ctx context.Context, tokenAccount solana.PublicKey) (*rpc.UiTokenAmount, error) {
	client := s.getNextClient()

	result, err := client.GetTokenAccountBalance(ctx, tokenAccount, rpc.CommitmentConfirmed)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения баланса токена: %w", err)
	}

	return result.Value, nil
}

// GetAssociatedTokenAddress получает адрес ATA
func (s *solanaService) GetAssociatedTokenAddress(owner, mint solana.PublicKey) (solana.PublicKey, error) {
	ata, _, err := solana.FindAssociatedTokenAddress(owner, mint)
	if err != nil {
		return solana.PublicKey{}, fmt.Errorf("ошибка получения ATA адреса: %w", err)
	}
	return ata, nil
}

// CheckATAExists проверяет существование ATA
func (s *solanaService) CheckATAExists(ctx context.Context, ata solana.PublicKey) (bool, error) {
	client := s.getNextClient()

	accountInfo, err := client.GetAccountInfo(ctx, ata)
	if err != nil {
		// Если аккаунт не найден, это не ошибка - просто ATA не существует
		if err.Error() == "not found" {
			return false, nil
		}
		return false, fmt.Errorf("ошибка проверки ATA: %w", err)
	}

	// Если accountInfo пустой, значит аккаунт не существует
	return accountInfo != nil && accountInfo.Value != nil, nil
}

// getNextClient возвращает следующий доступный клиент (round-robin)
func (s *solanaService) getNextClient() *rpc.Client {
	s.mu.Lock()
	defer s.mu.Unlock()

	client := s.clients[s.currentClient]
	s.currentClient = (s.currentClient + 1) % len(s.clients)

	return client
}

// getClientURLs возвращает список URL клиентов для параллельных запросов
func (s *solanaService) getClientURLs() []string {
	return s.config.SolanaRPCURLs
}

// getClientByURL находит клиент по URL
func (s *solanaService) getClientByURL(url string) *rpc.Client {
	for i, configURL := range s.config.SolanaRPCURLs {
		if configURL == url && i < len(s.clients) {
			return s.clients[i]
		}
	}
	return nil
}

// IsValidPublicKey проверяет корректность public key
func IsValidPublicKey(key string) bool {
	// Проверяем длину и base58 символы
	if len(key) < 32 || len(key) > 44 {
		return false
	}

	// Простая проверка base58 символов
	base58Pattern := `^[1-9A-HJ-NP-Za-km-z]+$`
	matched, _ := regexp.MatchString(base58Pattern, key)
	return matched
}

// FormatLamports форматирует lamports в SOL
func FormatLamports(lamports uint64) string {
	sol := float64(lamports) / 1e9
	return fmt.Sprintf("%.9f SOL", sol)
}

// ParseSOLAmount парсит количество SOL в lamports
func ParseSOLAmount(solAmount string) (uint64, error) {
	var sol float64
	_, err := fmt.Sscanf(solAmount, "%f", &sol)
	if err != nil {
		return 0, fmt.Errorf("ошибка парсинга SOL количества: %w", err)
	}

	lamports := uint64(sol * 1e9)
	return lamports, nil
}

// DiagnoseNetworkConnectivity проверяет состояние сетевых соединений
func (s *solanaService) DiagnoseNetworkConnectivity(ctx context.Context) map[string]error {
	results := make(map[string]error)

	for _, url := range s.config.SolanaRPCURLs {
		client := s.getClientByURL(url)
		if client == nil {
			results[url] = fmt.Errorf("клиент не найден")
			continue
		}

		// Быстрая проверка соединения с коротким таймаутом
		testCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		_, err := client.GetSlot(testCtx, rpc.CommitmentFinalized)
		cancel()

		results[url] = err
	}

	return results
}
