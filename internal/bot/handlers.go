package bot

import (
	"context"
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func (b *Bot) HandleUpdate(update tgbotapi.Update) {
	ctx := context.Background()
	userID := update.Message.From.ID
	chatID := update.Message.Chat.ID
	text := update.Message.Text

	user, err := b.userService.GetUser(ctx, userID)
	if err != nil {
		b.logger.Errorf("Failed to get user: %v", err)
		return
	}

	hasAddress := user != nil && user.DepositAddress != ""

	userState := b.getUserState(userID)

	if userState == stateAwaitingCardNumber {
		b.HandleCardNumberInput(ctx, update)
		return
	}

	switch text {
	case "/test_tx":
		b.handleTestTransaction(update.Message.Chat.ID, update.Message.From.ID)
	case "/start":
		b.handleStart(ctx, chatID, userID, hasAddress)

	case "💰 Получить адрес для пополнения":
		b.handleAddressRequest(ctx, chatID, userID)

	case "💳 Указать номер карты":
		b.setState(userID, stateAwaitingCardNumber)
		msg := tgbotapi.NewMessage(chatID, "Пожалуйста, отправьте номер вашей карты:")
		msg.ReplyMarkup = tgbotapi.NewRemoveKeyboard(true)
		b.API.Send(msg)

	case "🔄 Проверить статус транзакции":
		b.handleCheckTransactions(ctx, chatID, userID)

	default:
		msg := tgbotapi.NewMessage(chatID, "Неизвестная команда. Используйте меню.")
		msg.ReplyMarkup = GetMainMenu(hasAddress)
		b.API.Send(msg)
	}
}

func (b *Bot) handleCheckTransactions(ctx context.Context, chatID, userID int64) {
	user, err := b.userService.GetUser(ctx, userID)
	if err != nil || user == nil || user.DepositAddress == "" {
		msg := tgbotapi.NewMessage(chatID, "У вас нет активного адреса для проверки.")
		msg.ReplyMarkup = GetMainMenu(false)
		b.API.Send(msg)
		return
	}

	msg := tgbotapi.NewMessage(chatID, "🔍 Проверяю транзакции для вашего адреса...")
	b.API.Send(msg)

	transactions, err := b.checkUserTransactions(ctx, user.DepositAddress)
	if err != nil {
		b.logger.Errorf("Error checking transactions: %v", err)
		msg := tgbotapi.NewMessage(chatID, "Произошла ошибка при проверке транзакций.")
		msg.ReplyMarkup = GetMainMenu(true)
		b.API.Send(msg)
		return
	}

	if len(transactions) == 0 {
		msg := tgbotapi.NewMessage(chatID, "На вашем адресе пока нет новых транзакций.")
		msg.ReplyMarkup = GetMainMenu(true)
		b.API.Send(msg)
		return
	}

	response := "📊 Найдены транзакции:\n\n"
	for _, tx := range transactions {
		response += fmt.Sprintf("• %.8f BTC - %s\n", tx.AmountBTC, tx.TxID)
	}

	msg = tgbotapi.NewMessage(chatID, response)
	msg.ReplyMarkup = GetMainMenu(true)
	b.API.Send(msg)
}

func (b *Bot) handleStart(ctx context.Context, chatID, userID int64, hasAddress bool) {
	user, err := b.userService.GetUser(ctx, userID)
	if err != nil {
		b.logger.Errorf("Failed to get user: %v", err)
	}

	welcomeText := "Добро пожаловать! Используйте меню для работы с ботом:"

	if user == nil || user.CardNumber == "" {
		welcomeText += "\n\n1. Сначала укажите номер карты"
		welcomeText += "\n2. Затем получите адрес для пополнения"
	} else if !hasAddress {
		welcomeText += "\n\nТеперь вы можете получить адрес для пополнения"
	} else {
		welcomeText += "\n\nВы можете проверять статус транзакций"
	}

	msg := tgbotapi.NewMessage(chatID, welcomeText)
	msg.ReplyMarkup = GetMainMenu(hasAddress)
	b.API.Send(msg)
}

func (b *Bot) handleAddressRequest(ctx context.Context, chatID, userID int64) {
	user, err := b.userService.GetUser(ctx, userID)
	if err != nil {
		b.logger.Errorf("Failed to get user: %v", err)
		b.API.Send(tgbotapi.NewMessage(chatID, "Произошла ошибка. Попробуйте позже."))
		return
	}

	if user == nil || user.CardNumber == "" {
		msg := tgbotapi.NewMessage(chatID, "❌ Для получения адреса пополнения необходимо сначала указать номер карты.\n\n"+
			"Пожалуйста, нажмите кнопку '💳 Указать номер карты' и следуйте инструкциям.")
		msg.ReplyMarkup = GetMainMenu(false)
		b.API.Send(msg)
		return
	}

	userWithAddress, err := b.userService.UpdateAddress(ctx, userID)
	if err != nil {
		b.logger.Errorf("Failed to get user address: %v", err)
		b.API.Send(tgbotapi.NewMessage(chatID, "Не удалось сгенерировать адрес. Попробуйте позже."))
		return
	}

	msgText := fmt.Sprintf(
		"Ваш уникальный адрес для пополнения:\n\n`%s`\n\n"+
			"Отправляйте BTC только на этот адрес. Любое поступление на него будет зачислено на ваш баланс.",
		userWithAddress.DepositAddress,
	)
	msg := tgbotapi.NewMessage(chatID, msgText)
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = GetMainMenu(true)
	b.API.Send(msg)
}

func (b *Bot) HandleCardNumberInput(ctx context.Context, update tgbotapi.Update) {
	userID := update.Message.From.ID
	chatID := update.Message.Chat.ID

	user, err := b.userService.GetUser(ctx, userID)
	if err != nil {
		b.logger.Errorf("Failed to get user: %v", err)
		return
	}

	if user == nil {
		if err := b.userService.CreateUser(ctx, userID); err != nil {
			b.logger.Errorf("Failed to create user: %v", err)
			b.API.Send(tgbotapi.NewMessage(chatID, "Произошла ошибка. Попробуйте позже."))
			return
		}
	}

	if err := b.userService.UpdateCardNumber(ctx, userID, update.Message.Text); err != nil {
		b.logger.Errorf("Failed to update card number: %v", err)
		b.API.Send(tgbotapi.NewMessage(chatID, "Ошибка сохранения номера карты. Попробуйте позже."))
		return
	}

	user, err = b.userService.GetUser(ctx, userID)
	if err != nil {
		b.logger.Errorf("Failed to get user: %v", err)
		return
	}

	hasAddress := user != nil && user.DepositAddress != ""

	b.setState(userID, stateDefault)
	msg := tgbotapi.NewMessage(chatID, "✅ Номер карты сохранен!")
	msg.ReplyMarkup = GetMainMenu(hasAddress)
	b.API.Send(msg)
}
