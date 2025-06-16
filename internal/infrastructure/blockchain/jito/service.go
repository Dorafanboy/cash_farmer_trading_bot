package jito

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"cash-farmer/internal/infrastructure/blockchain/common"
	"cash-farmer/internal/infrastructure/blockchain/config"
)

// JitoService интерфейс для работы с Jito
type JitoService interface {
	// GetTipAccounts получает tip accounts параллельно от всех endpoints
	GetTipAccounts(ctx context.Context) ([]TipAccount, error)

	// SendBundle отправляет bundle параллельно через все endpoints
	SendBundle(ctx context.Context, transactions []string) (*ParallelBundleResult, error)

	// GetBundleStatus проверяет статус bundle
	GetBundleStatus(ctx context.Context, bundleIDs []string) ([]BundleResult, error)

	// SendBundleAggressive агрессивная отправка БЕЗ задержек (HFT режим)
	SendBundleAggressive(ctx context.Context, transactions []string) (*ParallelBundleResult, error)
}

// jitoService реализация JitoService
type jitoService struct {
	config     *config.BlockchainConfig
	httpClient *http.Client
	endpoints  []string
	mu         sync.RWMutex
}

// NewJitoService создает новый Jito service
func NewJitoService(cfg *config.BlockchainConfig) JitoService {
	return &jitoService{
		config:    cfg,
		endpoints: cfg.GetJitoEndpoints(),
		httpClient: &http.Client{
			Timeout: cfg.RequestTimeout,
		},
	}
}

// GetTipAccounts получает tip accounts параллельно от всех endpoints
func (j *jitoService) GetTipAccounts(ctx context.Context) ([]TipAccount, error) {
	startTime := time.Now()

	// Используем параллельный retry для получения tip accounts
	result, err := common.WithParallelRetry(ctx, j.endpoints, func(endpoint string) ([]TipAccount, error) {
		return j.getTipAccountsFromEndpoint(ctx, endpoint)
	})

	if err != nil {
		return nil, fmt.Errorf("не удалось получить tip accounts: %w", err)
	}

	duration := time.Since(startTime)
	fmt.Printf("🎯 Tip accounts получены за %v\n", duration)

	return result, nil
}

// SendBundle отправляет bundle параллельно через все endpoints
func (j *jitoService) SendBundle(ctx context.Context, transactions []string) (*ParallelBundleResult, error) {
	return j.sendBundleInternal(ctx, transactions, false)
}

// SendBundleAggressive агрессивная отправка БЕЗ задержек
func (j *jitoService) SendBundleAggressive(ctx context.Context, transactions []string) (*ParallelBundleResult, error) {
	return j.sendBundleInternal(ctx, transactions, true)
}

// sendBundleInternal внутренняя логика отправки bundle
func (j *jitoService) sendBundleInternal(ctx context.Context, transactions []string, aggressive bool) (*ParallelBundleResult, error) {
	startTime := time.Now()

	mode := "STANDARD"
	if aggressive {
		mode = "AGGRESSIVE"
	}

	fmt.Printf("🌐 Jito [%s]: отправляем бандл через %d endpoints\n", mode, len(j.endpoints))
	for i, ep := range j.endpoints {
		fmt.Printf("  %d. %s\n", i+1, ep)
	}

	type endpointResult struct {
		endpoint string
		result   *BundleResult
		err      error
	}

	resultChan := make(chan endpointResult, len(j.endpoints))
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var wg sync.WaitGroup

	// Параллельная отправка на все endpoints
	for _, endpoint := range j.endpoints {
		wg.Add(1)
		go func(ep string) {
			defer wg.Done()

			fmt.Printf("📡 Попытка отправки на %s...\n", ep)
			bundleResult, err := j.sendBundleToEndpoint(ctx, ep, transactions, aggressive)

			if err != nil {
				fmt.Printf("❌ Ошибка на %s: %v\n", ep, err)
			} else {
				fmt.Printf("✅ Успех на %s\n", ep)
			}

			select {
			case resultChan <- endpointResult{endpoint: ep, result: bundleResult, err: err}:
			case <-ctx.Done():
			}
		}(endpoint)
	}

	// Закрываем канал когда все горутины завершатся
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	var successResults []BundleResult
	var failedResults []BundleResult
	var fastestEndpoint string
	firstSuccess := true

	// Собираем результаты
	for result := range resultChan {
		if result.err == nil && result.result != nil {
			successResults = append(successResults, *result.result)

			// Запоминаем первый успешный endpoint как самый быстрый
			if firstSuccess {
				fastestEndpoint = result.endpoint
				firstSuccess = false

				if aggressive {
					// В агрессивном режиме останавливаемся на первом успехе
					cancel()
					break
				}
			}
		} else {
			if result.result != nil {
				failedResults = append(failedResults, *result.result)
			} else {
				// Создаем результат ошибки
				failedResults = append(failedResults, BundleResult{
					Endpoint:    result.endpoint,
					Error:       result.err.Error(),
					ProcessedAt: time.Now(),
				})
			}
		}
	}

	totalDuration := time.Since(startTime)

	bundleResult := &ParallelBundleResult{
		SuccessfulResults: successResults,
		FailedResults:     failedResults,
		FastestEndpoint:   fastestEndpoint,
		TotalDuration:     totalDuration,
	}

	if len(successResults) == 0 {
		return bundleResult, fmt.Errorf("все endpoints не смогли отправить bundle")
	}

	fmt.Printf("🚀 Bundle отправлен [%s] за %v через %s\n", mode, totalDuration, fastestEndpoint)

	return bundleResult, nil
}

