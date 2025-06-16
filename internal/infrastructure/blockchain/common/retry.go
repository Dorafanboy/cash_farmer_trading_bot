package common

import (
	"context"
	"fmt"
	"time"
)

// RetryConfig конфигурация для retry механизма
type RetryConfig struct {
	MaxRetries int
	BaseDelay  time.Duration
	MaxDelay   time.Duration
}

// DefaultRetryConfig возвращает стандартную конфигурацию retry
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries: 3,
		BaseDelay:  1 * time.Second,
		MaxDelay:   10 * time.Second,
	}
}

// AggressiveRetryConfig возвращает агрессивную конфигурацию для HFT
func AggressiveRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries: 5,
		BaseDelay:  100 * time.Millisecond,
		MaxDelay:   1 * time.Second,
	}
}

// RetryableFunc тип функции которую можно повторить
type RetryableFunc func() error

// RetryableWithResultFunc функция с результатом
type RetryableWithResultFunc[T any] func() (T, error)

// WithRetry выполняет функцию с повторами при ошибках
func WithRetry(ctx context.Context, config RetryConfig, fn RetryableFunc) error {
	var lastErr error

	for attempt := 0; attempt <= config.MaxRetries; attempt++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		err := fn()
		if err == nil {
			return nil
		}

		lastErr = err

		if attempt < config.MaxRetries {
			delay := calculateDelay(attempt, config)
			time.Sleep(delay)
		}
	}

	return fmt.Errorf("все попытки исчерпаны после %d попыток, последняя ошибка: %w", config.MaxRetries+1, lastErr)
}

// WithRetryAndResult выполняет функцию с результатом и повторами
func WithRetryAndResult[T any](ctx context.Context, config RetryConfig, fn RetryableWithResultFunc[T]) (T, error) {
	var lastErr error
	var zeroValue T

	for attempt := 0; attempt <= config.MaxRetries; attempt++ {
		select {
		case <-ctx.Done():
			return zeroValue, ctx.Err()
		default:
		}

		result, err := fn()
		if err == nil {
			return result, nil
		}

		lastErr = err

		if attempt < config.MaxRetries {
			delay := calculateDelay(attempt, config)
			time.Sleep(delay)
		}
	}

	return zeroValue, fmt.Errorf("все попытки исчерпаны после %d попыток, последняя ошибка: %w", config.MaxRetries+1, lastErr)
}

// WithParallelRetry выполняет функцию параллельно через несколько источников
func WithParallelRetry[T any](ctx context.Context, sources []string, fn func(source string) (T, error)) (T, error) {
	type result struct {
		value  T
		source string
		err    error
	}

	resultChan := make(chan result, len(sources))
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Запускаем запросы параллельно
	for _, source := range sources {
		go func(src string) {
			value, err := fn(src)
			select {
			case resultChan <- result{value: value, source: src, err: err}:
			case <-ctx.Done():
			}
		}(source)
	}

	var allErrors []string
	var zeroValue T
	successCount := 0
	failureCount := 0

	// Ждем первый успешный результат
	for i := 0; i < len(sources); i++ {
		select {
		case res := <-resultChan:
			if res.err == nil {
				successCount++
				cancel() // Отменяем остальные запросы
				//fmt.Printf("✅ Успешный запрос к %s (попытка %d/%d)\n", res.source, i+1, len(sources))
				return res.value, nil
			}
			failureCount++
			allErrors = append(allErrors, fmt.Sprintf("%s: %v", res.source, res.err))
			//fmt.Printf("❌ Ошибка запроса к %s: %v (%d/%d)\n", res.source, res.err, failureCount, len(sources))
		case <-ctx.Done():
			return zeroValue, fmt.Errorf("контекст отменен: %v", ctx.Err())
		}
	}

	// Создаем детальное сообщение об ошибке
	errorSummary := fmt.Sprintf("все параллельные попытки неудачны (%d из %d источников):", failureCount, len(sources))
	for _, errMsg := range allErrors {
		errorSummary += fmt.Sprintf("\n  - %s", errMsg)
	}

	return zeroValue, fmt.Errorf(errorSummary)
}

// calculateDelay рассчитывает задержку для retry с exponential backoff
func calculateDelay(attempt int, config RetryConfig) time.Duration {
	delay := config.BaseDelay * time.Duration(1<<attempt) // exponential backoff
	if delay > config.MaxDelay {
		return config.MaxDelay
	}
	return delay
}

// IsRetryableError проверяет, можно ли повторить запрос при данной ошибке
func IsRetryableError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()

	// Сетевые ошибки - можно повторить
	retryableErrors := []string{
		"connection refused",
		"timeout",
		"context deadline exceeded",
		"network is unreachable",
		"temporary failure",
		"i/o timeout",
		"broken pipe",
		"connection reset",
		"too many requests",
		"rate limit",
		"wsarecv", // Windows specific network error
		"wsasend", // Windows specific network error
		"EOF",
		"no such host",
		"503", // Service Unavailable
		"502", // Bad Gateway
		"504", // Gateway Timeout
		"429", // Too Many Requests
		"connection attempt failed",
	}

	for _, retryable := range retryableErrors {
		if contains(errStr, retryable) {
			return true
		}
	}

	return false
}

// contains проверяет содержит ли строка подстроку (case-insensitive)
func contains(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr ||
			(len(s) > len(substr) &&
				indexOfSubstring(s, substr) >= 0))
}

// indexOfSubstring ищет подстроку в строке
func indexOfSubstring(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
