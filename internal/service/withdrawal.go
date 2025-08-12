package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/Fi44er/btc_bot/internal/models"
)

// GetPendingWithdrawalByUserID просто передает вызов в слой репозитория.
// Здесь может быть добавлена дополнительная бизнес-логика при необходимости.
func (s *Service) GetPendingWithdrawalByUserID(ctx context.Context, userID int64) (*models.Withdrawal, error) {
	return s.repo.GetPendingWithdrawalByUserID(ctx, userID)
}

// DeleteWithdrawal передает вызов на удаление в слой репозитория.
func (s *Service) DeleteWithdrawal(ctx context.Context, id int64) error {
	return s.repo.DeleteWithdrawal(ctx, id)
}

// UpdateUserBalance передает вызов на обновление баланса в слой репозитория.
func (s *Service) UpdateUserBalance(ctx context.Context, userID int64, newBalance float64) error {
	return s.repo.UpdateUserBalance(ctx, userID, newBalance)
}

func (s *Service) GetPendingWithdrawals(ctx context.Context) ([]*models.Withdrawal, error) {
	withdrawals, err := s.repo.GetPendingWithdrawals(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get pending withdrawals: %w", err)
	}
	return withdrawals, nil
}

func (s *Service) GetWithdrawalByID(ctx context.Context, id int64) (*models.Withdrawal, error) {
	withdrawal, err := s.repo.GetWithdrawalByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get withdrawal by id: %w", err)
	}
	return withdrawal, nil
}

func (s *Service) UpdateWithdrawalStatus(ctx context.Context, id int64, status string) error {
	// Проверяем допустимые статусы
	validStatuses := map[string]bool{
		"pending":   true,
		"completed": true,
		"canceled":  true,
	}

	if !validStatuses[status] {
		return errors.New("invalid withdrawal status")
	}

	err := s.repo.UpdateWithdrawalStatus(ctx, id, status)
	if err != nil {
		return fmt.Errorf("failed to update withdrawal status: %w", err)
	}
	return nil
}

// ProcessWithdrawal обрабатывает вывод средств (используется в обработчике админа)
func (s *Service) ProcessWithdrawal(ctx context.Context, withdrawalID int64) error {
	// Начинаем транзакцию
	tx, err := s.repo.BeginTransaction(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err != nil {
			s.repo.Rollback(tx)
		}
	}()

	// Получаем заявку на вывод
	withdrawal, err := s.repo.GetWithdrawalByID(ctx, withdrawalID)
	if err != nil {
		return fmt.Errorf("failed to get withdrawal: %w", err)
	}

	if withdrawal == nil {
		return errors.New("withdrawal not found")
	}

	if withdrawal.Status != "pending" {
		return errors.New("withdrawal already processed")
	}

	// Получаем пользователя
	user, err := s.repo.GetUser(ctx, withdrawal.UserID)
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}

	if user == nil {
		return errors.New("user not found")
	}

	// Проверяем баланс
	if user.Balance < withdrawal.Amount {
		return errors.New("insufficient funds")
	}

	// Обновляем баланс пользователя
	newBalance := user.Balance - withdrawal.Amount
	err = s.repo.WithTransaction(tx).
		Model(&models.User{}).
		Where("telegram_id = ?", user.TelegramID).
		Update("balance", newBalance).
		Error
	if err != nil {
		return fmt.Errorf("failed to update user balance: %w", err)
	}

	// Обновляем статус заявки
	err = s.repo.WithTransaction(tx).
		Model(&models.Withdrawal{}).
		Where("id = ?", withdrawal.ID).
		Update("status", "completed").
		Error
	if err != nil {
		return fmt.Errorf("failed to update withdrawal status: %w", err)
	}

	// Коммитим транзакцию
	if err = s.repo.Commit(tx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func (s *Service) CreateOrUpdateWithdrawal(ctx context.Context, withdrawalDelta *models.Withdrawal) (*models.Withdrawal, bool, error) {
	// 1. Получаем пользователя, чтобы проверить его актуальный баланс
	user, err := s.repo.GetUser(ctx, withdrawalDelta.UserID)
	if err != nil {
		return nil, false, fmt.Errorf("не удалось получить данные пользователя: %w", err)
	}
	if user == nil {
		return nil, false, errors.New("пользователь не найден")
	}

	// 2. Ищем, есть ли у пользователя УЖЕ существующая заявка в статусе 'pending'
	existingWithdrawal, err := s.repo.GetPendingWithdrawalByUserID(ctx, withdrawalDelta.UserID)
	if err != nil {
		return nil, false, fmt.Errorf("не удалось проверить существующие заявки: %w", err)
	}

	// 3. Логика ветвится в зависимости от того, нашли ли мы существующую заявку
	if existingWithdrawal != nil {
		// --- СЦЕНАРИЙ ОБНОВЛЕНИЯ ---

		// Вычисляем итоговую сумму, которая ПОЛУЧИТСЯ, если мы добавим новую.
		totalAmount := existingWithdrawal.Amount + withdrawalDelta.Amount

		// Проверяем итоговую сумму относительно общего баланса. Это ЕДИНСТВЕННАЯ проверка.
		if totalAmount > user.Balance {
			return nil, false, fmt.Errorf("недостаточно средств. Итоговая сумма заявки (%.8f) превысит ваш баланс (%.8f)", totalAmount, user.Balance)
		}

		// Если проверка прошла, обновляем сумму в объекте и сохраняем в БД.
		existingWithdrawal.Amount = totalAmount
		if err := s.repo.UpdateWithdrawal(ctx, existingWithdrawal); err != nil {
			return nil, false, fmt.Errorf("не удалось обновить заявку в базе данных: %w", err)
		}

		// Возвращаем обновленную заявку и флаг isUpdate = true
		return existingWithdrawal, true, nil

	} else {
		// --- СЦЕНАРИЙ СОЗДАНИЯ НОВОЙ ЗАЯВКИ ---

		// Проверяем, что запрашиваемая сумма не превышает баланс
		if withdrawalDelta.Amount > user.Balance {
			return nil, false, fmt.Errorf("недостаточно средств. Запрашиваемая сумма (%.8f) превышает ваш баланс (%.8f)", withdrawalDelta.Amount, user.Balance)
		}

		// Если все в порядке, создаем новую запись
		withdrawalDelta.Status = "pending" // Устанавливаем статус
		if err := s.repo.CreateWithdrawal(ctx, withdrawalDelta); err != nil {
			return nil, false, fmt.Errorf("не удалось создать заявку в базе данных: %w", err)
		}

		// Возвращаем созданную заявку и флаг isUpdate = false
		return withdrawalDelta, false, nil
	}
}
