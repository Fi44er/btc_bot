package repository

import (
	"context"
	"fmt"

	"github.com/Fi44er/btc_bot/internal/models"
	"gorm.io/gorm"
)

func (r *Repository) CreateWallet(ctx context.Context, wallet *models.SystemWallet, tx *gorm.DB) error {
	db := tx
	if tx == nil {
		db = r.db
	}

	return db.WithContext(ctx).Create(wallet).Error
}

func (r *Repository) GetWalletByID(ctx context.Context, id int64) (*models.SystemWallet, error) {
	var wallet models.SystemWallet
	err := r.db.WithContext(ctx).First(&wallet, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &wallet, nil
}

func (r *Repository) GetAllUsersWithAddresses(ctx context.Context) ([]*models.User, error) {
	var users []*models.User

	err := r.db.WithContext(ctx).
		Preload("SystemWallet").
		Joins("JOIN system_wallets ON users.system_wallet_id = system_wallets.id").
		Where("system_wallets.address IS NOT NULL AND system_wallets.address != ''").
		Find(&users).Error

	if err != nil {
		r.logger.Errorf("failed to get all users with addresses: %v", err)
		return nil, fmt.Errorf("failed to get all users with addresses: %w", err)
	}

	return users, nil
}
