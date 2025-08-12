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
			b.HandleCardNumberInput(ctx, update, user)
			return
		case stateAwaitingWithdrawAmount:
			b.handleWithdrawAmount(ctx, chatID, user, text)
			return
		}

		// Добавляем в модель User поле IsAdmin, чтобы не вызывать b.isAdmin постоянно
		user.IsAdmin = b.isAdmin(userID)

		switch text {
		case "/test_tx":
			b.handleTestTransaction(chatID, userID)
		case "/start":
			b.handleStart(ctx, chatID, user)
		case "💰 Получить адрес для пополнения":
			b.handleAddressRequest(ctx, chatID, user)
		case "💳 Указать номер карты", "💳 Изменить номер карты":
			b.setState(userID, stateAwaitingCardNumber)
			// Удаляем старое меню перед запросом ввода
			b.sendMessage(chatID, "Пожалуйста, отправьте номер вашей карты:", tgbotapi.NewRemoveKeyboard(true))
		case "🔄 Проверить статус транзакции":
			b.handleCheckTransactions(ctx, chatID, userID)
		case "📊 Посмотреть баланс":
			b.handleBalanceRequest(ctx, chatID, user)
		case "💸 Вывести средства":
			b.handleWithdrawRequest(ctx, chatID, user)
		case "👨‍💻 Запросы на вывод":
			if b.isAdmin(userID) {
				b.handleWithdrawalRequests(ctx, chatID, user)
			}
		default:
			b.sendMessage(chatID, "Неизвестная команда. Используйте меню.", GetMainMenu(user))
		}
	})(update)
}

func (b *Bot) handleStart(ctx context.Context, chatID int64, user *models.User) {
	welcomeText := "Добро пожаловать! Используйте меню для работы с ботом."
	// Отправляем приветствие и всегда показываем актуальное меню
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
			"Отправляйте BTC только на этот адрес. Любое поступление на него будет зачислено на ваш баланс.",
		userWithAddress.SystemWallet.Address,
	)

	// Отправляем сообщение и снова показываем меню
	b.sendMessage(chatID, msgText, GetMainMenu(userWithAddress))
}

// handleCallbackQuery теперь является маршрутизатором для колбэков
func (b *Bot) handleCallbackQuery(callback *tgbotapi.CallbackQuery) {
	ctx := context.Background()
	user, err := b.service.GetUser(ctx, callback.From.ID)
	if err != nil || user == nil {
		b.logger.Errorf("Failed to get user for callback: %v", err)
		return
	}
	user.IsAdmin = b.isAdmin(user.TelegramID)

	switch {
	case strings.HasPrefix(callback.Data, "show_key:"):
		b.handleShowKeyCallback(ctx, callback, user)
	case strings.HasPrefix(callback.Data, "confirm_withdraw"), strings.HasPrefix(callback.Data, "cancel_withdraw"):
		b.handleUserWithdrawCallback(ctx, callback, user)
	case strings.HasPrefix(callback.Data, "admin_"):
		if user.IsAdmin {
			b.handleAdminWithdrawCallback(ctx, callback)
		}
	}
}

func (b *Bot) HandleCardNumberInput(ctx context.Context, update tgbotapi.Update, user *models.User) {
	chatID := update.Message.Chat.ID
	userID := user.TelegramID
	cardNumber := update.Message.Text

	if err := b.service.UpdateCardNumber(ctx, userID, cardNumber); err != nil {
		b.logger.Errorf("Failed to update card number: %v", err)
		// Возвращаем главное меню даже в случае ошибки
		b.sendMessage(chatID, "Ошибка сохранения номера карты. Попробуйте позже.", GetMainMenu(user))
		return
	}

	// Сбрасываем состояние пользователя
	b.setState(userID, stateDefault)

	// Обновляем данные пользователя в текущем объекте, чтобы меню было актуальным
	user.CardNumber = cardNumber

	// Отправляем подтверждение и следом сообщение с главным меню
	b.sendMessage(chatID, "✅ Номер карты сохранен!", nil)
	b.sendMessage(chatID, "Выберите действие в меню:", GetMainMenu(user))
}

func (b *Bot) handleBalanceRequest(_ context.Context, chatID int64, user *models.User) {
	balance := utils.RoundTo(user.Balance, 8) // Увеличил точность для BTC
	msgText := fmt.Sprintf("Ваш текущий баланс: %.8f RUB", balance)
	b.sendMessage(chatID, msgText, GetMainMenu(user))
}

// handleShowKeyCallback обрабатывает запрос на показ приватного ключа
func (b *Bot) handleShowKeyCallback(ctx context.Context, callback *tgbotapi.CallbackQuery, user *models.User) {
	walletIDStr := strings.TrimPrefix(callback.Data, "show_key:")
	walletID, err := strconv.ParseInt(walletIDStr, 10, 64)
	if err != nil {
		b.logger.Errorf("Invalid wallet ID in callback: %v", err)
		return
	}

	wallet, err := b.service.GetWalletByID(ctx, walletID)
	if err != nil || wallet == nil {
		b.logger.Errorf("Failed to get wallet: %v", err)
		b.answerCallback(callback.ID, "Не удалось найти кошелек.")
		return
	}

	response := fmt.Sprintf("🔐 Данные кошелька:\n\nАдрес: `%s`\nПриватный ключ: `%s`", wallet.Address, wallet.PrivateKey)
	msg := tgbotapi.NewMessage(callback.Message.Chat.ID, response)
	msg.ParseMode = tgbotapi.ModeMarkdown
	b.API.Send(msg)

	// Убираем инлайн-кнопку из сообщения
	edit := tgbotapi.NewEditMessageReplyMarkup(callback.Message.Chat.ID, callback.Message.MessageID, tgbotapi.InlineKeyboardMarkup{})
	b.API.Send(edit)

	b.answerCallback(callback.ID, "Приватный ключ показан.")
}
