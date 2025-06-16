package ui

import (
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type Builder struct{}

func NewBuilder() *Builder {
	return &Builder{}
}

func (b *Builder) BuildMainMenuKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("💰 Buy", "buy"),
			tgbotapi.NewInlineKeyboardButtonData("💸 Sell", "sell"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📊 Positions", "positions"),
			tgbotapi.NewInlineKeyboardButtonData("⚙️ Settings", "settings"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("👛 Manage Wallets", "manage_wallets"),
			tgbotapi.NewInlineKeyboardButtonData("🔄 Refresh", "refresh"),
		),
	)
}

func (b *Builder) BuildWalletManagementKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("➕ Generate", "wallet_generate"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📤 Export", "wallet_export"),
			tgbotapi.NewInlineKeyboardButtonData("🗑 Delete", "wallet_delete"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🟢 Set Primary", "wallet_set_primary"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("← Back", "back"),
			tgbotapi.NewInlineKeyboardButtonData("🏠 Menu", "menu"),
		),
	)
}

func (b *Builder) BuildBuyPresetsKeyboard(userPresets [5]*float64, slippage float64) tgbotapi.InlineKeyboardMarkup {
	// Создаем ряды кнопок динамически на основе пользовательских пресетов
	var rows [][]tgbotapi.InlineKeyboardButton

	// Первый ряд: пресеты 1-3 (если они не nil)
	row1 := []tgbotapi.InlineKeyboardButton{}
	if userPresets[0] != nil {
		row1 = append(row1, tgbotapi.NewInlineKeyboardButtonData(
			fmt.Sprintf("%.2f SOL ✏️", *userPresets[0]),
			fmt.Sprintf("buy_preset:%.6f", *userPresets[0]),
		))
	}
	if userPresets[1] != nil {
		row1 = append(row1, tgbotapi.NewInlineKeyboardButtonData(
			fmt.Sprintf("%.2f SOL ✏️", *userPresets[1]),
			fmt.Sprintf("buy_preset:%.6f", *userPresets[1]),
		))
	}
	if userPresets[2] != nil {
		row1 = append(row1, tgbotapi.NewInlineKeyboardButtonData(
			fmt.Sprintf("%.1f SOL ✏️", *userPresets[2]),
			fmt.Sprintf("buy_preset:%.6f", *userPresets[2]),
		))
	}
	if len(row1) > 0 {
		rows = append(rows, row1)
	}

	// Второй ряд: пресеты 4-5 (если они не nil)
	row2 := []tgbotapi.InlineKeyboardButton{}
	if userPresets[3] != nil {
		row2 = append(row2, tgbotapi.NewInlineKeyboardButtonData(
			fmt.Sprintf("%.2f SOL ✏️", *userPresets[3]),
			fmt.Sprintf("buy_preset:%.6f", *userPresets[3]),
		))
	}
	if userPresets[4] != nil {
		row2 = append(row2, tgbotapi.NewInlineKeyboardButtonData(
			fmt.Sprintf("%.1f SOL ✏️", *userPresets[4]),
			fmt.Sprintf("buy_preset:%.6f", *userPresets[4]),
		))
	}
	if len(row2) > 0 {
		rows = append(rows, row2)
	}

	// Третий ряд: Buy slippage кнопка
	row3 := []tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardButtonData(fmt.Sprintf("Buy slippage %.0f%% ✏️", slippage), "buy_slippage_edit"),
	}
	rows = append(rows, row3)

	// Навигационный ряд
	navRow := tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("← Back", "back"),
		tgbotapi.NewInlineKeyboardButtonData("Menu", "menu"),
	)
	rows = append(rows, navRow)

	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

func (b *Builder) BuildBackMenuKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("← Back", "back"),
			tgbotapi.NewInlineKeyboardButtonData("🏠 Menu", "menu"),
		),
	)
}

// BuildSettingsMenuKeyboard - главное меню настроек
func (b *Builder) BuildSettingsMenuKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("💰 Buy Settings", "settings_buy"),
			tgbotapi.NewInlineKeyboardButtonData("💸 Sell Settings", "settings_sell"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("⚡ Priority Fee", "settings_priority"),
			tgbotapi.NewInlineKeyboardButtonData("🛡️ MEV Protection", "settings_mev"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("← Back", "back"),
			tgbotapi.NewInlineKeyboardButtonData("🏠 Menu", "menu"),
		),
	)
}

