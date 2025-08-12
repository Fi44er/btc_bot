package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/Fi44er/btc_bot/internal/models"
	"github.com/Fi44er/btc_bot/utils"
	"gorm.io/gorm"
)

type Repository struct {
	db     *gorm.DB
	logger *utils.Logger
}

func NewRepository(db *gorm.DB, logger *utils.Logger) *Repository {
	return &Repository{db: db, logger: logger}
}

func (r *Repository) GetUserByAddress(ctx context.Context, address string) (*models.User, error) {
	var user models.User

	err := r.db.WithContext(ctx).
		Preload("SystemWallet").
		Joins("JOIN system_wallets ON users.system_wallet_id = system_wallets.id").
		Where("system_wallets.address = ?", address).
		First(&user).
		Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user by address %s: %w", address, err)
	}

	return &user, nil
}

func (r *Repository) GetAllUsersWithAddresses(ctx context.Context) ([]*models.User, error) {
	var users []*models.User
	err := r.db.WithContext(ctx).Where("deposit_address IS NOT NULL").Find(&users).Error
	if err != nil {
		return nil, err
	}
	return users, nil
}
