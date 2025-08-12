package bot

import (
	"context"
	"sync"

	"github.com/Fi44er/btc_bot/config"
	"github.com/Fi44er/btc_bot/internal/models"
	"github.com/Fi44er/btc_bot/utils"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type IService interface {
	GetUser(ctx context.Context, userID int64) (*models.User, error)
	CreateUser(ctx context.Context, userID int64) error
	UpdateCardNumber(ctx context.Context, userID int64, cardNumber string) error
	HandleCheckTransactions(ctx context.Context, userID int64, _ models.NotifyCallback) (float64, error)
	GetAdminChatID() int64

	UpdateUserWallet(ctx context.Context, telegramID int64) (*models.User, error)
	GetBTCRUBRate() (float64, error)

	GetWalletByID(ctx context.Context, id int64) (*models.SystemWallet, error)

	GetPendingWithdrawals(ctx context.Context) ([]*models.Withdrawal, error)
	GetWithdrawalByID(ctx context.Context, id int64) (*models.Withdrawal, error)
	UpdateWithdrawalStatus(ctx context.Context, id int64, status string) error

	GetPendingWithdrawalByUserID(ctx context.Context, userID int64) (*models.Withdrawal, error)

	// НОВЫЙ МЕТОД: Удалить запись о выводе по ID
	DeleteWithdrawal(ctx context.Context, id int64) error

	UpdateUserBalance(ctx context.Context, userID int64, newBalance float64) error
	CreateOrUpdateWithdrawal(ctx context.Context, withdrawal *models.Withdrawal) (*models.Withdrawal, bool, error)
}

type Bot struct {
	API        *tgbotapi.BotAPI
	service    IService
	logger     *utils.Logger
	config     *config.Config
	stateMutex *sync.Mutex
	// Карта для хранения состояний пользователей (например, "ожидание ввода карты")
	userStates map[int64]string
	// Карта для хранения временных данных пользователя (например, сумма для вывода)
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
	// Используем метод isAdmin для проверки, а не захардкоженный ID
	isAdmin := user.IsAdmin

	// Если нет карты - показываем только кнопку для ввода карты
	if !hasCard {
		return tgbotapi.NewReplyKeyboard(
			tgbotapi.NewKeyboardButtonRow(
				tgbotapi.NewKeyboardButton("💳 Указать номер карты"),
			),
		)
	}

	// Полное меню для пользователей с картой
	rows := [][]tgbotapi.KeyboardButton{
		{
			tgbotapi.NewKeyboardButton("📊 Посмотреть баланс"),
			tgbotapi.NewKeyboardButton("💳 Изменить номер карты"),
		},
		{
			tgbotapi.NewKeyboardButton("💸 Вывести средства"),
			tgbotapi.NewKeyboardButton("🔄 Проверить статус транзакции"),
		},
		{
			tgbotapi.NewKeyboardButton("💰 Получить адрес для пополнения"),
		},
	}

	if isAdmin {
		rows = append(rows, []tgbotapi.KeyboardButton{
			tgbotapi.NewKeyboardButton("👨‍💻 Запросы на вывод"),
		})
	}

	return tgbotapi.NewReplyKeyboard(rows...)
}
