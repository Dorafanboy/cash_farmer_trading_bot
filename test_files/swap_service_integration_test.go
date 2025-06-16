package test_files

import (
	"cash-farmer/internal/application/services"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TDD TEST 1: SimpleServiceContainer должен предоставлять доступ к TokenSwapService
func TestSimpleServiceContainer_GetTokenSwapService(t *testing.T) {
	// ARRANGE
	container, err := services.NewSimpleServiceContainer()
	require.NoError(t, err)
	defer container.Close()

	// ACT - этот метод пока не существует (RED phase)
	swapService := container.GetTokenSwapService()

	// ASSERT
	assert.NotNil(t, swapService, "TokenSwapService должен быть доступен через SimpleServiceContainer")
}
