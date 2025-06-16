package test_files

import (
	"context"
	"testing"

	"cash-farmer/internal/application/services"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TDD TEST 2: Database presets должны использоваться вместо hardcoded
func TestBuyPresets_UseDatabaseValues(t *testing.T) {
	// ARRANGE
	container, err := services.NewSimpleServiceContainer()
	require.NoError(t, err)
	defer container.Close()

	// Установить custom presets для пользователя
	settingsService := container.GetSettingsService()
	userID := int64(12345)

	// Установим custom buy presets: 0.005, 0.01, 0.025, 0.05, 0.1
	err = settingsService.UpdateBuyPresets(context.Background(), userID,
		func() *float64 { v := 0.005; return &v }(),
		func() *float64 { v := 0.01; return &v }(),
		func() *float64 { v := 0.025; return &v }(),
		func() *float64 { v := 0.05; return &v }(),
		func() *float64 { v := 0.1; return &v }())
	require.NoError(t, err)

	// ACT & ASSERT
	// Получаем настройки пользователя
	settings, err := settingsService.GetUserSettings(context.Background(), userID)
	require.NoError(t, err)

	// Проверяем что preset 1 теперь 0.005, а не hardcoded 0.001
	assert.Equal(t, 0.005, *settings.BuyPreset1, "Должен использоваться database preset, не hardcoded")
}