// sendBundleToEndpoint отправляет bundle на конкретный endpoint
func (j *jitoService) sendBundleToEndpoint(ctx context.Context, endpoint string, transactions []string, aggressive bool) (*BundleResult, error) {
	request := CreateBundleRequest(1, transactions)

	requestBody, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("ошибка маршалинга запроса: %w", err)
	}

	// Конфигурация retry в зависимости от режима
	var retryConfig common.RetryConfig
	if aggressive {
		retryConfig = common.AggressiveRetryConfig()
	} else {
		retryConfig = common.DefaultRetryConfig()
	}

	var bundleResult *BundleResult
	err = common.WithRetry(ctx, retryConfig, func() error {
		resp, reqErr := j.makeHTTPRequest(ctx, endpoint, requestBody)
		if reqErr != nil {
			return reqErr
		}

		bundleResponse, parseErr := ParseBundleResponse(resp)
		if parseErr != nil {
			return parseErr
		}

		bundleResult = &BundleResult{
			Endpoint:    endpoint,
			ProcessedAt: time.Now(),
		}

		if bundleResponse.Error != nil {
			bundleResult.Error = bundleResponse.Error.Message
			return fmt.Errorf("Jito API ошибка: %s", bundleResponse.Error.Message)
		}

		// Парсим результат как bundle ID
		if bundleResponse.Result != nil {
			if bundleID, ok := bundleResponse.Result.(string); ok {
				bundleResult.BundleID = bundleID
				bundleResult.Status = BundleStatusPending
			}
		}

		return nil
	})

	if err != nil {
		if bundleResult == nil {
			bundleResult = &BundleResult{
				Endpoint:    endpoint,
				Error:       err.Error(),
				ProcessedAt: time.Now(),
			}
		}
		return bundleResult, err
	}

	return bundleResult, nil
}

// getTipAccountsFromEndpoint получает tip accounts от конкретного endpoint
func (j *jitoService) getTipAccountsFromEndpoint(ctx context.Context, endpoint string) ([]TipAccount, error) {
	request := CreateTipAccountsRequest(1)

	requestBody, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("ошибка маршалинга запроса: %w", err)
	}

	respBody, err := j.makeHTTPRequest(ctx, endpoint, requestBody)
	if err != nil {
		return nil, err
	}

	tipAccounts, err := ParseTipAccountsResponse(respBody)
	if err != nil {
		return nil, fmt.Errorf("ошибка парсинга tip accounts: %w", err)
	}

	return tipAccounts, nil
}

// GetBundleStatus проверяет статус bundle
func (j *jitoService) GetBundleStatus(ctx context.Context, bundleIDs []string) ([]BundleResult, error) {
	// Пробуем получить статус с первого доступного endpoint
	for _, endpoint := range j.endpoints {
		results, err := j.getBundleStatusFromEndpoint(ctx, endpoint, bundleIDs)
		if err == nil {
			return results, nil
		}
	}

	return nil, fmt.Errorf("не удалось получить статус bundle ни с одного endpoint")
}

// getBundleStatusFromEndpoint получает статус bundle от конкретного endpoint
func (j *jitoService) getBundleStatusFromEndpoint(ctx context.Context, endpoint string, bundleIDs []string) ([]BundleResult, error) {
	request := CreateBundleStatusRequest(1, bundleIDs)

	requestBody, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("ошибка маршалинга запроса: %w", err)
	}

	respBody, err := j.makeHTTPRequest(ctx, endpoint, requestBody)
	if err != nil {
		return nil, err
	}

	statusResponse, err := ParseBundleStatusResponse(respBody)
	if err != nil {
		return nil, fmt.Errorf("ошибка парсинга статуса: %w", err)
	}

	var results []BundleResult
	if statusResponse.Error == nil {
		results = append(results, BundleResult{
			BundleID:    statusResponse.Result.Value.BundleID,
			Status:      statusResponse.Result.Value.ConfirmationStatus,
			Slot:        statusResponse.Result.Value.Slot,
			Endpoint:    endpoint,
			ProcessedAt: time.Now(),
		})
	}

	return results, nil
}

// makeHTTPRequest выполняет HTTP запрос к Jito API
func (j *jitoService) makeHTTPRequest(ctx context.Context, endpoint string, requestBody []byte) ([]byte, error) {
	url := endpoint + "/bundles"

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, fmt.Errorf("ошибка создания запроса: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := j.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ошибка HTTP запроса к %s: %w", endpoint, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d от %s", resp.StatusCode, endpoint)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("ошибка чтения ответа: %w", err)
	}

	return body, nil
}