// BuildBuySettingsKeyboard создает клавиатуру для настроек покупки с кастомизируемыми пресетами
func (b *Builder) BuildBuySettingsKeyboard(currentAmount float64, currentSlippage float64, buyPresets [5]*float64) tgbotapi.InlineKeyboardMarkup {
	// Создаем ряды для пресетов согласно референсу
	var presetRows [][]tgbotapi.InlineKeyboardButton

	// Первый ряд: пресеты 1-3 (как на скрине: 0.01, 0.05, 0.1)
	row1 := []tgbotapi.InlineKeyboardButton{}
	if buyPresets[0] != nil {
		row1 = append(row1, tgbotapi.NewInlineKeyboardButtonData(
			fmt.Sprintf("%.3f SOL ✏️", *buyPresets[0]),
			fmt.Sprintf("buy_preset_edit:1"),
		))
	} else {
		row1 = append(row1, tgbotapi.NewInlineKeyboardButtonData("0.01 SOL ✏️", "buy_preset_edit:1"))
	}
	if buyPresets[1] != nil {
		row1 = append(row1, tgbotapi.NewInlineKeyboardButtonData(
			fmt.Sprintf("%.3f SOL ✏️", *buyPresets[1]),
			fmt.Sprintf("buy_preset_edit:2"),
		))
	} else {
		row1 = append(row1, tgbotapi.NewInlineKeyboardButtonData("0.05 SOL ✏️", "buy_preset_edit:2"))
	}
	if buyPresets[2] != nil {
		row1 = append(row1, tgbotapi.NewInlineKeyboardButtonData(
			fmt.Sprintf("%.3f SOL ✏️", *buyPresets[2]),
			fmt.Sprintf("buy_preset_edit:3"),
		))
	} else {
		row1 = append(row1, tgbotapi.NewInlineKeyboardButtonData("0.1 SOL ✏️", "buy_preset_edit:3"))
	}
	presetRows = append(presetRows, row1)

	// Второй ряд: пресеты 4-5 (как на скрине: 0.02, 0.2)
	row2 := []tgbotapi.InlineKeyboardButton{}
	if buyPresets[3] != nil {
		row2 = append(row2, tgbotapi.NewInlineKeyboardButtonData(
			fmt.Sprintf("%.3f SOL ✏️", *buyPresets[3]),
			fmt.Sprintf("buy_preset_edit:4"),
		))
	} else {
		row2 = append(row2, tgbotapi.NewInlineKeyboardButtonData("0.5 SOL ✏️", "buy_preset_edit:4"))
	}
	if buyPresets[4] != nil {
		row2 = append(row2, tgbotapi.NewInlineKeyboardButtonData(
			fmt.Sprintf("%.3f SOL ✏️", *buyPresets[4]),
			fmt.Sprintf("buy_preset_edit:5"),
		))
	} else {
		row2 = append(row2, tgbotapi.NewInlineKeyboardButtonData("1.0 SOL ✏️", "buy_preset_edit:5"))
	}
	presetRows = append(presetRows, row2)

	// Третий ряд: Buy slippage кнопка с ✏️
	row3 := []tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardButtonData(fmt.Sprintf("Buy slippage %.0f%% ✏️", currentSlippage), "buy_slippage_edit"),
	}
	presetRows = append(presetRows, row3)

	// Навигационный ряд
	navRow := tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("⬅️ Back to Settings", "settings"),
	)
	presetRows = append(presetRows, navRow)

	return tgbotapi.NewInlineKeyboardMarkup(presetRows...)
}

