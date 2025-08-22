package bot

import (
	"fmt"
	"github.com/Fi44er/btc_bot/internal/models"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func (b *Bot) notifyAboutTransaction(user *models.User, tx *models.Transaction) {
	b.logger.Infof("NOTIFY: Callback received for user %d, tx %s. Preparing notifications...", user.TelegramID, tx.TxID)

	rate, err := b.service.GetBTCRUBRate()
	if err != nil {
		b.logger.Warnf("Failed to get BTC/RUB rate for notification: %v", err)
		rate = 4000000.0
	}
	rubAmount := tx.AmountBTC * rate

	adminMsgText := fmt.Sprintf(
		"✅ Новое пополнение!\n\n"+
			"👤 *Пользователь:* `%d`\n"+
			"💳 *Карта:* `%s`\n"+
			"💰 *Сумма:* `%.8f` BTC (`~%.2f` RUB)\n"+
			"🧾 *Адрес:* `%s`\n"+
			"🔗 *TXID:* `%s`", user.TelegramID,
		user.CardNumber,
		tx.AmountBTC,
		rubAmount,
		tx.Address,
		tx.TxID)

	contactBtn := tgbotapi.NewInlineKeyboardButtonData(
		"✍️ Связаться с пользователем",
		fmt.Sprintf("contact_user:%d", user.TelegramID),
	)
	keyboard := tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(contactBtn))

	adminChatID := b.service.GetAdminChatID()
	b.logger.Infof("NOTIFY: Attempting to send notification to ADMIN with ChatID: %d", adminChatID)
	b.sendMessage(adminChatID, adminMsgText, keyboard)

	userMsg := fmt.Sprintf(
		"✅ Ваш баланс пополнен на `%.2f` RUB (из `%.8f` BTC).\n\n"+
			"Для вывода средств дождитесь, когда с вами свяжется администратор.",
		rubAmount, tx.AmountBTC,
	)
	b.logger.Infof("NOTIFY: Attempting to send notification to USER with ChatID: %d", user.TelegramID)
	b.sendMessage(user.TelegramID, userMsg, GetMainMenu(user))
}
