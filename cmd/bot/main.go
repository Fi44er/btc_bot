package main

import (
	"github.com/Fi44er/btc_bot/config"
	"github.com/Fi44er/btc_bot/db"
	"github.com/Fi44er/btc_bot/internal/bot"
	"github.com/Fi44er/btc_bot/internal/repository"
	"github.com/Fi44er/btc_bot/internal/service"
	"github.com/Fi44er/btc_bot/utils"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func main() {
	logger := utils.InitLogger()
	cfg, err := config.LoadConfig(".env")
	if err != nil {
		logger.Fatal("Failed to load config: ", err)
	}

	database, err := db.ConnectDb(cfg.DB_URL, logger)
	if err != nil {
		logger.Fatal(err)
	}

	if err := db.Migrate(database, true, logger); err != nil {
		logger.Fatal(err)
	}

	repo := repository.NewRepository(database)
	userService, err := service.NewUserService(repo, cfg.MasterKeySeed, cfg.AdminChatID)
	if err != nil {
		logger.Fatal("Failed to create user service: ", err)
	}

	telegramBot, err := tgbotapi.NewBotAPI(cfg.TelegramBotToken)
	if err != nil {
		logger.Fatal("Failed to create bot API: ", err)
	}

	bot := bot.NewBot(telegramBot, userService, logger)
	bot.Start()
}
