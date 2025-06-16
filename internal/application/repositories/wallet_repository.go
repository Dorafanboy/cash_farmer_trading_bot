package repositories

import (
	"context"

	"cash-farmer/internal/domain/entities"
)

// WalletRepository defines interface for wallet database operations
type WalletRepository interface {
	Save(ctx context.Context, wallet *entities.Wallet) error
	FindByID(ctx context.Context, id int64) (*entities.Wallet, error)
	FindByUserID(ctx context.Context, userID int64) ([]*entities.Wallet, error)
	FindDefaultByUserID(ctx context.Context, userID int64) (*entities.Wallet, error)
	FindByName(ctx context.Context, userID int64, name string) (*entities.Wallet, error)
	Update(ctx context.Context, wallet *entities.Wallet) error
	Delete(ctx context.Context, id int64, userID int64) error
	SetAsDefault(ctx context.Context, userID int64, walletID int64) error
	Count(ctx context.Context, userID int64) (int, error)
}
