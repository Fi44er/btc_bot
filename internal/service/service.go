package service

import (
	"context"
	"errors"

	"github.com/Fi44er/btc_bot/internal/models"
	"github.com/btcsuite/btcd/btcutil/hdkeychain"
	"github.com/btcsuite/btcd/chaincfg"
)

type UserService struct {
	repo        Repository
	masterKey   *hdkeychain.ExtendedKey
	netParams   *chaincfg.Params
	addressIdx  uint32
	adminChatID int64
}

type Repository interface {
	GetUser(ctx context.Context, telegramID int64) (*models.User, error)
	CreateUser(ctx context.Context, user *models.User) error
	UpdateUser(ctx context.Context, user *models.User) error
	GetUserByAddress(ctx context.Context, address string) (*models.User, error)
	GetAllUsersWithAddresses(ctx context.Context) ([]*models.User, error)
	IsTransactionConfirmed(ctx context.Context, txID string) (bool, error)
	CreateOrUpdateTransaction(ctx context.Context, tx *models.Transaction) error
}

func NewUserService(repo Repository, masterKeySeed string, adminChatID int64) (*UserService, error) {
	masterKey, err := hdkeychain.NewKeyFromString(masterKeySeed)
	if err != nil {
		return nil, err
	}

	return &UserService{
		repo:        repo,
		masterKey:   masterKey,
		netParams:   &chaincfg.TestNet3Params,
		adminChatID: adminChatID,
	}, nil
}

func (s *UserService) GetAdminChatID() int64 {
	return s.adminChatID
}

func (s *UserService) CreateUser(ctx context.Context, userID int64) error {
	user := &models.User{
		TelegramID: userID,
	}
	return s.repo.CreateUser(ctx, user)
}

func (s *UserService) UpdateAddress(ctx context.Context, telegramID int64) (*models.User, error) {
	user, err := s.repo.GetUser(ctx, telegramID)
	if err != nil {
		return nil, err
	}

	if user == nil {
		return nil, errors.New("user not found")
	}

	address, err := s.generateNewAddress()
	if err != nil {
		return nil, err
	}

	user.DepositAddress = address
	if err := s.repo.UpdateUser(ctx, user); err != nil {
		return nil, err
	}
	return user, nil
}

func (s *UserService) UpdateCardNumber(ctx context.Context, telegramID int64, cardNumber string) error {
	user, err := s.repo.GetUser(ctx, telegramID)
	if err != nil {
		return err
	}

	if user == nil {
		return errors.New("user not found")
	}

	user.CardNumber = cardNumber
	return s.repo.UpdateUser(ctx, user)
}

func (s *UserService) GetUserByAddress(ctx context.Context, address string) (*models.User, error) {
	return s.repo.GetUserByAddress(ctx, address)
}

func (s *UserService) IsTransactionProcessed(ctx context.Context, txID string) (bool, error) {
	return s.repo.IsTransactionConfirmed(ctx, txID)
}

func (s *UserService) GetUser(ctx context.Context, telegramID int64) (*models.User, error) {
	return s.repo.GetUser(ctx, telegramID)
}

func (s *UserService) CreateOrUpdateTransaction(ctx context.Context, tx *models.Transaction) error {
	return s.repo.CreateOrUpdateTransaction(ctx, tx)
}

func (s *UserService) generateNewAddress() (string, error) {
	child, err := s.masterKey.Derive(s.addressIdx)
	if err != nil {
		return "", err
	}

	addr, err := child.Address(s.netParams)
	if err != nil {
		return "", err
	}

	s.addressIdx++
	return addr.EncodeAddress(), nil
}
