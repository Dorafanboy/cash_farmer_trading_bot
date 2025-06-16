package services

import (
	"context"
	"fmt"
	"log"

	"cash-farmer/internal/domain/entities"
	"cash-farmer/internal/infrastructure/database/sqlc"

	"github.com/jackc/pgx/v5/pgtype"
)

// SettingsService provides business logic for user settings operations
type SettingsService interface {
	// Settings management
	GetUserSettings(ctx context.Context, userID int64) (*entities.UserSettings, error)
	CreateDefaultSettings(ctx context.Context, userID int64) (*entities.UserSettings, error)
	UpdateSettings(ctx context.Context, userID int64, settings *entities.UserSettings) error

	// Specific setting updates
	UpdateSlippage(ctx context.Context, userID int64, buySlippage, sellSlippage float64) error
	UpdateFees(ctx context.Context, userID int64, priorityFee, jitoTip int64) error
	UpdateDefaultSOLAmount(ctx context.Context, userID int64, amount float64) error
	UpdateSellPresets(ctx context.Context, userID int64, preset25, preset50, preset75, preset100 *float64) error
	UpdateBuyPresets(ctx context.Context, userID int64, preset1, preset2, preset3, preset4, preset5 *float64) error
	UpdateSingleBuyPreset(ctx context.Context, userID int64, presetIndex int, amount *float64) error
	UpdateSingleSellPreset(ctx context.Context, userID int64, presetIndex int, amount *float64) error

	// Reset operations
	ResetToDefaults(ctx context.Context, userID int64) error
	DeleteSettings(ctx context.Context, userID int64) error
}

// SettingsServiceImpl implements SettingsService
type SettingsServiceImpl struct {
	db *sqlc.Queries
}

// NewSettingsService creates a new settings service
func NewSettingsService(db *sqlc.Queries) SettingsService {
	return &SettingsServiceImpl{
		db: db,
	}
}

// GetUserSettings retrieves user settings, creating defaults if not exist
func (ss *SettingsServiceImpl) GetUserSettings(ctx context.Context, userID int64) (*entities.UserSettings, error) {
	log.Printf("Getting settings for user %d", userID)

	dbSettings, err := ss.db.GetUserSettings(ctx, userID)
	if err != nil {
		// If settings don't exist, create defaults
		log.Printf("Settings not found for user %d, creating defaults", userID)
		return ss.CreateDefaultSettings(ctx, userID)
	}

	settings := ss.dbSettingsRowToEntity(&dbSettings)
	log.Printf("Retrieved settings for user %d", userID)
	return settings, nil
}

