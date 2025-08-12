package bot

import (
	"context"

	"github.com/Fi44er/btc_bot/internal/models"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func (b *Bot) withUserCheck(handler func(context.Context, tgbotapi.Update, *models.User)) func(tgbotapi.Update) {
	return func(update tgbotapi.Update) {
		ctx := context.Background()
		userID := update.Message.From.ID

		user, err := b.service.GetUser(ctx, userID)
		if err != nil {
			b.logger.Errorf("Failed to get user: %v", err)
			b.sendMessage(update.Message.Chat.ID, "Произошла ошибка. Попробуйте позже.", nil)
			return
		}

		if user == nil {
			if err := b.service.CreateUser(ctx, userID); err != nil {
				b.logger.Errorf("Failed to create user: %v", err)
				b.sendMessage(update.Message.Chat.ID, "Произошла ошибка. Попробуйте позже.", nil)
				return
			}
			user, err = b.service.GetUser(ctx, userID)
			if err != nil {
				b.logger.Errorf("Failed to get user after creation: %v", err)
				return
			}
		}

		handler(ctx, update, user)
	}
}
