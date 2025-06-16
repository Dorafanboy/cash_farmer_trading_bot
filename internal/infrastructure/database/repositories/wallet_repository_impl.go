package repositories

import (
	"context"
	"errors"
	"time"

	"cash-farmer/internal/application/repositories"
	"cash-farmer/internal/domain/entities"
	"cash-farmer/internal/domain/valueobjects"
	"cash-farmer/internal/infrastructure/database/sqlc"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// WalletRepositoryImpl implements WalletRepository using sqlc
type WalletRepositoryImpl struct {
	queries *sqlc.Queries
}

// NewWalletRepository creates new WalletRepository implementation
func NewWalletRepository(queries *sqlc.Queries) repositories.WalletRepository {
	return &WalletRepositoryImpl{
		queries: queries,
	}
}

// Save saves a new wallet
func (r *WalletRepositoryImpl) Save(ctx context.Context, wallet *entities.Wallet) error {
	params := sqlc.SaveWalletParams{
		UserID:     wallet.UserID,
		Name:       wallet.Name,
		PublicKey:  wallet.GetPublicKeyString(),
		PrivateKey: wallet.PrivateKey,
		Mnemonic:   pgtype.Text{String: wallet.Mnemonic, Valid: wallet.Mnemonic != ""},
		CreatedAt:  pgtype.Timestamptz{Time: wallet.CreatedAt, Valid: true},
		IsDefault:  wallet.IsDefault,
	}

	return r.queries.SaveWallet(ctx, params)
}

// FindByID finds wallet by its ID
func (r *WalletRepositoryImpl) FindByID(ctx context.Context, id int64) (*entities.Wallet, error) {
	sqlcWallet, err := r.queries.FindWalletByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	return r.convertFromFindWalletByIDRow(sqlcWallet)
}

// FindByUserID finds all user wallets
func (r *WalletRepositoryImpl) FindByUserID(ctx context.Context, userID int64) ([]*entities.Wallet, error) {
	sqlcWallets, err := r.queries.FindWalletsByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	wallets := make([]*entities.Wallet, 0, len(sqlcWallets))
	for _, sqlcWallet := range sqlcWallets {
		wallet, err := r.convertFromFindWalletsByUserIDRow(sqlcWallet)
		if err != nil {
			return nil, err
		}
		wallets = append(wallets, wallet)
	}

	return wallets, nil
}

// FindDefaultByUserID finds user default wallet
func (r *WalletRepositoryImpl) FindDefaultByUserID(ctx context.Context, userID int64) (*entities.Wallet, error) {
	sqlcWallet, err := r.queries.FindDefaultWalletByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	return r.convertFromFindDefaultWalletByUserIDRow(sqlcWallet)
}

// FindByName finds user wallet by name
func (r *WalletRepositoryImpl) FindByName(ctx context.Context, userID int64, name string) (*entities.Wallet, error) {
	params := sqlc.FindWalletByNameParams{
		UserID: userID,
		Name:   name,
	}

	sqlcWallet, err := r.queries.FindWalletByName(ctx, params)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	return r.convertFromFindWalletByNameRow(sqlcWallet)
}

// Update updates existing wallet
func (r *WalletRepositoryImpl) Update(ctx context.Context, wallet *entities.Wallet) error {
	params := sqlc.UpdateWalletParams{
		ID:         wallet.ID,
		Name:       wallet.Name,
		PublicKey:  wallet.GetPublicKeyString(),
		PrivateKey: wallet.PrivateKey,
		Mnemonic:   pgtype.Text{String: wallet.Mnemonic, Valid: wallet.Mnemonic != ""},
		IsDefault:  wallet.IsDefault,
	}

	return r.queries.UpdateWallet(ctx, params)
}

// Delete removes wallet
func (r *WalletRepositoryImpl) Delete(ctx context.Context, id int64, userID int64) error {
	params := sqlc.DeleteWalletParams{
		ID:     id,
		UserID: userID,
	}

	return r.queries.DeleteWallet(ctx, params)
}

// SetAsDefault sets wallet as default and removes flag from others
func (r *WalletRepositoryImpl) SetAsDefault(ctx context.Context, userID int64, walletID int64) error {
	// First, clear all default flags for the user
	if err := r.queries.ClearDefaultWallets(ctx, userID); err != nil {
		return err
	}

	// Then, set the specific wallet as default
	params := sqlc.SetWalletDefaultParams{
		ID:     walletID,
		UserID: userID,
	}

	return r.queries.SetWalletDefault(ctx, params)
}

// Count returns user wallet count
func (r *WalletRepositoryImpl) Count(ctx context.Context, userID int64) (int, error) {
	count, err := r.queries.CountUserWallets(ctx, userID)
	return int(count), err
}

// convertFromFindWalletByIDRow converts FindWalletByIDRow to entities.Wallet
func (r *WalletRepositoryImpl) convertFromFindWalletByIDRow(row sqlc.FindWalletByIDRow) (*entities.Wallet, error) {
	publicKey, err := valueobjects.NewSolanaAddress(row.PublicKey)
	if err != nil {
		return nil, err
	}

	var mnemonic string
	if row.Mnemonic.Valid {
		mnemonic = row.Mnemonic.String
	}

	var createdAt time.Time
	if row.CreatedAt.Valid {
		createdAt = row.CreatedAt.Time
	}

	return &entities.Wallet{
		ID:         row.ID,
		UserID:     row.UserID,
		Name:       row.Name,
		PublicKey:  publicKey,
		PrivateKey: row.PrivateKey,
		Mnemonic:   mnemonic,
		CreatedAt:  createdAt,
		IsDefault:  row.IsDefault,
	}, nil
}

// convertFromFindWalletsByUserIDRow converts FindWalletsByUserIDRow to entities.Wallet
func (r *WalletRepositoryImpl) convertFromFindWalletsByUserIDRow(row sqlc.FindWalletsByUserIDRow) (*entities.Wallet, error) {
	publicKey, err := valueobjects.NewSolanaAddress(row.PublicKey)
	if err != nil {
		return nil, err
	}

	var mnemonic string
	if row.Mnemonic.Valid {
		mnemonic = row.Mnemonic.String
	}

	var createdAt time.Time
	if row.CreatedAt.Valid {
		createdAt = row.CreatedAt.Time
	}

	return &entities.Wallet{
		ID:         row.ID,
		UserID:     row.UserID,
		Name:       row.Name,
		PublicKey:  publicKey,
		PrivateKey: row.PrivateKey,
		Mnemonic:   mnemonic,
		CreatedAt:  createdAt,
		IsDefault:  row.IsDefault,
	}, nil
}

// convertFromFindDefaultWalletByUserIDRow converts FindDefaultWalletByUserIDRow to entities.Wallet
func (r *WalletRepositoryImpl) convertFromFindDefaultWalletByUserIDRow(row sqlc.FindDefaultWalletByUserIDRow) (*entities.Wallet, error) {
	publicKey, err := valueobjects.NewSolanaAddress(row.PublicKey)
	if err != nil {
		return nil, err
	}

	var mnemonic string
	if row.Mnemonic.Valid {
		mnemonic = row.Mnemonic.String
	}

	var createdAt time.Time
	if row.CreatedAt.Valid {
		createdAt = row.CreatedAt.Time
	}

	return &entities.Wallet{
		ID:         row.ID,
		UserID:     row.UserID,
		Name:       row.Name,
		PublicKey:  publicKey,
		PrivateKey: row.PrivateKey,
		Mnemonic:   mnemonic,
		CreatedAt:  createdAt,
		IsDefault:  row.IsDefault,
	}, nil
}

// convertFromFindWalletByNameRow converts FindWalletByNameRow to entities.Wallet
func (r *WalletRepositoryImpl) convertFromFindWalletByNameRow(row sqlc.FindWalletByNameRow) (*entities.Wallet, error) {
	publicKey, err := valueobjects.NewSolanaAddress(row.PublicKey)
	if err != nil {
		return nil, err
	}

	var mnemonic string
	if row.Mnemonic.Valid {
		mnemonic = row.Mnemonic.String
	}

	var createdAt time.Time
	if row.CreatedAt.Valid {
		createdAt = row.CreatedAt.Time
	}

	return &entities.Wallet{
		ID:         row.ID,
		UserID:     row.UserID,
		Name:       row.Name,
		PublicKey:  publicKey,
		PrivateKey: row.PrivateKey,
		Mnemonic:   mnemonic,
		CreatedAt:  createdAt,
		IsDefault:  row.IsDefault,
	}, nil
}

// convertFromSqlcWallet converts sqlc.Wallet to entities.Wallet
func (r *WalletRepositoryImpl) convertFromSqlcWallet(sqlcWallet sqlc.Wallet) (*entities.Wallet, error) {
	publicKey, err := valueobjects.NewSolanaAddress(sqlcWallet.PublicKey)
	if err != nil {
		return nil, err
	}

	var mnemonic string
	if sqlcWallet.Mnemonic.Valid {
		mnemonic = sqlcWallet.Mnemonic.String
	}

	var createdAt time.Time
	if sqlcWallet.CreatedAt.Valid {
		createdAt = sqlcWallet.CreatedAt.Time
	}

	wallet := &entities.Wallet{
		ID:         sqlcWallet.ID,
		UserID:     sqlcWallet.UserID,
		Name:       sqlcWallet.Name,
		PublicKey:  publicKey,
		PrivateKey: sqlcWallet.PrivateKey,
		Mnemonic:   mnemonic,
		CreatedAt:  createdAt,
		IsDefault:  sqlcWallet.IsDefault,
	}

	return wallet, nil
}
