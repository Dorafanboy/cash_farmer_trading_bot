package jito

import (
	"encoding/json"
	"fmt"
	"time"
)

// BundleStatus статус Jito bundle
type BundleStatus string

const (
	BundleStatusPending   BundleStatus = "Pending"
	BundleStatusFailed    BundleStatus = "Failed"
	BundleStatusProcessed BundleStatus = "Processed"
	BundleStatusDropped   BundleStatus = "Dropped"
	BundleStatusInflight  BundleStatus = "Inflight"
	BundleStatusLanded    BundleStatus = "Landed"
	BundleStatusUnknown   BundleStatus = "Unknown"
)

// TipAccount структура tip account из main.go
type TipAccount struct {
	Account   string `json:"account"`
	PublicKey string `json:"publickey"`
}

// GetTipAccountsResponse ответ от Jito API для tip accounts
type GetTipAccountsResponse []TipAccount

// BundleRequest запрос на отправку bundle
type BundleRequest struct {
	Jsonrpc string      `json:"jsonrpc"`
	ID      int         `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params"`
}

// BundleResponse ответ от Jito API
type BundleResponse struct {
	Jsonrpc string      `json:"jsonrpc"`
	ID      int         `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *RPCError   `json:"error,omitempty"`
}

// RPCError ошибка от Jito RPC
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// BundleStatusResponse ответ для проверки статуса bundle
type BundleStatusResponse struct {
	Jsonrpc string `json:"jsonrpc"`
	ID      int    `json:"id"`
	Result  struct {
		Context struct {
			Slot int64 `json:"slot"`
		} `json:"context"`
		Value struct {
			BundleID           string       `json:"bundle_id"`
			Transactions       []string     `json:"transactions"`
			Slot               int64        `json:"slot"`
			ConfirmationStatus BundleStatus `json:"confirmation_status"`
			Err                interface{}  `json:"err"`
		} `json:"value"`
	} `json:"result"`
	Error *RPCError `json:"error,omitempty"`
}

// SendBundleParams параметры для отправки bundle
type SendBundleParams struct {
	EncodedTransactions []string `json:"encodedTransactions"`
}

// GetBundleStatusesParams параметры для проверки статуса
type GetBundleStatusesParams struct {
	BundleIDs []string `json:"bundle_ids"`
}

// BundleResult результат операции с bundle
type BundleResult struct {
	BundleID    string       `json:"bundle_id"`
	Status      BundleStatus `json:"status"`
	Slot        int64        `json:"slot,omitempty"`
	Error       string       `json:"error,omitempty"`
	Endpoint    string       `json:"endpoint"`
	ProcessedAt time.Time    `json:"processed_at"`
}

// ParallelBundleResult результат параллельной отправки bundle
type ParallelBundleResult struct {
	SuccessfulResults []BundleResult `json:"successful_results"`
	FailedResults     []BundleResult `json:"failed_results"`
	FastestEndpoint   string         `json:"fastest_endpoint"`
	TotalDuration     time.Duration  `json:"total_duration"`
}

// TipAccountResult результат получения tip accounts
type TipAccountResult struct {
	TipAccounts  []TipAccount  `json:"tip_accounts"`
	Endpoint     string        `json:"endpoint"`
	ResponseTime time.Duration `json:"response_time"`
	Error        string        `json:"error,omitempty"`
}

// CreateBundleRequest создает RPC запрос для отправки bundle
func CreateBundleRequest(id int, transactions []string) *BundleRequest {
	return &BundleRequest{
		Jsonrpc: "2.0",
		ID:      id,
		Method:  "sendBundle",
		Params: map[string]interface{}{
			"encodedTransactions": transactions,
		},
	}
}

// CreateBundleStatusRequest создает RPC запрос для проверки статуса
func CreateBundleStatusRequest(id int, bundleIDs []string) *BundleRequest {
	return &BundleRequest{
		Jsonrpc: "2.0",
		ID:      id,
		Method:  "getBundleStatuses",
		Params: map[string]interface{}{
			"value": bundleIDs,
		},
	}
}

// CreateTipAccountsRequest создает RPC запрос для получения tip accounts
func CreateTipAccountsRequest(id int) *BundleRequest {
	return &BundleRequest{
		Jsonrpc: "2.0",
		ID:      id,
		Method:  "getTipAccounts",
		Params:  []interface{}{},
	}
}

// ParseBundleResponse парсит ответ от Jito API
func ParseBundleResponse(data []byte) (*BundleResponse, error) {
	var response BundleResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

// ParseTipAccountsResponse парсит ответ с tip accounts
func ParseTipAccountsResponse(data []byte) ([]TipAccount, error) {
	var response struct {
		Jsonrpc string       `json:"jsonrpc"`
		ID      int          `json:"id"`
		Result  []TipAccount `json:"result"`
		Error   *RPCError    `json:"error,omitempty"`
	}

	if err := json.Unmarshal(data, &response); err != nil {
		return nil, err
	}

	if response.Error != nil {
		return nil, &JitoAPIError{
			Code:    response.Error.Code,
			Message: response.Error.Message,
		}
	}

	return response.Result, nil
}

// ParseBundleStatusResponse парсит ответ со статусом bundle
func ParseBundleStatusResponse(data []byte) (*BundleStatusResponse, error) {
	var response BundleStatusResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

// JitoAPIError кастомная ошибка Jito API
type JitoAPIError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *JitoAPIError) Error() string {
	return fmt.Sprintf("Jito API ошибка %d: %s", e.Code, e.Message)
}

// IsLanded проверяет, успешно ли обработан bundle
func (s BundleStatus) IsLanded() bool {
	return s == BundleStatusLanded || s == BundleStatusProcessed
}

// IsFailed проверяет, провалился ли bundle
func (s BundleStatus) IsFailed() bool {
	return s == BundleStatusFailed || s == BundleStatusDropped
}

// IsPending проверяет, обрабатывается ли bundle
func (s BundleStatus) IsPending() bool {
	return s == BundleStatusPending || s == BundleStatusInflight
}
