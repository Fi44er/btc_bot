package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/Fi44er/btc_bot/internal/models"
)

func (s *Service) GetPendingWithdrawalByUserID(ctx context.Context, userID int64) (*models.Withdrawal, error) {
	return s.repo.GetPendingWithdrawalByUserID(ctx, userID)
}

func (s *Service) DeleteWithdrawal(ctx context.Context, id int64) error {
	return s.repo.DeleteWithdrawal(ctx, id)
}

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

func (s *Service) ProcessWithdrawal(ctx context.Context, withdrawalID int64) error {
	tx, err := s.repo.BeginTransaction(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err != nil {
			s.repo.Rollback(tx)
		}
	}()

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

	user, err := s.repo.GetUser(ctx, withdrawal.UserID)
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}

	if user == nil {
		return errors.New("user not found")
	}

	if user.Balance < withdrawal.Amount {
		return errors.New("insufficient funds")
	}

	newBalance := user.Balance - withdrawal.Amount
	err = s.repo.WithTransaction(tx).
		Model(&models.User{}).
		Where("telegram_id = ?", user.TelegramID).
		Update("balance", newBalance).
		Error
	if err != nil {
		return fmt.Errorf("failed to update user balance: %w", err)
	}

	err = s.repo.WithTransaction(tx).
		Model(&models.Withdrawal{}).
		Where("id = ?", withdrawal.ID).
		Update("status", "completed").
		Error
	if err != nil {
		return fmt.Errorf("failed to update withdrawal status: %w", err)
	}

	if err = s.repo.Commit(tx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func (s *Service) CreateOrUpdateWithdrawal(ctx context.Context, withdrawalDelta *models.Withdrawal) (*models.Withdrawal, bool, error) {
	user, err := s.repo.GetUser(ctx, withdrawalDelta.UserID)
	if err != nil {
		return nil, false, fmt.Errorf("не удалось получить данные пользователя: %w", err)
	}
	if user == nil {
		return nil, false, errors.New("пользователь не найден")
	}

	existingWithdrawal, err := s.repo.GetPendingWithdrawalByUserID(ctx, withdrawalDelta.UserID)
	if err != nil {
		return nil, false, fmt.Errorf("не удалось проверить существующие заявки: %w", err)
	}

	if existingWithdrawal != nil {
		totalAmount := existingWithdrawal.Amount + withdrawalDelta.Amount

		if totalAmount > user.Balance {
			return nil, false, fmt.Errorf("недостаточно средств. Итоговая сумма заявки (%.8f) превысит ваш баланс (%.8f)", totalAmount, user.Balance)
		}

		existingWithdrawal.Amount = totalAmount
		if err := s.repo.UpdateWithdrawal(ctx, existingWithdrawal); err != nil {
			return nil, false, fmt.Errorf("не удалось обновить заявку в базе данных: %w", err)
		}

		return existingWithdrawal, true, nil

	} else {
		if withdrawalDelta.Amount > user.Balance {
			return nil, false, fmt.Errorf("недостаточно средств. Запрашиваемая сумма (%.8f) превышает ваш баланс (%.8f)", withdrawalDelta.Amount, user.Balance)
		}

		withdrawalDelta.Status = "pending" // Устанавливаем статус
		if err := s.repo.CreateWithdrawal(ctx, withdrawalDelta); err != nil {
			return nil, false, fmt.Errorf("не удалось создать заявку в базе данных: %w", err)
		}

		return withdrawalDelta, false, nil
	}
}