// BuildSellSettingsKeyboard создает клавиатуру для настроек продажи
func (b *Builder) BuildSellSettingsKeyboard(currentSlippage float64, sellPresets [4]*float64) tgbotapi.InlineKeyboardMarkup {
	var presetRows [][]tgbotapi.InlineKeyboardButton

	// Первый ряд: пресеты 25%, 50%, 75%
	row1 := []tgbotapi.InlineKeyboardButton{}
	if sellPresets[0] != nil {
		row1 = append(row1, tgbotapi.NewInlineKeyboardButtonData(
			fmt.Sprintf("%.0f%% ✏️", *sellPresets[0]),
			"sell_preset_edit:25",
		))
	} else {
		row1 = append(row1, tgbotapi.NewInlineKeyboardButtonData("25% ✏️", "sell_preset_edit:25"))
	}
	if sellPresets[1] != nil {
		row1 = append(row1, tgbotapi.NewInlineKeyboardButtonData(
			fmt.Sprintf("%.0f%% ✏️", *sellPresets[1]),
			"sell_preset_edit:50",
		))
	} else {
		row1 = append(row1, tgbotapi.NewInlineKeyboardButtonData("50% ✏️", "sell_preset_edit:50"))
	}
	if sellPresets[2] != nil {
		row1 = append(row1, tgbotapi.NewInlineKeyboardButtonData(
			fmt.Sprintf("%.0f%% ✏️", *sellPresets[2]),
			"sell_preset_edit:75",
		))
	} else {
		row1 = append(row1, tgbotapi.NewInlineKeyboardButtonData("75% ✏️", "sell_preset_edit:75"))
	}
	presetRows = append(presetRows, row1)

	// Второй ряд: пресет 100% + пространство
	row2 := []tgbotapi.InlineKeyboardButton{}
	if sellPresets[3] != nil {
		row2 = append(row2, tgbotapi.NewInlineKeyboardButtonData(
			fmt.Sprintf("%.0f%% ✏️", *sellPresets[3]),
			"sell_preset_edit:100",
		))
	} else {
		row2 = append(row2, tgbotapi.NewInlineKeyboardButtonData("100% ✏️", "sell_preset_edit:100"))
	}
	presetRows = append(presetRows, row2)

	// Третий ряд: Sell slippage кнопка
	row3 := []tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardButtonData(fmt.Sprintf("Sell slippage %.0f%% ✏️", currentSlippage), "sell_slippage_edit"),
	}
	presetRows = append(presetRows, row3)

	// Навигационный ряд
	navRow := tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("⬅️ Back to Settings", "settings"),
	)
	presetRows = append(presetRows, navRow)

	return tgbotapi.NewInlineKeyboardMarkup(presetRows...)
}

// BuildPriorityFeeKeyboard создает клавиатуру для настроек Priority Fee
func (b *Builder) BuildPriorityFeeKeyboard(currentFee float64) tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📝 Edit Priority Fee", "priority_fee_edit"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("⬅️ Back to Settings", "settings"),
		),
	)
}

// BuildSlippageSettingsKeyboard создает клавиатуру для настроек slippage
func (b *Builder) BuildSlippageSettingsKeyboard(buySlippage, sellSlippage float64) tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📝 Edit Buy Slippage", "buy_slippage_edit"),
			tgbotapi.NewInlineKeyboardButtonData("📝 Edit Sell Slippage", "sell_slippage_edit"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("⬅️ Back to Settings", "settings"),
		),
	)
}

// BuildMEVSettingsKeyboard создает клавиатуру для настроек MEV защиты
func (b *Builder) BuildMEVSettingsKeyboard(enabled bool, priorityFee, jitoTip int64) tgbotapi.InlineKeyboardMarkup {
	var toggleButton tgbotapi.InlineKeyboardButton
	if enabled {
		toggleButton = tgbotapi.NewInlineKeyboardButtonData("🔴 Disable MEV", "mev_toggle_off")
	} else {
		toggleButton = tgbotapi.NewInlineKeyboardButtonData("🟢 Enable MEV", "mev_toggle_on")
	}

	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			toggleButton,
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📝 Edit Jito Tip", "jito_tip_edit"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("⬅️ Back to Settings", "settings"),
		),
	)
}

// BuildMEVEducationKeyboard - образовательный интерфейс MEV
func (b *Builder) BuildMEVEducationKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("← Back to MEV Settings", "settings_mev"),
		),
	)
}
