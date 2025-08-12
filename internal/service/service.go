package service

import (
	"context"
	"errors"

	"github.com/Fi44er/btc_bot/config"
	"github.com/Fi44er/btc_bot/internal/models"
	"github.com/Fi44er/btc_bot/utils"
	"github.com/btcsuite/btcd/btcutil/hdkeychain"
	"github.com/btcsuite/btcd/chaincfg"
	"gorm.io/gorm"
)

type Service struct {
	repo        Repository
	masterKey   *hdkeychain.ExtendedKey
	netParams   *chaincfg.Params
	addressIdx  uint32
	adminChatID int64
	logger      *utils.Logger
	config      *config.Config
}

type Repository interface {
	GetUser(ctx context.Context, telegramID int64) (*models.User, error)
	CreateUser(ctx context.Context, user *models.User) error
	UpdateUser(ctx context.Context, user *models.User, tx *gorm.DB) error
	GetUserByAddress(ctx context.Context, address string) (*models.User, error)
	GetAllUsersWithAddresses(ctx context.Context) ([]*models.User, error)

	GetTransaction(ctx context.Context, txID string) (*models.Transaction, error)
	CreateOrUpdateTransaction(ctx context.Context, tx *models.Transaction) error

	CreateWallet(ctx context.Context, wallet *models.SystemWallet, tx *gorm.DB) error
	GetWalletByID(ctx context.Context, id int64) (*models.SystemWallet, error)

	BeginTransaction(ctx context.Context) (*gorm.DB, error)
	Commit(tx *gorm.DB) error
	Rollback(tx *gorm.DB)
	WithTransaction(tx *gorm.DB) *gorm.DB

	GetAllWithdrawals(ctx context.Context) ([]models.Withdrawal, error)
	CreateWithdrawal(ctx context.Context, withdrawal *models.Withdrawal) error
	SumPendingWithdrawals(ctx context.Context, userID int64) (float64, error)
	GetPendingWithdrawalByUser(ctx context.Context, userID int64) (*models.Withdrawal, error)
	UpdateWithdrawal(ctx context.Context, withdrawal *models.Withdrawal) error

	GetPendingWithdrawals(ctx context.Context) ([]*models.Withdrawal, error)
	GetWithdrawalByID(ctx context.Context, id int64) (*models.Withdrawal, error)
	UpdateWithdrawalStatus(ctx context.Context, id int64, status string) error

	GetPendingWithdrawalByUserID(ctx context.Context, userID int64) (*models.Withdrawal, error)
	DeleteWithdrawal(ctx context.Context, id int64) error
	UpdateUserBalance(ctx context.Context, userID int64, newBalance float64) error
}

func NewUserService(repo Repository, masterKeySeed string, adminChatID int64, coconfig *config.Config, logger *utils.Logger) (*Service, error) {
	masterKey, err := hdkeychain.NewKeyFromString(masterKeySeed)
	if err != nil {
		return nil, err
	}

	return &Service{
		logger:      logger,
		config:      coconfig,
		repo:        repo,
		masterKey:   masterKey,
		netParams:   &chaincfg.TestNet3Params,
		adminChatID: adminChatID,
	}, nil
}

func (s *Service) GetAdminChatID() int64 {
	return s.adminChatID
}

func (s *Service) CreateUser(ctx context.Context, userID int64) error {
	user := &models.User{
		TelegramID: userID,
	}
	return s.repo.CreateUser(ctx, user)
}

func (s *Service) UpdateUserWallet(ctx context.Context, telegramID int64) (*models.User, error) {
	user, err := s.repo.GetUser(ctx, telegramID)
	if err != nil {
		return nil, err
	}

	if user == nil {
		return nil, errors.New("user not found")
	}

	if user.SystemWalletID != nil {
		s.logger.Warnf("User %d already has a wallet", telegramID)
		return user, nil
	}

	address, err := s.generateNewAddress()
	if err != nil {
		return nil, err
	}

	privateAddrKey, err := utils.GetAddressPrivateKey(s.config.MasterKeySeed, address, &chaincfg.TestNet3Params)
	if err != nil {
		s.logger.Errorf("Failed to get private key: %v", err)
		return nil, err
	}

	wallet := &models.SystemWallet{
		Address:    address,
		PrivateKey: privateAddrKey,
	}
	user.SystemWallet = wallet

	tx, err := s.repo.BeginTransaction(ctx)
	if err != nil {
		return nil, err
	}

	defer func() {
		if r := recover(); r != nil {
			s.logger.Errorf("Panic occurred: %v", r)
			s.repo.Rollback(tx)
		}
	}()

	if err := s.repo.CreateWallet(ctx, wallet, tx); err != nil {
		s.logger.Errorf("Failed to create wallet: %v", err)
		s.repo.Rollback(tx)
		return nil, err
	}

	if err := s.repo.UpdateUser(ctx, user, tx); err != nil {
		s.logger.Errorf("Failed to update user: %v", err)
		s.repo.Rollback(tx)
		return nil, err
	}

	if err := s.repo.Commit(tx); err != nil {
		s.logger.Errorf("Failed to commit transaction: %v", err)
		return nil, err
	}

	return user, nil
}

func (s *Service) UpdateCardNumber(ctx context.Context, telegramID int64, cardNumber string) error {
	user, err := s.repo.GetUser(ctx, telegramID)
	if err != nil {
		return err
	}

	if user == nil {
		return errors.New("user not found")
	}

	user.CardNumber = cardNumber
	return s.repo.UpdateUser(ctx, user, nil)
}

func (s *Service) GetUserByAddress(ctx context.Context, address string) (*models.User, error) {
	return s.repo.GetUserByAddress(ctx, address)
}

func (s *Service) IsTransactionProcessed(ctx context.Context, txID string) (*models.Transaction, error) {
	return s.repo.GetTransaction(ctx, txID)
}

func (s *Service) GetUser(ctx context.Context, telegramID int64) (*models.User, error) {
	return s.repo.GetUser(ctx, telegramID)
}

func (s *Service) CreateOrUpdateTransaction(ctx context.Context, tx *models.Transaction) error {
	return s.repo.CreateOrUpdateTransaction(ctx, tx)
}

func (s *Service) generateNewAddress() (string, error) {
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
