package bot

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/Fi44er/btc_bot/internal/models"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const (
	withdrawalsPerPage  = 5
	adminCommissionRate = 0.06
)

func applyAdminCommission(amount float64) float64 {
	return amount * (1 - adminCommissionRate)
}

// --- Логика вывода для пользователя (без изменений) ---

func (b *Bot) handleWithdrawRequest(ctx context.Context, chatID int64, user *models.User) {
	if user.Balance <= 0 {
		b.sendMessage(chatID, "❌ На вашем балансе недостаточно средств для вывода.", GetMainMenu(user))
		return
	}
	msg := fmt.Sprintf(
		"💰 Ваш текущий баланс: `%.8f` RUB\n\nВведите сумму для вывода:",
		user.Balance,
	)
	b.setState(user.TelegramID, stateAwaitingWithdrawAmount)
	b.sendMessage(chatID, msg, tgbotapi.NewRemoveKeyboard(true))
}

func (b *Bot) handleWithdrawAmount(ctx context.Context, chatID int64, user *models.User, text string) {
	amountToAdd, err := strconv.ParseFloat(strings.Replace(text, ",", ".", -1), 64)
	if err != nil || amountToAdd <= 0 {
		b.sendMessage(chatID, "❌ Неверная сумма. Введите положительное число.", tgbotapi.NewRemoveKeyboard(true))
		return
	}
	existingWithdrawal, err := b.service.GetPendingWithdrawalByUserID(ctx, user.TelegramID)
	if err != nil {
		b.logger.Errorf("Ошибка получения ожидающего вывода для пользователя %d: %v", user.TelegramID, err)
		b.sendMessage(chatID, "❌ Произошла внутренняя ошибка. Попробуйте позже.", GetMainMenu(user))
		return
	}
	var pendingAmount float64
	if existingWithdrawal != nil {
		pendingAmount = existingWithdrawal.Amount
	}
	availableBalance := user.Balance - pendingAmount
	if amountToAdd > availableBalance {
		b.setState(user.TelegramID, stateDefault)
		errorMsg := fmt.Sprintf(
			"❌ Недостаточно средств для добавления к выводу.\n\n"+
				"Ваш общий баланс: `%.8f` RUB\n"+
				"Уже в заявке на вывод: `%.8f` RUB\n"+
				"----------------------------------\n"+
				"*Доступно для вывода: `%.8f` RUB*\n\n"+
				"Вы пытаетесь добавить еще `%.8f` RUB, что превышает доступный лимит.",
			user.Balance, pendingAmount, availableBalance, amountToAdd,
		)
		b.sendMessage(chatID, errorMsg, GetMainMenu(user))
		return
	}
	b.setUserActionData(user.TelegramID, text)
	b.setState(user.TelegramID, stateDefault)
	msg := fmt.Sprintf(
		"Подтвердите операцию:\n\n➡️ Добавить к выводу: `%.8f RUB`\n💳 На карту: `%s`",
		amountToAdd, user.CardNumber,
	)
	confirmKeyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✅ Подтвердить", "confirm_withdraw"),
			tgbotapi.NewInlineKeyboardButtonData("❌ Отменить", "cancel_withdraw"),
		),
	)
	b.sendMessage(chatID, msg, confirmKeyboard)
}

