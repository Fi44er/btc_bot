package repository

import (
	"context"
	"errors"

	"github.com/Fi44er/btc_bot/internal/models"
	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) GetUser(ctx context.Context, telegramID int64) (*models.User, error) {
	var user models.User
	err := r.db.WithContext(ctx).First(&user, "telegram_id = ?", telegramID).Error
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

func (r *Repository) UpdateUser(ctx context.Context, user *models.User) error {
	return r.db.WithContext(ctx).Save(user).Error
}

func (r *Repository) GetUserByAddress(ctx context.Context, address string) (*models.User, error) {
	var user models.User
	err := r.db.WithContext(ctx).First(&user, "deposit_address = ?", address).Error
	if err != nil {
		return nil, err
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

func (r *Repository) IsTransactionConfirmed(ctx context.Context, txID string) (bool, error) {
	var tx models.Transaction
	err := r.db.WithContext(ctx).First(&tx, "tx_id = ?", txID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, err
	}
	return tx.Confirmed, nil
}

func (r *Repository) CreateOrUpdateTransaction(ctx context.Context, tx *models.Transaction) error {
	var transaction models.Transaction
	if err := r.db.WithContext(ctx).Where("tx_id = ?", tx.TxID).First(transaction).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return r.db.WithContext(ctx).Create(tx).Error
		}
		return err
	}

	if err := r.db.WithContext(ctx).Where("tx_id = ?", tx.TxID).Updates(tx).Error; err != nil {
		return err
	}
	return r.db.WithContext(ctx).Create(tx).Error
}
