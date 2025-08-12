package repository

import (
	"context"

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
