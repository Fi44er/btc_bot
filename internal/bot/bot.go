package bot

import (
	"context"
	"sync"
	"time"

	"github.com/Fi44er/btc_bot/config"
	"github.com/Fi44er/btc_bot/internal/models"
	"github.com/Fi44er/btc_bot/utils"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type IService interface {
	GetUser(ctx context.Context, userID int64) (*models.User, error)
	GetUsersWithWallets(ctx context.Context) ([]*models.User, error)
	CreateUser(ctx context.Context, userID int64) error
	UpdateCardNumber(ctx context.Context, userID int64, cardNumber string) error
	HandleCheckTransactions(ctx context.Context, userID int64, notifyCallback models.NotifyCallback) (float64, error)
	GetAdminChatID() int64

	UpdateUserWallet(ctx context.Context, telegramID int64) (*models.User, error)
	GetBTCRUBRate() (float64, error)

	UpdateUserBalance(ctx context.Context, userID int64, newBalance float64) error
}

type Bot struct {
	API            *tgbotapi.BotAPI
	service        IService
	logger         *utils.Logger
	config         *config.Config
	stateMutex     *sync.Mutex
	userStates     map[int64]string
	userActionData map[int64]string
}

func NewBot(
	api *tgbotapi.BotAPI,
	service IService,
	logger *utils.Logger,
	config *config.Config,
) *Bot {
	return &Bot{
		API:            api,
		service:        service,
		logger:         logger,
		config:         config,
		stateMutex:     &sync.Mutex{},
		userStates:     make(map[int64]string),
		userActionData: make(map[int64]string),
	}
}

func (b *Bot) Start() {
	b.logger.Info("Starting bot...")

	go b.startTransactionChecker()

	updates := b.API.GetUpdatesChan(tgbotapi.NewUpdate(0))
	for update := range updates {
		b.logger.Debugf("Received update: %+v", update)
		if update.CallbackQuery != nil {
			b.handleCallbackQuery(update.CallbackQuery)
			continue
		}
		if update.Message != nil {
			b.HandleUpdate(update)
		}
	}
}

func GetMainMenu(user *models.User) tgbotapi.ReplyKeyboardMarkup {
	hasCard := user.CardNumber != ""

	if !hasCard {
		return tgbotapi.NewReplyKeyboard(
			tgbotapi.NewKeyboardButtonRow(
				tgbotapi.NewKeyboardButton("💳 Указать номер карты"),
			),
		)
	}

	// Меню для пользователя с картой.
	rows := [][]tgbotapi.KeyboardButton{
		{
			tgbotapi.NewKeyboardButton("📊 Посмотреть баланс"),
			tgbotapi.NewKeyboardButton("💳 Изменить номер карты"),
		},
		{
			tgbotapi.NewKeyboardButton("💰 Получить адрес для пополнения"),
			// Новая кнопка для подтверждения вывода
			tgbotapi.NewKeyboardButton("✅ Подтвердить вывод"),
		},
	}

	return tgbotapi.NewReplyKeyboard(rows...)
}

func (b *Bot) startTransactionChecker() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	b.logger.Info("Transaction checker started")

	for range ticker.C {
		ctx := context.Background()
		b.logger.Info("Running scheduled transaction check...")

		users, err := b.service.GetUsersWithWallets(ctx)
		if err != nil {
			b.logger.Errorf("Failed to get users with wallets for checking: %v", err)
			continue
		}

		if len(users) == 0 {
			b.logger.Info("No users with wallets to check.")
			continue
		}

		b.logger.Infof("Checking transactions for %d users...", len(users))
		for _, user := range users {
			_, err := b.service.HandleCheckTransactions(ctx, user.TelegramID, b.notifyAboutTransaction)
			if err != nil {
				b.logger.Warnf("Error checking transaction for user %d: %v", user.TelegramID, err)
			}
			time.Sleep(1 * time.Second)
		}
	}
}
