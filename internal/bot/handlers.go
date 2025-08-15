package bot

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/Fi44er/btc_bot/internal/models"
	"github.com/Fi44er/btc_bot/utils"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func (b *Bot) HandleUpdate(update tgbotapi.Update) {
	b.withUserCheck(func(ctx context.Context, update tgbotapi.Update, user *models.User) {
		text := update.Message.Text
		chatID := update.Message.Chat.ID
		userID := user.TelegramID

		b.logger.Infof("Processing message from user %d: %s", userID, text)

		userState := b.getUserState(userID)

		switch userState {
		case stateAwaitingCardNumber:
			b.handleCardNumberInput(ctx, update, user)
			return
		case stateAwaitingWithdrawConfirmationAmount:
			b.handleWithdrawConfirmation(ctx, chatID, user, text)
			return
		case stateAwaitingAdminNickname:
			b.handleAdminNicknameInput(ctx, chatID, text)
			return
		}

		switch text {
		case "/start":
			b.handleStart(ctx, chatID, user)
		case "💰 Получить адрес для пополнения":
			b.handleAddressRequest(ctx, chatID, user)
		case "💳 Указать номер карты", "💳 Изменить номер карты":
			b.setState(userID, stateAwaitingCardNumber)
			b.sendMessage(chatID, "Пожалуйста, отправьте номер вашей карты:", tgbotapi.NewRemoveKeyboard(true))
		case "📊 Посмотреть баланс":
			b.handleBalanceRequest(ctx, chatID, user)
		case "✅ Пришло на карту":
			b.handleWithdrawRequest(ctx, chatID, user)
		default:
			b.sendMessage(chatID, "Неизвестная команда. Используйте меню.", GetMainMenu(user))
		}
	})(update)
}

func (b *Bot) handleStart(ctx context.Context, chatID int64, user *models.User) {
	welcomeText := "Добро пожаловать! Используйте меню для работы с ботом."
	b.sendMessage(chatID, welcomeText, GetMainMenu(user))
}

func (b *Bot) handleAddressRequest(ctx context.Context, chatID int64, user *models.User) {
	if user.CardNumber == "" {
		b.sendMessage(
			chatID,
			"❌ Для получения адреса пополнения необходимо сначала указать номер карты.",
			GetMainMenu(user),
		)
		return
	}

	userWithAddress, err := b.service.UpdateUserWallet(ctx, user.TelegramID)
	if err != nil {
		b.logger.Errorf("Failed to get user address: %v", err)
		b.sendMessage(chatID, "Не удалось сгенерировать адрес. Попробуйте позже.", GetMainMenu(user))
		return
	}

	msgText := fmt.Sprintf(
		"Ваш уникальный адрес для пополнения:\n\n`%s`\n\n"+
			"Любое поступление BTC на него будет автоматически зачислено на ваш баланс после подтверждения в сети.",
		userWithAddress.SystemWallet.Address,
	)
	b.sendMessage(chatID, msgText, GetMainMenu(userWithAddress))
}

func (b *Bot) handleCallbackQuery(callback *tgbotapi.CallbackQuery) {
	ctx := context.Background()
	if !b.isAdmin(callback.From.ID) {
		b.answerCallback(callback.ID, "Это действие доступно только администратору.")
		return
	}

	if strings.HasPrefix(callback.Data, "contact_user:") {
		b.handleContactUserCallback(ctx, callback)
	}
}

func (b *Bot) handleCardNumberInput(ctx context.Context, update tgbotapi.Update, user *models.User) {
	chatID := update.Message.Chat.ID
	userID := user.TelegramID
	cardNumber := update.Message.Text

	if err := b.service.UpdateCardNumber(ctx, userID, cardNumber); err != nil {
		b.logger.Errorf("Failed to update card number: %v", err)
		b.sendMessage(chatID, "Ошибка сохранения номера карты. Попробуйте позже.", GetMainMenu(user))
		return
	}

	b.setState(userID, stateDefault)
	user.CardNumber = cardNumber
	b.sendMessage(chatID, "✅ Номер карты сохранен!", nil)
	b.sendMessage(chatID, "Выберите действие в меню:", GetMainMenu(user))
}

func (b *Bot) handleBalanceRequest(_ context.Context, chatID int64, user *models.User) {
	balance := utils.RoundTo(user.Balance, 2)
	msgText := fmt.Sprintf("Ваш текущий баланс: %.2f RUB", balance)
	b.sendMessage(chatID, msgText, GetMainMenu(user))
}

func (b *Bot) handleContactUserCallback(ctx context.Context, callback *tgbotapi.CallbackQuery) {
	adminID := callback.From.ID
	parts := strings.Split(strings.TrimPrefix(callback.Data, "contact_user:"), ":")
	if len(parts) != 1 {
		b.logger.Errorf("Invalid callback data for contact_user: %s", callback.Data)
		b.answerCallback(callback.ID, "Ошибка: неверные данные кнопки.")
		return
	}
	targetUserIDStr := parts[0]

	b.setUserActionData(adminID, targetUserIDStr)
	b.setState(adminID, stateAwaitingAdminNickname)

	edit := tgbotapi.NewEditMessageReplyMarkup(callback.Message.Chat.ID, callback.Message.MessageID, tgbotapi.InlineKeyboardMarkup{})
	b.API.Send(edit)

	msgText := "Введите ваш никнейм (например, @my\\_nickname), чтобы отправить его пользователю."
	b.sendMessage(adminID, msgText, tgbotapi.NewRemoveKeyboard(true))
	b.answerCallback(callback.ID, "")
}

func (b *Bot) handleAdminNicknameInput(ctx context.Context, adminChatID int64, nickname string) {
	targetUserIDStr := b.getUserActionData(adminChatID)
	targetUserID, err := strconv.ParseInt(targetUserIDStr, 10, 64)
	if err != nil {
		b.logger.Errorf("Failed to parse target user ID from action data: %v", err)
		return
	}

	b.clearUserActionData(adminChatID)
	b.setState(adminChatID, stateDefault)

	markdownEscaper := strings.NewReplacer(
		"_", "\\_", "*", "\\*", "[", "\\[", "]", "\\]",
		"(", "\\(", ")", "\\)", "~", "\\~", "`", "\\`",
		">", "\\>", "#", "\\#", "+", "\\+", "-", "\\-",
		"=", "\\=", "|", "\\|", "{", "\\{", "}", "\\}",
		".", "\\.", "!", "\\!",
	)
	safeNickname := markdownEscaper.Replace(nickname)

	userMsg := fmt.Sprintf(
		"🔵 Для организации вывода средств, пожалуйста, свяжитесь с администратором: %s\n\n"+
			"После получения перевода на карту, не забудьте нажать кнопку '✅ Подтвердить вывод' в главном меню.",
		safeNickname,
	)

	user, err := b.service.GetUser(ctx, targetUserID)
	if err != nil {
		b.logger.Errorf("Could not get user %d to send admin contact: %v", targetUserID, err)
		b.sendMessage(adminChatID, fmt.Sprintf("Не удалось найти пользователя %d для отправки сообщения.", targetUserID), nil)
		return
	}
	b.sendMessage(targetUserID, userMsg, GetMainMenu(user))

	adminConfirmMsg := fmt.Sprintf("✅ Ваши контактные данные (%s) отправлены пользователю %d.", safeNickname, targetUserID)
	b.sendMessage(adminChatID, adminConfirmMsg, nil)
}
