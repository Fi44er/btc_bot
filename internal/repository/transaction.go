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
		return nil, nil // Возвращаем nil вместо ошибки, если транзакция не найдена
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
			// Записи нет — создаём новую
			return r.db.WithContext(ctx).Create(tx).Error
		}
		// Ошибка при запросе
		return err
	}

	// Запись есть — обновляем
	return r.db.WithContext(ctx).Model(&existing).Updates(tx).Error
}
