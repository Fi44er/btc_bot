package bot

import (
	"sync"

	"github.com/Fi44er/btc_bot/config"
	"github.com/Fi44er/btc_bot/internal/service"
	"github.com/Fi44er/btc_bot/utils"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const (
	stateDefault            = ""
	stateAwaitingCardNumber = "awaiting_card_number"
)

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

type Bot struct {
	API         *tgbotapi.BotAPI
	userService *service.UserService
	logger      *utils.Logger
	userStates  map[int64]string
	stateMutex  *sync.Mutex
	hasAddress  bool
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
		hasAddress:  false,
		config:      config,
	}
}

func (b *Bot) Start() {
	updates := b.API.GetUpdatesChan(tgbotapi.NewUpdate(0))
	for update := range updates {
		if update.CallbackQuery != nil {
			b.handleCallbackQuery(update.CallbackQuery)
			continue
		}
		if update.Message != nil {
			b.HandleUpdate(update)
		}
	}
}

func (b *Bot) setState(userID int64, state string) {
	b.stateMutex.Lock()
	defer b.stateMutex.Unlock()
	b.userStates[userID] = state
}

func (b *Bot) getUserState(userID int64) string {
	b.stateMutex.Lock()
	defer b.stateMutex.Unlock()
	return b.userStates[userID]
}
