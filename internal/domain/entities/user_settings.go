package entities

import (
	"time"
)

// UserSettings represents user trading preferences and defaults
type UserSettings struct {
	UserID           int64    `json:"user_id"`
	BuySlippage      float64  `json:"buy_slippage"`
	SellSlippage     float64  `json:"sell_slippage"`
	PriorityFee      int64    `json:"priority_fee"`
	JitoTip          int64    `json:"jito_tip"`
	DefaultSOLAmount float64  `json:"default_sol_amount"`
	SellPreset25     *float64 `json:"sell_preset_25,omitempty"`
	SellPreset50     *float64 `json:"sell_preset_50,omitempty"`
	SellPreset75     *float64 `json:"sell_preset_75,omitempty"`
	SellPreset100    *float64 `json:"sell_preset_100,omitempty"`
	// Buy presets for customizable purchase amounts in SOL
	BuyPreset1 *float64  `json:"buy_preset_1,omitempty"`
	BuyPreset2 *float64  `json:"buy_preset_2,omitempty"`
	BuyPreset3 *float64  `json:"buy_preset_3,omitempty"`
	BuyPreset4 *float64  `json:"buy_preset_4,omitempty"`
	BuyPreset5 *float64  `json:"buy_preset_5,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// NewUserSettings creates new UserSettings with default values
func NewUserSettings(userID int64) *UserSettings {
	now := time.Now()
	preset25 := 25.0
	preset50 := 50.0
	preset75 := 75.0
	preset100 := 100.0

	// Default buy presets: 0.01, 0.05, 0.1, 0.5, 1 SOL
	buyPreset1 := 0.01
	buyPreset2 := 0.05
	buyPreset3 := 0.1
	buyPreset4 := 0.5
	buyPreset5 := 1.0

	return &UserSettings{
		UserID:           userID,
		BuySlippage:      15.0,        // 15% default (как требовал пользователь)
		SellSlippage:     1.0,         // 1% default
		PriorityFee:      1000,        // 1000 lamports default
		JitoTip:          100000,      // 100000 lamports default
		DefaultSOLAmount: 0.1,         // 0.1 SOL default
		SellPreset25:     &preset25,   // 25%
		SellPreset50:     &preset50,   // 50%
		SellPreset75:     &preset75,   // 75%
		SellPreset100:    &preset100,  // 100%
		BuyPreset1:       &buyPreset1, // 0.01 SOL
		BuyPreset2:       &buyPreset2, // 0.05 SOL
		BuyPreset3:       &buyPreset3, // 0.1 SOL
		BuyPreset4:       &buyPreset4, // 0.5 SOL
		BuyPreset5:       &buyPreset5, // 1.0 SOL
		CreatedAt:        now,
		UpdatedAt:        now,
	}
}

// UpdateSlippage updates slippage settings. Use 0 to keep current value.
func (us *UserSettings) UpdateSlippage(buySlippage, sellSlippage float64) {
	if buySlippage > 0 {
		us.BuySlippage = buySlippage
	}
	if sellSlippage > 0 {
		us.SellSlippage = sellSlippage
	}
	us.UpdatedAt = time.Now()
}

// UpdateFees updates priority fee and jito tip
func (us *UserSettings) UpdateFees(priorityFee, jitoTip int64) {
	us.PriorityFee = priorityFee
	us.JitoTip = jitoTip
	us.UpdatedAt = time.Now()
}

// UpdateDefaultSOLAmount updates the default SOL amount for purchases
func (us *UserSettings) UpdateDefaultSOLAmount(amount float64) {
	us.DefaultSOLAmount = amount
	us.UpdatedAt = time.Now()
}

// UpdateSellPresets updates all sell preset percentages
func (us *UserSettings) UpdateSellPresets(preset25, preset50, preset75, preset100 *float64) {
	us.SellPreset25 = preset25
	us.SellPreset50 = preset50
	us.SellPreset75 = preset75
	us.SellPreset100 = preset100
	us.UpdatedAt = time.Now()
}

// UpdateBuyPresets updates all buy preset amounts
func (us *UserSettings) UpdateBuyPresets(preset1, preset2, preset3, preset4, preset5 *float64) {
	us.BuyPreset1 = preset1
	us.BuyPreset2 = preset2
	us.BuyPreset3 = preset3
	us.BuyPreset4 = preset4
	us.BuyPreset5 = preset5
	us.UpdatedAt = time.Now()
}

// UpdateSingleBuyPreset updates a specific buy preset by index (1-5)
func (us *UserSettings) UpdateSingleBuyPreset(index int, amount *float64) {
	switch index {
	case 1:
		us.BuyPreset1 = amount
	case 2:
		us.BuyPreset2 = amount
	case 3:
		us.BuyPreset3 = amount
	case 4:
		us.BuyPreset4 = amount
	case 5:
		us.BuyPreset5 = amount
	}
	us.UpdatedAt = time.Now()
}

// UpdateSingleSellPreset updates a specific sell preset by index (1-4)
func (us *UserSettings) UpdateSingleSellPreset(index int, amount *float64) {
	switch index {
	case 1:
		us.SellPreset25 = amount
	case 2:
		us.SellPreset50 = amount
	case 3:
		us.SellPreset75 = amount
	case 4:
		us.SellPreset100 = amount
	}
	us.UpdatedAt = time.Now()
}

// GetTotalFeeInLamports returns total fee (priority + jito tip) in lamports
func (us *UserSettings) GetTotalFeeInLamports() int64 {
	return us.PriorityFee + us.JitoTip
}

// GetTotalFeeInSOL returns total fee in SOL (1 SOL = 1,000,000,000 lamports)
func (us *UserSettings) GetTotalFeeInSOL() float64 {
	return float64(us.GetTotalFeeInLamports()) / 1_000_000_000
}

// GetActivePresets returns a map of active (non-nil) sell presets
func (us *UserSettings) GetActivePresets() map[string]float64 {
	presets := make(map[string]float64)

	if us.SellPreset25 != nil {
		presets["25%"] = *us.SellPreset25
	}
	if us.SellPreset50 != nil {
		presets["50%"] = *us.SellPreset50
	}
	if us.SellPreset75 != nil {
		presets["75%"] = *us.SellPreset75
	}
	if us.SellPreset100 != nil {
		presets["100%"] = *us.SellPreset100
	}

	return presets
}

// GetActiveBuyPresets returns a map of active (non-nil) buy presets with their SOL amounts
func (us *UserSettings) GetActiveBuyPresets() map[string]float64 {
	presets := make(map[string]float64)

	if us.BuyPreset1 != nil {
		presets["preset_1"] = *us.BuyPreset1
	}
	if us.BuyPreset2 != nil {
		presets["preset_2"] = *us.BuyPreset2
	}
	if us.BuyPreset3 != nil {
		presets["preset_3"] = *us.BuyPreset3
	}
	if us.BuyPreset4 != nil {
		presets["preset_4"] = *us.BuyPreset4
	}
	if us.BuyPreset5 != nil {
		presets["preset_5"] = *us.BuyPreset5
	}

	return presets
}

// GetBuyPresets returns all buy presets as a slice of pointers
func (us *UserSettings) GetBuyPresets() []*float64 {
	return []*float64{
		us.BuyPreset1,
		us.BuyPreset2,
		us.BuyPreset3,
		us.BuyPreset4,
		us.BuyPreset5,
	}
}

// GetBuyPresetByIndex returns buy preset value by index (1-5)
func (us *UserSettings) GetBuyPresetByIndex(index int) *float64 {
	switch index {
	case 1:
		return us.BuyPreset1
	case 2:
		return us.BuyPreset2
	case 3:
		return us.BuyPreset3
	case 4:
		return us.BuyPreset4
	case 5:
		return us.BuyPreset5
	default:
		return nil
	}
}

// IsHighSlippage checks if slippage settings are considered high (>5%)
func (us *UserSettings) IsHighSlippage() bool {
	return us.BuySlippage > 5.0 || us.SellSlippage > 5.0
}

// IsHighFees checks if fee settings are considered high (>0.01 SOL total)
func (us *UserSettings) IsHighFees() bool {
	return us.GetTotalFeeInSOL() > 0.01
}

// ResetToDefaults resets all settings to default values
func (us *UserSettings) ResetToDefaults() {
	preset25 := 25.0
	preset50 := 50.0
	preset75 := 75.0
	preset100 := 100.0

	// Default buy presets
	buyPreset1 := 0.01
	buyPreset2 := 0.05
	buyPreset3 := 0.1
	buyPreset4 := 0.5
	buyPreset5 := 1.0

	us.BuySlippage = 15.0 // 15% default (как требовал пользователь)
	us.SellSlippage = 1.0
	us.PriorityFee = 1000
	us.JitoTip = 100000
	us.DefaultSOLAmount = 0.1
	us.SellPreset25 = &preset25
	us.SellPreset50 = &preset50
	us.SellPreset75 = &preset75
	us.SellPreset100 = &preset100
	us.BuyPreset1 = &buyPreset1
	us.BuyPreset2 = &buyPreset2
	us.BuyPreset3 = &buyPreset3
	us.BuyPreset4 = &buyPreset4
	us.BuyPreset5 = &buyPreset5
	us.UpdatedAt = time.Now()
}

// Validate performs validation on UserSettings
func (us *UserSettings) Validate() error {
	if us.UserID <= 0 {
		return NewValidationError("user_id", "must be positive")
	}

	// Slippage validation (1% to 5000% for high-risk trading)
	if us.BuySlippage < 1.0 || us.BuySlippage > 5000.0 {
		return NewValidationError("buy_slippage", "must be between 1.0 and 5000.0")
	}
	if us.SellSlippage < 1.0 || us.SellSlippage > 5000.0 {
		return NewValidationError("sell_slippage", "must be between 1.0 and 5000.0")
	}

	// Fee validation (non-negative)
	if us.PriorityFee < 0 {
		return NewValidationError("priority_fee", "cannot be negative")
	}
	if us.JitoTip < 0 {
		return NewValidationError("jito_tip", "cannot be negative")
	}

	// Default SOL amount validation (positive)
	if us.DefaultSOLAmount <= 0 {
		return NewValidationError("default_sol_amount", "must be positive")
	}

	// Sell preset validation (0-100% when not nil)
	if us.SellPreset25 != nil && (*us.SellPreset25 <= 0 || *us.SellPreset25 > 100) {
		return NewValidationError("sell_preset_25", "must be between 0 and 100")
	}
	if us.SellPreset50 != nil && (*us.SellPreset50 <= 0 || *us.SellPreset50 > 100) {
		return NewValidationError("sell_preset_50", "must be between 0 and 100")
	}
	if us.SellPreset75 != nil && (*us.SellPreset75 <= 0 || *us.SellPreset75 > 100) {
		return NewValidationError("sell_preset_75", "must be between 0 and 100")
	}
	if us.SellPreset100 != nil && (*us.SellPreset100 <= 0 || *us.SellPreset100 > 100) {
		return NewValidationError("sell_preset_100", "must be between 0 and 100")
	}

	// Buy preset validation (0.001-100 SOL when not nil)
	buyPresets := []*float64{us.BuyPreset1, us.BuyPreset2, us.BuyPreset3, us.BuyPreset4, us.BuyPreset5}
	presetNames := []string{"buy_preset_1", "buy_preset_2", "buy_preset_3", "buy_preset_4", "buy_preset_5"}

	for i, preset := range buyPresets {
		if preset != nil && (*preset < 0.001 || *preset > 100.0) {
			return NewValidationError(presetNames[i], "must be between 0.001 and 100.0")
		}
	}

	return nil
}
