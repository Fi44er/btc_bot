package bot

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/Fi44er/btc_bot/internal/models"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

var (
	testnet_api_url = "https://mempool.space/testnet4/api"
	mainnet_api_url = "https://mempool.space/api"
)

func (b *Bot) handleTestTransaction(chatID, userID int64) {
	user, err := b.userService.GetUser(context.Background(), userID)
	if err != nil || user == nil {
		msg := tgbotapi.NewMessage(chatID, "❌ Ошибка: пользователь не найден")
		b.API.Send(msg)
		return
	}

	testTx := &models.Transaction{
		TxID:      "test_tx_" + time.Now().Format("20060102150405"),
		UserID:    userID,
		Address:   user.DepositAddress,
		AmountBTC: 0.001,
		Confirmed: true,
	}

	b.notifyAboutTransaction(user, testTx, true)

	msg := tgbotapi.NewMessage(chatID, "✅ Тестовая транзакция отправлена администратору")
	b.API.Send(msg)
}

func (b *Bot) checkUserTransactions(ctx context.Context, address string) ([]*models.Transaction, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	url := fmt.Sprintf("%s/address/%s/txs", testnet_api_url, address)

	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %v", err)
	}

	var apiResponse []struct {
		TxID string `json:"txid"`
		Vout []struct {
			Address string `json:"scriptpubkey_address"`
			Value   uint64 `json:"value"`
		} `json:"vout"`
		Status struct {
			Confirmed bool `json:"confirmed"`
		}
	}

	if err := json.Unmarshal(body, &apiResponse); err != nil {
		b.logger.Errorf("Failed to decode JSON: %v\nRaw response: %s", err, string(body))
		return nil, fmt.Errorf("invalid API response format")
	}

	var newTransactions []*models.Transaction

	for _, tx := range apiResponse {
		for _, output := range tx.Vout {
			if output.Address == address {
				amountBTC := float64(output.Value) / 1e8
				if err := b.processTransaction(ctx, tx.TxID, address, amountBTC, tx.Status.Confirmed); err != nil {
					b.logger.Errorf("Transaction processing failed: %v", err)
					continue
				}

				newTransactions = append(newTransactions, &models.Transaction{
					TxID:      tx.TxID,
					Address:   address,
					AmountBTC: amountBTC,
					Confirmed: tx.Status.Confirmed,
				})
			}
		}
	}

	return newTransactions, nil
}

func (b *Bot) processTransaction(ctx context.Context, txID, address string, amountBTC float64, confirmed bool) error {
	if amountBTC <= 0 {
		return fmt.Errorf("invalid amount: %.8f BTC", amountBTC)
	}

	if exists, _ := b.userService.IsTransactionProcessed(ctx, txID); exists {
		return nil
	}

	user, err := b.userService.GetUserByAddress(ctx, address)
	if err != nil {
		return fmt.Errorf("failed to get user: %v", err)
	}

	tx := &models.Transaction{
		TxID:      txID,
		UserID:    user.TelegramID,
		Address:   address,
		AmountBTC: amountBTC,
		Confirmed: confirmed,
	}

	if err := b.userService.CreateOrUpdateTransaction(ctx, tx); err != nil {
		return fmt.Errorf("failed to save transaction: %v", err)
	}

	if confirmed {
		b.notifyAboutTransaction(user, tx, false)
	}
	return nil
}

func (b *Bot) notifyAboutTransaction(user *models.User, tx *models.Transaction, isTest bool) {
	rate, err := getBTCRUBRate()
	if err != nil {
		b.logger.Warnf("Failed to get BTC/RUB rate: %v", err)
		rate = 6500000.0
	}
	rub := tx.AmountBTC * rate

	testNote := ""
	if isTest {
		testNote = "\n\n⚠️ ЭТО ТЕСТОВОЕ УВЕДОМЛЕНИЕ"
	}

	adminMsg := tgbotapi.NewMessage(
		b.userService.GetAdminChatID(),
		fmt.Sprintf("✅ Новая транзакция%s\n\nПользователь: %d\nКарта: %s\nСумма: %.8f BTC (%.2f ₽)\nАдрес: %s\nTXID: %s",
			testNote,
			user.TelegramID,
			user.CardNumber,
			tx.AmountBTC,
			rub,
			tx.Address,
			tx.TxID,
		),
	)
	adminMsg.ParseMode = "Markdown"
	b.API.Send(adminMsg)

	if !isTest {
		userMsg := tgbotapi.NewMessage(
			user.TelegramID,
			fmt.Sprintf("💸 Получено %.8f BTC\n\nTransaction ID: %s", tx.AmountBTC, tx.TxID),
		)
		b.API.Send(userMsg)
	}
}

func getBTCRUBRate() (float64, error) {
	resp, err := http.Get("https://api.binance.com/api/v3/ticker/price?symbol=BTCRUB")
	if err != nil {
		return 0, fmt.Errorf("failed to get BTC/RUB rate: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("bad response from Binance: %s", string(body))
	}

	var data struct {
		Symbol string `json:"symbol"`
		Price  string `json:"price"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return 0, fmt.Errorf("failed to parse Binance response: %v", err)
	}

	rate, err := strconv.ParseFloat(data.Price, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid price format: %v", err)
	}

	return rate, nil
}