func (b *Bot) handleUserWithdrawCallback(ctx context.Context, callback *tgbotapi.CallbackQuery, user *models.User) {
	chatID := callback.Message.Chat.ID
	b.answerCallback(callback.ID, "")
	editMarkup := tgbotapi.NewEditMessageReplyMarkup(chatID, callback.Message.MessageID, tgbotapi.InlineKeyboardMarkup{})
	b.API.Send(editMarkup)
	switch callback.Data {
	case "confirm_withdraw":
		amountStr := b.getUserActionData(user.TelegramID)
		b.clearUserActionData(user.TelegramID)
		amountToAdd, err := strconv.ParseFloat(strings.Replace(amountStr, ",", ".", -1), 64)
		if err != nil {
			b.sendMessage(chatID, "❌ Ошибка обработки суммы.", GetMainMenu(user))
			return
		}
		deltaWithdrawal := &models.Withdrawal{
			UserID:     user.TelegramID,
			CardNumber: user.CardNumber,
			Amount:     amountToAdd,
		}
		finalWithdrawal, isUpdate, err := b.service.CreateOrUpdateWithdrawal(ctx, deltaWithdrawal)
		if err != nil {
			b.logger.Errorf("Ошибка при создании/обновлении вывода: %v", err)
			b.sendMessage(chatID, "❌ "+err.Error(), GetMainMenu(user))
			return
		}
		if isUpdate {
			b.sendMessage(chatID, fmt.Sprintf("✅ Ваш существующий запрос на вывод обновлен. Новая сумма: `%.8f` RUB.", finalWithdrawal.Amount), GetMainMenu(user))
			b.notifyAdminAboutUpdatedWithdrawal(finalWithdrawal, amountToAdd)
		} else {
			b.sendMessage(chatID, fmt.Sprintf("✅ Запрос на вывод `%.8f` RUB успешно создан и отправлен на обработку.", finalWithdrawal.Amount), GetMainMenu(user))
			b.notifyAdminAboutWithdrawal(finalWithdrawal)
		}
	case "cancel_withdraw":
		b.clearUserActionData(user.TelegramID)
		b.sendMessage(chatID, "❌ Операция отменена.", GetMainMenu(user))
	}
}

// --- Уведомления для Админа (с комиссией) ---

func (b *Bot) notifyAdminAboutWithdrawal(withdrawal *models.Withdrawal) {
	amountToPay := applyAdminCommission(withdrawal.Amount)
	msg := fmt.Sprintf(
		"🆕 Новый запрос на вывод #%d\n\n"+
			"👤 Пользователь: `%d`\n"+
			"💳 Карта: `%s`\n"+
			"💰 Сумма к выплате: `%.8f` RUB (запрошено: `%.8f` RUB)",
		withdrawal.ID,
		withdrawal.UserID,
		withdrawal.CardNumber,
		amountToPay,
		withdrawal.Amount,
	)
	adminMsg := tgbotapi.NewMessage(b.config.AdminChatID, msg)
	adminMsg.ParseMode = tgbotapi.ModeMarkdown
	b.API.Send(adminMsg)
}

func (b *Bot) notifyAdminAboutUpdatedWithdrawal(withdrawal *models.Withdrawal, addedAmount float64) {
	adjustedAdded := applyAdminCommission(addedAmount)
	adjustedTotal := applyAdminCommission(withdrawal.Amount)
	msg := fmt.Sprintf(
		"🔄 Сумма в запросе #%d обновлена\n\n"+
			"👤 Пользователь: `%d`\n"+
			"💳 Карта: `%s`\n\n"+
			"💰 Добавлено к выплате: `%.8f` RUB\n"+
			"💰 *Итоговая сумма к выплате: `%.8f` RUB*",
		withdrawal.ID,
		withdrawal.UserID,
		withdrawal.CardNumber,
		adjustedAdded,
		adjustedTotal,
	)
	adminMsg := tgbotapi.NewMessage(b.config.AdminChatID, msg)
	adminMsg.ParseMode = tgbotapi.ModeMarkdown
	b.API.Send(adminMsg)
}

// --- Логика вывода для админа (БЕЗ общей суммы) ---

func (b *Bot) handleWithdrawalRequests(ctx context.Context, chatID int64, user *models.User) {
	withdrawals, err := b.service.GetPendingWithdrawals(ctx)
	if err != nil {
		b.logger.Errorf("Failed to get pending withdrawals: %v", err)
		b.sendMessage(chatID, "❌ Ошибка получения запросов на вывод", nil)
		return
	}
	if len(withdrawals) == 0 {
		b.sendMessage(chatID, "ℹ️ Нет ожидающих запросов на вывод.", nil)
		return
	}
	// Просто отправляем первую страницу без подсчета общей суммы
	b.sendWithdrawalsPage(ctx, chatID, withdrawals, 0)
}

