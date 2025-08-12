package repository

import (
	"context"
	"errors"

	"github.com/Fi44er/btc_bot/internal/models"
	"gorm.io/gorm"
)

func (r *Repository) GetUser(ctx context.Context, telegramID int64) (*models.User, error) {
	var user models.User
	err := r.db.WithContext(ctx).Preload("SystemWallet").Preload("Transactions").First(&user, "telegram_id = ?", telegramID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

func (r *Repository) CreateUser(ctx context.Context, user *models.User) error {
	return r.db.WithContext(ctx).Create(user).Error
}

func (r *Repository) UpdateUser(ctx context.Context, user *models.User, tx *gorm.DB) error {
	db := tx
	if tx == nil {
		db = r.db
	}

	return db.WithContext(ctx).Save(user).Error
}
