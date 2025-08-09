package bot

import (
	"sync"

	"github.com/Fi44er/btc_bot/config"
	"github.com/Fi44er/btc_bot/internal/service"
	"github.com/Fi44er/btc_bot/utils"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type Bot struct {
	API         *tgbotapi.BotAPI
	userService *service.UserService
	logger      *utils.Logger
	userStates  map[int64]string
	stateMutex  *sync.Mutex
	config      *config.Config
}

func NewBot(
	api *tgbotapi.BotAPI,
	userService *service.UserService,
	logger *utils.Logger,
	config *config.Config,
) *Bot {
	return &Bot{
		API:         api,
		userService: userService,
		logger:      logger,
		userStates:  make(map[int64]string),
		stateMutex:  &sync.Mutex{},
		config:      config,
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

func GetMainMenu(hasAddress bool) tgbotapi.ReplyKeyboardMarkup {
	rows := [][]tgbotapi.KeyboardButton{
		{
			tgbotapi.NewKeyboardButton("💰 Получить адрес для пополнения"),
			tgbotapi.NewKeyboardButton("💳 Указать номер карты"),
		},
	}

	if hasAddress {
		rows = append(rows, []tgbotapi.KeyboardButton{
			tgbotapi.NewKeyboardButton("🔄 Проверить статус транзакции"),
		})
	}

	return tgbotapi.NewReplyKeyboard(rows...)
}
