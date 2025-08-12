package bot

import (
	"context"
	"fmt"
	"time"

	"github.com/Fi44er/btc_bot/internal/models"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func (b *Bot) handleTestTransaction(chatID, userID int64) {
	user, err := b.service.GetUser(context.Background(), userID)
	if err != nil || user == nil {
		msg := tgbotapi.NewMessage(chatID, "❌ Ошибка: пользователь не найден")
		b.API.Send(msg)
		return
	}

	testTx := &models.Transaction{
		TxID:      "test_tx_" + time.Now().Format("20060102150405"),
		UserID:    userID,
		Address:   user.SystemWallet.Address,
		AmountBTC: 0.001,
		Confirmed: true,
	}

	b.notifyAboutTransaction(user, testTx, true)

	msg := tgbotapi.NewMessage(chatID, "✅ Тестовая транзакция отправлена администратору")
	b.API.Send(msg)
}

func (b *Bot) handleCheckTransactions(ctx context.Context, chatID, userID int64) {
	user, err := b.service.GetUser(ctx, userID)
	if err != nil || user == nil {
		b.sendMessage(chatID, "❌ Пользователь не найден", nil)
		return
	}

	msg := tgbotapi.NewMessage(chatID, "🔍 Проверяю транзакции для вашего адреса...")
	b.API.Send(msg)

	rubAdded, err := b.service.HandleCheckTransactions(ctx, userID, nil)
	if err != nil {
		b.logger.Errorf("Error checking transactions: %v", err)
		b.sendMessage(chatID, err.Error(), GetMainMenu(user))
		return
	}

	if rubAdded > 0 {
		b.sendMessage(chatID, fmt.Sprintf("✅ На ваш баланс зачислено %.2f ₽", rubAdded), GetMainMenu(user))
	} else {
		b.sendMessage(chatID, "❌ Новых транзакций не найдено", GetMainMenu(user))
	}
}

func (b *Bot) notifyAboutTransaction(user *models.User, tx *models.Transaction, isTest bool) {
	rate, err := b.service.GetBTCRUBRate()
	if err != nil {
		b.logger.Warnf("Failed to get BTC/RUB rate: %v", err)
		rate = 3900027.0
	}
	rub := tx.AmountBTC * rate

	testNote := ""
	if isTest {
		testNote = "\n\n⚠️ ЭТО ТЕСТОВОЕ УВЕДОМЛЕНИЕ"
	}

	adminMsgText := fmt.Sprintf("✅ Новая транзакция%s\n\nПользователь: %d\nКарта: %s\nСумма: %.8f BTC (%.2f ₽)\nАдрес: %s\nTXID: %s",
		testNote,
		user.TelegramID,
		user.CardNumber,
		tx.AmountBTC,
		rub,
		tx.Address,
		tx.TxID,
	)

	btn := tgbotapi.NewInlineKeyboardButtonData("🔑 Показать приватный ключ",
		fmt.Sprintf("show_key:%v", user.SystemWallet.ID))
	keyboard := tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(btn))

	adminMsg := tgbotapi.NewMessage(b.service.GetAdminChatID(), adminMsgText)
	adminMsg.ParseMode = "Markdown"
	adminMsg.ReplyMarkup = keyboard
	b.API.Send(adminMsg)

	if !isTest {
		userMsg := tgbotapi.NewMessage(
			user.TelegramID,
			fmt.Sprintf("💸 Получено %.8f BTC\n\nTransaction ID: %s", tx.AmountBTC, tx.TxID),
		)
		b.API.Send(userMsg)
	}
}
