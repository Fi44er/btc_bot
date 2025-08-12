package service

import (
	"context"

	"github.com/Fi44er/btc_bot/internal/models"
)

func (s Service) GetWalletByID(ctx context.Context, id int64) (*models.SystemWallet, error) {
	return s.repo.GetWalletByID(ctx, id)
}
