package service

import (
	"context"

	"github.com/Fi44er/btc_bot/internal/models"
)

func (s Service) GetWalletByID(ctx context.Context, id int64) (*models.SystemWallet, error) {
	return s.repo.GetWalletByID(ctx, id)
}

func (s *Service) GetUsersWithWallets(ctx context.Context) ([]*models.User, error) {
	s.logger.Info("Service: fetching all users with wallets")
	users, err := s.repo.GetAllUsersWithAddresses(ctx)
	if err != nil {
		s.logger.Errorf("Service: failed to fetch users with wallets: %v", err)
		return nil, err
	}
	return users, nil
}
