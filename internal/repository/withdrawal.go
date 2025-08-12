package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/Fi44er/btc_bot/internal/models"
	"gorm.io/gorm"
)

func (r *Repository) GetAllWithdrawals(ctx context.Context) ([]models.Withdrawal, error) {
	var withdrawals []models.Withdrawal
	err := r.db.WithContext(ctx).Find(&withdrawals).Error
	if err != nil {
		return nil, err
	}
	return withdrawals, nil
}

func (r *Repository) SumPendingWithdrawals(ctx context.Context, userID int64) (float64, error) {
	var sum float64
	err := r.db.WithContext(ctx).
		Model(&models.Withdrawal{}).
		Where("user_id = ? AND status = ?", userID, "pending").
		Select("COALESCE(SUM(amount),0)").Scan(&sum).Error
	return sum, err
}

// Получает существующую заявку пользователя со статусом pending
func (r *Repository) GetPendingWithdrawalByUser(ctx context.Context, userID int64) (*models.Withdrawal, error) {
	var withdrawal models.Withdrawal
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND status = ?", userID, "pending").
		First(&withdrawal).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &withdrawal, nil
}

// Обновить заявку
func (r *Repository) UpdateWithdrawal(ctx context.Context, withdrawal *models.Withdrawal) error {
	return r.db.WithContext(ctx).Save(withdrawal).Error
}

func (r *Repository) CreateWithdrawal(ctx context.Context, withdrawal *models.Withdrawal) error {
	return r.db.WithContext(ctx).Create(withdrawal).Error
}

func (r *Repository) GetPendingWithdrawals(ctx context.Context) ([]*models.Withdrawal, error) {
	var withdrawals []*models.Withdrawal
	err := r.db.WithContext(ctx).
		Where("status = ?", "pending").
		Order("created_at ASC").
		Find(&withdrawals).
		Error

	if err != nil {
		return nil, fmt.Errorf("failed to get pending withdrawals: %w", err)
	}
	return withdrawals, nil
}

func (r *Repository) GetWithdrawalByID(ctx context.Context, id int64) (*models.Withdrawal, error) {
	var withdrawal models.Withdrawal
	err := r.db.WithContext(ctx).
		Where("id = ?", id).
		First(&withdrawal).
		Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get withdrawal by id %d: %w", id, err)
	}
	return &withdrawal, nil
}

func (r *Repository) UpdateWithdrawalStatus(ctx context.Context, id int64, status string) error {
	err := r.db.WithContext(ctx).
		Model(&models.Withdrawal{}).
		Where("id = ?", id).
		Update("status", status).
		Error

	if err != nil {
		return fmt.Errorf("failed to update withdrawal status: %w", err)
	}
	return nil
}

// GetPendingWithdrawalByUserID ищет один активный (статус 'pending') запрос на вывод для пользователя.
func (r *Repository) GetPendingWithdrawalByUserID(ctx context.Context, userID int64) (*models.Withdrawal, error) {
	var withdrawal models.Withdrawal

	// Ищем запись, где user_id совпадает и статус 'pending'
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND status = ?", userID, "pending").
		First(&withdrawal).
		Error

	if err != nil {
		// Если запись не найдена - это не ошибка, а ожидаемое поведение.
		// Сервис будет знать, что нужно создать новую запись, а не обновлять.
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		// Любая другая ошибка является системной.
		r.logger.Errorf("ошибка получения ожидающего вывода из БД для пользователя %d: %v", userID, err)
		return nil, fmt.Errorf("ошибка БД при поиске вывода: %w", err)
	}

	return &withdrawal, nil
}

// DeleteWithdrawal физически удаляет запись о выводе из таблицы по ее ID.
func (r *Repository) DeleteWithdrawal(ctx context.Context, id int64) error {
	// GORM позволяет удалить запись, передав модель и ее ID
	tx := r.db.WithContext(ctx).Delete(&models.Withdrawal{}, id)

	if tx.Error != nil {
		r.logger.Errorf("ошибка удаления вывода #%d из БД: %v", id, tx.Error)
		return fmt.Errorf("ошибка БД при удалении вывода: %w", tx.Error)
	}

	// Важная проверка: если ни одна строка не была затронута, значит,
	// запись с таким ID не существовала. Это стоит считать ошибкой.
	if tx.RowsAffected == 0 {
		return fmt.Errorf("запись о выводе с ID %d не найдена для удаления", id)
	}

	r.logger.Infof("Запись о выводе #%d успешно удалена из БД", id)
	return nil
}

// UpdateUserBalance обновляет поле 'balance' у пользователя по его telegram_id.
func (r *Repository) UpdateUserBalance(ctx context.Context, userID int64, newBalance float64) error {
	// Используем Model(...).Where(...).Update(...) для целевого обновления одного поля.
	// Это эффективнее, чем загружать и сохранять весь объект пользователя.
	tx := r.db.WithContext(ctx).
		Model(&models.User{}).
		Where("telegram_id = ?", userID).
		Update("balance", newBalance)

	if tx.Error != nil {
		r.logger.Errorf("ошибка обновления баланса для пользователя %d в БД: %v", userID, tx.Error)
		return fmt.Errorf("ошибка БД при обновлении баланса: %w", tx.Error)
	}

	// Также проверяем, что пользователь действительно был найден и обновлен.
	if tx.RowsAffected == 0 {
		return fmt.Errorf("пользователь с telegram_id %d не найден для обновления баланса", userID)
	}

	r.logger.Infof("Баланс пользователя %d обновлен на %.8f", userID, newBalance)
	return nil
}
