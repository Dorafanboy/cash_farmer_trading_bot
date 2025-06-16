package sessions

import (
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// FlowType represents the type of user flow
type FlowType string

const (
	FlowNone             FlowType = "none"
	FlowWalletCreate     FlowType = "wallet_create"
	FlowWalletImport     FlowType = "wallet_import"
	FlowWalletDelete     FlowType = "wallet_delete"
	FlowWalletExport     FlowType = "wallet_export"
	FlowWalletSetPrimary FlowType = "wallet_set_primary"
	FlowTokenBuy         FlowType = "token_buy"
	FlowTokenSell        FlowType = "token_sell"
	FlowCustomAmount     FlowType = "custom_amount"
	FlowSettings         FlowType = "settings"
	// Settings edit flows
	FlowBuyAmountEdit    FlowType = "buy_amount_edit"
	FlowBuySlippageEdit  FlowType = "buy_slippage_edit"
	FlowBuyPresetEdit    FlowType = "buy_preset_edit"
	FlowSellSlippageEdit FlowType = "sell_slippage_edit"
	FlowSellPresetEdit   FlowType = "sell_preset_edit"
	FlowPriorityFeeEdit  FlowType = "priority_fee_edit"
	FlowJitoTipEdit      FlowType = "jito_tip_edit"
)

// UserState represents the current state of a user session
type UserState struct {
	UserID      int64                  `json:"user_id"`
	CurrentFlow FlowType               `json:"current_flow"`
	Step        int                    `json:"step"`
	Data        map[string]interface{} `json:"data"`
	LastMessage *tgbotapi.Message      `json:"last_message"`
	CreatedAt   time.Time              `json:"created_at"`
}
