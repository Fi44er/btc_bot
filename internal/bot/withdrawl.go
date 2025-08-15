package bot

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/Fi44er/btc_bot/internal/models"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func (b *Bot) handleWithdrawRequest(ctx context.Context, chatID int64, user *models.User) {
	if user.Balance <= 0 {
		b.sendMessage(chatID, "❌ На вашем балансе нет средств для вывода.", GetMainMenu(user))
		return
	}
	msg := "Пожалуйста, введите точную сумму в RUB, которую вы получили на карту от оператора."
	b.setState(user.TelegramID, stateAwaitingWithdrawConfirmationAmount)
	b.sendMessage(chatID, msg, tgbotapi.NewRemoveKeyboard(true))
}

func (b *Bot) handleWithdrawConfirmation(ctx context.Context, chatID int64, user *models.User, text string) {
	b.setState(user.TelegramID, stateDefault)

	receivedAmount, err := strconv.ParseFloat(strings.Replace(text, ",", ".", -1), 64)
	if err != nil || receivedAmount <= 0 {
		b.sendMessage(chatID, "❌ Неверная сумма. Введите положительное число. Операция отменена.", GetMainMenu(user))
		return
	}

	amountToDeduct := (receivedAmount / 1.11) * 1.06

	if user.Balance < amountToDeduct {
		errorMsg := fmt.Sprintf(
			"❌ Недостаточно средств для списания.\n\n"+
				"Ваш баланс: `%.2f` RUB\n"+
				"Требуется для списания: `%.2f` RUB\n\n"+
				"Операция отменена. Пожалуйста, проверьте введенную сумму или свяжитесь с администратором.",
			user.Balance, amountToDeduct,
		)
		b.sendMessage(chatID, errorMsg, GetMainMenu(user))
		return
	}

	newBalance := user.Balance - amountToDeduct
	err = b.service.UpdateUserBalance(ctx, user.TelegramID, newBalance)
	if err != nil {
		b.logger.Errorf("Failed to update user balance on withdraw confirmation: %v", err)
		b.sendMessage(chatID, "❌ Произошла ошибка при обновлении баланса. Свяжитесь с поддержкой.", GetMainMenu(user))
		return
	}

	user.Balance = newBalance

	successMsg := fmt.Sprintf(
		"✅ Вывод на сумму `%.2f` RUB успешно подтвержден!\n\nВаш новый баланс: `%.2f` RUB.",
		receivedAmount, newBalance,
	)
	b.sendMessage(chatID, successMsg, GetMainMenu(user))

	adminMsg := fmt.Sprintf(
		"✅ Пользователь `%d` подтвердил получение `%.2f` RUB.\n\n"+
			"С его баланса списано `%.2f` RUB.\n"+
			"Новый баланс пользователя: `%.2f` RUB.",
		user.TelegramID, receivedAmount, amountToDeduct, newBalance,
	)
	b.sendMessage(b.service.GetAdminChatID(), adminMsg, nil)
}
