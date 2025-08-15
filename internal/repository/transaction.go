package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/Fi44er/btc_bot/internal/models"
	"gorm.io/gorm"
)

func (r *Repository) GetTransaction(ctx context.Context, txID string) (*models.Transaction, error) {
	var tx models.Transaction
	err := r.db.WithContext(ctx).
		Where("tx_id = ?", txID).
		First(&tx).
		Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get transaction: %w", err)
	}

	return &tx, nil
}

func (r *Repository) CreateOrUpdateTransaction(ctx context.Context, tx *models.Transaction) error {
	var existing models.Transaction
	err := r.db.WithContext(ctx).Where("tx_id = ?", tx.TxID).First(&existing).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return r.db.WithContext(ctx).Create(tx).Error
		}
		return err
	}

	return r.db.WithContext(ctx).Model(&existing).Updates(tx).Error
}
