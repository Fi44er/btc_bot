package bot

import (
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Константы состояний пользователя
const (
	stateDefault                = ""
	stateAwaitingCardNumber     = "awaiting_card_number"
	stateAwaitingWithdrawAmount = "awaiting_withdraw_amount"
)

// sendMessage - унифицированная функция для отправки сообщений.
func (b *Bot) sendMessage(chatID int64, text string, replyMarkup interface{}) {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = tgbotapi.ModeMarkdown // По умолчанию используем Markdown для красивого форматирования
	if replyMarkup != nil {
		msg.ReplyMarkup = replyMarkup
	}
	if _, err := b.API.Send(msg); err != nil {
		b.logger.Errorf("Failed to send message: %v", err)
	}
}

func (b *Bot) isAdmin(userID int64) bool {
	return userID == b.config.AdminChatID
}

// --- Функции для управления состоянием ---

func (b *Bot) setState(userID int64, state string) {
	b.stateMutex.Lock()
	defer b.stateMutex.Unlock()
	if state == stateDefault {
		delete(b.userStates, userID) // Удаляем состояние, если оно по-умолчанию
	} else {
		b.userStates[userID] = state
	}
	b.logger.Debugf("Set state for user %d: %s", userID, state)
}

func (b *Bot) getUserState(userID int64) string {
	b.stateMutex.Lock()
	defer b.stateMutex.Unlock()
	return b.userStates[userID]
}

// --- Функции для управления временными данными ---

func (b *Bot) setUserActionData(userID int64, data string) {
	b.stateMutex.Lock()
	defer b.stateMutex.Unlock()
	b.userActionData[userID] = data
}

func (b *Bot) getUserActionData(userID int64) string {
	b.stateMutex.Lock()
	defer b.stateMutex.Unlock()
	return b.userActionData[userID]
}

func (b *Bot) clearUserActionData(userID int64) {
	b.stateMutex.Lock()
	defer b.stateMutex.Unlock()
	delete(b.userActionData, userID)
}