func (b *Bot) sendWithdrawalsPage(ctx context.Context, chatID int64, withdrawals []*models.Withdrawal, page int) {
	start := page * withdrawalsPerPage
	if start >= len(withdrawals) {
		start = 0
		page = 0
	}
	end := start + withdrawalsPerPage
	if end > len(withdrawals) {
		end = len(withdrawals)
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📋 Запросы на вывод (страница %d из %d):\n\n", page+1, (len(withdrawals)-1)/withdrawalsPerPage+1))

	for i := start; i < end; i++ {
		w := withdrawals[i]
		amountToPay := applyAdminCommission(w.Amount)
		sb.WriteString(fmt.Sprintf(
			"🆔 ID: %d\n👤 Пользователь: %d\n💳 Карта: %s\n💰 Сумма к выплате: `%.8f` RUB\n\n",
			w.ID,
			w.UserID,
			w.CardNumber,
			amountToPay,
		))
	}
	keyboardRows := make([][]tgbotapi.InlineKeyboardButton, 0)
	for i := start; i < end; i++ {
		w := withdrawals[i]
		btn := tgbotapi.NewInlineKeyboardButtonData(
			fmt.Sprintf("✅ Подтвердить вывод #%d", w.ID),
			fmt.Sprintf("admin_confirm_withdraw:%d", w.ID),
		)
		keyboardRows = append(keyboardRows, tgbotapi.NewInlineKeyboardRow(btn))
	}
	if len(withdrawals) > withdrawalsPerPage {
		paginationRow := make([]tgbotapi.InlineKeyboardButton, 0)
		if page > 0 {
			paginationRow = append(paginationRow, tgbotapi.NewInlineKeyboardButtonData("⬅️ Назад", fmt.Sprintf("admin_withdraw_page:%d", page-1)))
		}
		if end < len(withdrawals) {
			paginationRow = append(paginationRow, tgbotapi.NewInlineKeyboardButtonData("Вперед ➡️", fmt.Sprintf("admin_withdraw_page:%d", page+1)))
		}
		if len(paginationRow) > 0 {
			keyboardRows = append(keyboardRows, paginationRow)
		}
	}
	msg := tgbotapi.NewMessage(chatID, sb.String())
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(keyboardRows...)
	b.API.Send(msg)
}

func (b *Bot) handleAdminWithdrawCallback(ctx context.Context, callback *tgbotapi.CallbackQuery) {
	data := callback.Data

	if strings.HasPrefix(data, "admin_withdraw_page:") {
		page, err := strconv.Atoi(strings.TrimPrefix(data, "admin_withdraw_page:"))
		if err != nil {
			b.logger.Errorf("Invalid page number in callback: %v", err)
			return
		}

		withdrawals, err := b.service.GetPendingWithdrawals(ctx)
		if err != nil {
			b.logger.Errorf("Failed to get pending withdrawals: %v", err)
			return
		}

		b.sendWithdrawalsPage(ctx, callback.Message.Chat.ID, withdrawals, page)
		b.answerCallback(callback.ID, "")
		return
	}

	if strings.HasPrefix(data, "admin_confirm_withdraw:") {
		withdrawID, err := strconv.ParseInt(strings.TrimPrefix(data, "admin_confirm_withdraw:"), 10, 64)
		if err != nil {
			b.logger.Errorf("Invalid withdrawal ID in callback: %v", err)
			return
		}

		confirmText := "Вы уверены, что хотите подтвердить этот вывод? Это действие необратимо."
		confirmKeyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData(
					"✅ Да, подтвердить",
					fmt.Sprintf("admin_final_confirm_withdraw:%d", withdrawID),
				),
				tgbotapi.NewInlineKeyboardButtonData("❌ Отмена", "admin_cancel_action"),
			),
		)

		edit := tgbotapi.NewEditMessageTextAndMarkup(
			callback.Message.Chat.ID,
			callback.Message.MessageID,
			confirmText,
			confirmKeyboard,
		)
		b.API.Send(edit)
		b.answerCallback(callback.ID, "")
		return
	}

	if strings.HasPrefix(data, "admin_final_confirm_withdraw:") {
		withdrawID, err := strconv.ParseInt(strings.TrimPrefix(data, "admin_final_confirm_withdraw:"), 10, 64)
		if err != nil {
			b.logger.Errorf("Invalid withdrawal ID in callback: %v", err)
			return
		}

		err = b.processWithdrawal(ctx, withdrawID)
		if err != nil {
			b.logger.Errorf("Failed to process withdrawal %d: %v", withdrawID, err)
			b.answerCallback(callback.ID, "❌ Ошибка обработки вывода: "+err.Error())
			return
		}

		deleteMsg := tgbotapi.NewDeleteMessage(callback.Message.Chat.ID, callback.Message.MessageID)
		b.API.Send(deleteMsg)
		b.answerCallback(callback.ID, "✅ Вывод успешно обработан и удален.")
		return
	}

	if data == "admin_cancel_action" {
		edit := tgbotapi.NewEditMessageText(
			callback.Message.Chat.ID,
			callback.Message.MessageID,
			"❌ Действие отменено.",
		)
		b.API.Send(edit)
		b.answerCallback(callback.ID, "")
	}
}