// CreateDefaultSettings creates default settings for a user
func (ss *SettingsServiceImpl) CreateDefaultSettings(ctx context.Context, userID int64) (*entities.UserSettings, error) {
	log.Printf("Creating default settings for user %d", userID)

	// Create default settings entity
	settings := entities.NewUserSettings(userID)

	// Validate settings
	if err := settings.Validate(); err != nil {
		return nil, fmt.Errorf("invalid default settings: %w", err)
	}

	// Create in database
	err := ss.db.CreateUserSettings(ctx, sqlc.CreateUserSettingsParams{
		UserID:           userID,
		BuySlippage:      floatToNumeric(settings.BuySlippage),
		SellSlippage:     floatToNumeric(settings.SellSlippage),
		PriorityFee:      settings.PriorityFee,
		JitoTip:          settings.JitoTip,
		DefaultSolAmount: floatToNumeric(settings.DefaultSOLAmount),
		SellPreset25:     floatPtrToNumeric(settings.SellPreset25),
		SellPreset50:     floatPtrToNumeric(settings.SellPreset50),
		SellPreset75:     floatPtrToNumeric(settings.SellPreset75),
		SellPreset100:    floatPtrToNumeric(settings.SellPreset100),
		BuyPreset1:       floatPtrToNumeric(settings.BuyPreset1),
		BuyPreset2:       floatPtrToNumeric(settings.BuyPreset2),
		BuyPreset3:       floatPtrToNumeric(settings.BuyPreset3),
		BuyPreset4:       floatPtrToNumeric(settings.BuyPreset4),
		BuyPreset5:       floatPtrToNumeric(settings.BuyPreset5),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create settings in database: %w", err)
	}

	// Return the settings we just created
	log.Printf("Default settings created for user %d", userID)
	return settings, nil
}

// Helper functions for type conversion
func floatPtrToNumeric(f *float64) pgtype.Numeric {
	if f == nil {
		return pgtype.Numeric{Valid: false}
	}
	var n pgtype.Numeric
	n.Scan(fmt.Sprintf("%.6f", *f))
	return n
}

func numericToFloatPtr(n pgtype.Numeric) *float64 {
	if !n.Valid {
		return nil
	}
	f, _ := n.Float64Value()
	result := f.Float64
	return &result
}

// Helper functions floatToNumeric and numericToFloat are defined in portfolio_service.go

// UpdateSettings updates all user settings
func (ss *SettingsServiceImpl) UpdateSettings(ctx context.Context, userID int64, settings *entities.UserSettings) error {
	log.Printf("Updating settings for user %d", userID)

	// Ensure the settings belong to the correct user
	if settings.UserID != userID {
		return fmt.Errorf("settings user ID %d does not match requested user ID %d", settings.UserID, userID)
	}

	// Validate settings
	if err := settings.Validate(); err != nil {
		return fmt.Errorf("invalid settings: %w", err)
	}

	// Update in database using upsert
	err := ss.db.UpsertUserSettings(ctx, sqlc.UpsertUserSettingsParams{
		UserID:           userID,
		BuySlippage:      floatToNumeric(settings.BuySlippage),
		SellSlippage:     floatToNumeric(settings.SellSlippage),
		PriorityFee:      settings.PriorityFee,
		JitoTip:          settings.JitoTip,
		DefaultSolAmount: floatToNumeric(settings.DefaultSOLAmount),
		SellPreset25:     floatPtrToNumeric(settings.SellPreset25),
		SellPreset50:     floatPtrToNumeric(settings.SellPreset50),
		SellPreset75:     floatPtrToNumeric(settings.SellPreset75),
		SellPreset100:    floatPtrToNumeric(settings.SellPreset100),
		BuyPreset1:       floatPtrToNumeric(settings.BuyPreset1),
		BuyPreset2:       floatPtrToNumeric(settings.BuyPreset2),
		BuyPreset3:       floatPtrToNumeric(settings.BuyPreset3),
		BuyPreset4:       floatPtrToNumeric(settings.BuyPreset4),
		BuyPreset5:       floatPtrToNumeric(settings.BuyPreset5),
	})
	if err != nil {
		return fmt.Errorf("failed to update settings: %w", err)
	}

	log.Printf("Settings updated for user %d", userID)
	return nil
}

// UpdateSlippage updates buy and sell slippage settings
func (ss *SettingsServiceImpl) UpdateSlippage(ctx context.Context, userID int64, buySlippage, sellSlippage float64) error {
	log.Printf("Updating slippage for user %d: buy=%.2f%%, sell=%.2f%%", userID, buySlippage, sellSlippage)

	// Get current settings
	settings, err := ss.GetUserSettings(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to get current settings: %w", err)
	}

	// Update slippage
	settings.UpdateSlippage(buySlippage, sellSlippage)

	// Save updated settings
	err = ss.UpdateSettings(ctx, userID, settings)
	if err != nil {
		return fmt.Errorf("failed to save updated slippage: %w", err)
	}

	log.Printf("Slippage updated for user %d", userID)
	return nil
}

// UpdateFees updates priority fee and jito tip settings
func (ss *SettingsServiceImpl) UpdateFees(ctx context.Context, userID int64, priorityFee, jitoTip int64) error {
	log.Printf("Updating fees for user %d: priority=%d lamports, jito=%d lamports", userID, priorityFee, jitoTip)

	// Get current settings
	settings, err := ss.GetUserSettings(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to get current settings: %w", err)
	}

	log.Printf("🔍 DEBUG: Current settings BuySlippage=%.2f, SellSlippage=%.2f", settings.BuySlippage, settings.SellSlippage)

	// Update fees
	settings.UpdateFees(priorityFee, jitoTip)

	log.Printf("🔍 DEBUG: After UpdateFees BuySlippage=%.2f, SellSlippage=%.2f", settings.BuySlippage, settings.SellSlippage)

	// Save updated settings
	err = ss.UpdateSettings(ctx, userID, settings)
	if err != nil {
		return fmt.Errorf("failed to save updated fees: %w", err)
	}

	log.Printf("Fees updated for user %d", userID)
	return nil
}

// UpdateDefaultSOLAmount updates the default SOL amount for purchases
func (ss *SettingsServiceImpl) UpdateDefaultSOLAmount(ctx context.Context, userID int64, amount float64) error {
	log.Printf("Updating default SOL amount for user %d: %.6f SOL", userID, amount)

	// Get current settings
	settings, err := ss.GetUserSettings(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to get current settings: %w", err)
	}

	// Update default SOL amount
	settings.UpdateDefaultSOLAmount(amount)

	// Save updated settings
	err = ss.UpdateSettings(ctx, userID, settings)
	if err != nil {
		return fmt.Errorf("failed to save updated default SOL amount: %w", err)
	}

	log.Printf("Default SOL amount updated for user %d", userID)
	return nil
}

// UpdateSellPresets updates all sell preset percentages
func (ss *SettingsServiceImpl) UpdateSellPresets(ctx context.Context, userID int64, preset25, preset50, preset75, preset100 *float64) error {
	log.Printf("Updating sell presets for user %d", userID)

	// Get current settings
	settings, err := ss.GetUserSettings(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to get current settings: %w", err)
	}

	// Update sell presets
	settings.UpdateSellPresets(preset25, preset50, preset75, preset100)

	// Save updated settings
	err = ss.UpdateSettings(ctx, userID, settings)
	if err != nil {
		return fmt.Errorf("failed to save updated sell presets: %w", err)
	}

	log.Printf("Sell presets updated for user %d", userID)
	return nil
}

// UpdateBuyPresets updates all buy preset percentages
func (ss *SettingsServiceImpl) UpdateBuyPresets(ctx context.Context, userID int64, preset1, preset2, preset3, preset4, preset5 *float64) error {
	log.Printf("Updating buy presets for user %d", userID)

	// Get current settings
	settings, err := ss.GetUserSettings(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to get current settings: %w", err)
	}

	// Update buy presets
	settings.UpdateBuyPresets(preset1, preset2, preset3, preset4, preset5)

	// Save updated settings
	err = ss.UpdateSettings(ctx, userID, settings)
	if err != nil {
		return fmt.Errorf("failed to save updated buy presets: %w", err)
	}

	log.Printf("Buy presets updated for user %d", userID)
	return nil
}

// UpdateSingleBuyPreset updates a single buy preset percentage
func (ss *SettingsServiceImpl) UpdateSingleBuyPreset(ctx context.Context, userID int64, presetIndex int, amount *float64) error {
	log.Printf("Updating single buy preset for user %d: index=%d, amount=%.6f SOL", userID, presetIndex, *amount)

	// Validate preset index
	if presetIndex < 1 || presetIndex > 5 {
		return fmt.Errorf("invalid preset index: %d (must be 1-5)", presetIndex)
	}

	// Convert amount to numeric format
	numericAmount := floatPtrToNumeric(amount)

	// Use optimized single-column update queries
	var err error
	switch presetIndex {
	case 1:
		err = ss.db.UpdateSingleBuyPreset1(ctx, sqlc.UpdateSingleBuyPreset1Params{
			UserID:     userID,
			BuyPreset1: numericAmount,
		})
	case 2:
		err = ss.db.UpdateSingleBuyPreset2(ctx, sqlc.UpdateSingleBuyPreset2Params{
			UserID:     userID,
			BuyPreset2: numericAmount,
		})
	case 3:
		err = ss.db.UpdateSingleBuyPreset3(ctx, sqlc.UpdateSingleBuyPreset3Params{
			UserID:     userID,
			BuyPreset3: numericAmount,
		})
	case 4:
		err = ss.db.UpdateSingleBuyPreset4(ctx, sqlc.UpdateSingleBuyPreset4Params{
			UserID:     userID,
			BuyPreset4: numericAmount,
		})
	case 5:
		err = ss.db.UpdateSingleBuyPreset5(ctx, sqlc.UpdateSingleBuyPreset5Params{
			UserID:     userID,
			BuyPreset5: numericAmount,
		})
	}

	if err != nil {
		return fmt.Errorf("failed to update buy preset %d: %w", presetIndex, err)
	}

	log.Printf("Single buy preset %d updated for user %d", presetIndex, userID)
	return nil
}

// UpdateSingleSellPreset updates a single sell preset percentage
func (ss *SettingsServiceImpl) UpdateSingleSellPreset(ctx context.Context, userID int64, presetIndex int, amount *float64) error {
	log.Printf("Updating single sell preset for user %d: index=%d, amount=%.6f SOL", userID, presetIndex, *amount)

	// Validate preset index
	if presetIndex < 1 || presetIndex > 4 {
		return fmt.Errorf("invalid preset index: %d (must be 1-4)", presetIndex)
	}

	// Convert amount to numeric format
	numericAmount := floatPtrToNumeric(amount)

	// Use optimized single-column update queries
	var err error
	switch presetIndex {
	case 1:
		err = ss.db.UpdateSingleSellPreset1(ctx, sqlc.UpdateSingleSellPreset1Params{
			UserID:       userID,
			SellPreset25: numericAmount,
		})
	case 2:
		err = ss.db.UpdateSingleSellPreset2(ctx, sqlc.UpdateSingleSellPreset2Params{
			UserID:       userID,
			SellPreset50: numericAmount,
		})
	case 3:
		err = ss.db.UpdateSingleSellPreset3(ctx, sqlc.UpdateSingleSellPreset3Params{
			UserID:       userID,
			SellPreset75: numericAmount,
		})
	case 4:
		err = ss.db.UpdateSingleSellPreset4(ctx, sqlc.UpdateSingleSellPreset4Params{
			UserID:        userID,
			SellPreset100: numericAmount,
		})
	}

	if err != nil {
		return fmt.Errorf("failed to update sell preset %d: %w", presetIndex, err)
	}

	log.Printf("Single sell preset %d updated for user %d", presetIndex, userID)
	return nil
}

// ResetToDefaults resets user settings to default values
func (ss *SettingsServiceImpl) ResetToDefaults(ctx context.Context, userID int64) error {
	log.Printf("Resetting settings to defaults for user %d", userID)

	// Get current settings to ensure they exist
	settings, err := ss.GetUserSettings(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to get current settings: %w", err)
	}

	// Reset to defaults
	settings.ResetToDefaults()

	// Save updated settings
	err = ss.UpdateSettings(ctx, userID, settings)
	if err != nil {
		return fmt.Errorf("failed to save reset settings: %w", err)
	}

	log.Printf("Settings reset to defaults for user %d", userID)
	return nil
}

// DeleteSettings deletes user settings (will recreate defaults on next access)
func (ss *SettingsServiceImpl) DeleteSettings(ctx context.Context, userID int64) error {
	log.Printf("Deleting settings for user %d", userID)

	err := ss.db.DeleteUserSettings(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to delete settings: %w", err)
	}

	log.Printf("Settings deleted for user %d", userID)
	return nil
}

// dbSettingsRowToEntity converts database settings row to domain entity
func (ss *SettingsServiceImpl) dbSettingsRowToEntity(dbSettings *sqlc.GetUserSettingsRow) *entities.UserSettings {
	settings := &entities.UserSettings{
		UserID:           dbSettings.UserID,
		BuySlippage:      numericToFloat(dbSettings.BuySlippage),
		SellSlippage:     numericToFloat(dbSettings.SellSlippage),
		PriorityFee:      dbSettings.PriorityFee,
		JitoTip:          dbSettings.JitoTip,
		DefaultSOLAmount: numericToFloat(dbSettings.DefaultSolAmount),
		SellPreset25:     numericToFloatPtr(dbSettings.SellPreset25),
		SellPreset50:     numericToFloatPtr(dbSettings.SellPreset50),
		SellPreset75:     numericToFloatPtr(dbSettings.SellPreset75),
		SellPreset100:    numericToFloatPtr(dbSettings.SellPreset100),
		BuyPreset1:       numericToFloatPtr(dbSettings.BuyPreset1),
		BuyPreset2:       numericToFloatPtr(dbSettings.BuyPreset2),
		BuyPreset3:       numericToFloatPtr(dbSettings.BuyPreset3),
		BuyPreset4:       numericToFloatPtr(dbSettings.BuyPreset4),
		BuyPreset5:       numericToFloatPtr(dbSettings.BuyPreset5),
		CreatedAt:        dbSettings.CreatedAt.Time,
		UpdatedAt:        dbSettings.UpdatedAt.Time,
	}

	return settings
}