func (b *Bot) processWithdrawal(ctx context.Context, withdrawID int64) error {
	withdrawal, err := b.service.GetWithdrawalByID(ctx, withdrawID)
	if err != nil {
		return fmt.Errorf("не удалось получить заявку: %v", err)
	}
	if withdrawal == nil {
		return fmt.Errorf("заявка #%d не найдена (возможно, уже обработана)", withdrawID)
	}
	if withdrawal.Status != "pending" {
		return fmt.Errorf("заявка уже обработана (статус: %s)", withdrawal.Status)
	}

	user, err := b.service.GetUser(ctx, withdrawal.UserID)
	if err != nil {
		return fmt.Errorf("не удалось получить пользователя: %v", err)
	}
	if user == nil {
		return fmt.Errorf("пользователь для заявки не найден")
	}

	if user.Balance < withdrawal.Amount {
		b.sendMessage(b.config.AdminChatID, fmt.Sprintf("‼️ ВНИМАНИЕ: Недостаточно средств для вывода #%d. Баланс пользователя: %.8f RUB, требуется: %.8f RUB.", withdrawID, user.Balance, withdrawal.Amount), nil)
		return fmt.Errorf("недостаточно средств на балансе пользователя")
	}

	newBalance := user.Balance - withdrawal.Amount
	err = b.service.UpdateUserBalance(ctx, user.TelegramID, newBalance)
	if err != nil {
		return fmt.Errorf("не удалось обновить баланс пользователя: %v", err)
	}

	err = b.service.DeleteWithdrawal(ctx, withdrawID)
	if err != nil {
		b.logger.Errorf("CRITICAL: User balance updated for withdrawal %d, but failed to delete the withdrawal record: %v", withdrawID, err)
		return fmt.Errorf("не удалось удалить заявку после списания баланса: %v", err)
	}

	userMsg := fmt.Sprintf(
		"✅ Ваш вывод на сумму `%.8f` RUB (карта `%s`) успешно обработан!",
		withdrawal.Amount,
		withdrawal.CardNumber,
	)

	user.Balance = newBalance
	b.sendMessage(user.TelegramID, userMsg, GetMainMenu(user))

	return nil
}

func (b *Bot) answerCallback(callbackID string, text string) {
	callback := tgbotapi.NewCallback(callbackID, text)
	if _, err := b.API.Request(callback); err != nil {
		b.logger.Errorf("Failed to answer callback: %v", err)
	}
}
