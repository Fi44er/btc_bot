package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/Fi44er/btc_bot/internal/models"
)

var (
	testnetAPIURL = "https://mempool.space/testnet4/api"
	mainnetAPIURL = "https://mempool.space/api"
)

type processTransaction struct {
	ctx       context.Context
	txID      string
	address   string
	amountBTC float64
	confirmed bool
}

func (s *Service) HandleCheckTransactions(ctx context.Context, userID int64, _ models.NotifyCallback) (float64, error) {
	user, err := s.GetUser(ctx, userID)
	if err != nil || user == nil || user.SystemWallet == nil {
		s.logger.Errorf("Error checking transactions: %v", err)
		return 0, fmt.Errorf("У вас нет активного адреса для проверки.")
	}

	transactions, err := s.checkUserTransactions(ctx, user.SystemWallet.Address)
	if err != nil {
		s.logger.Errorf("Error checking transactions: %v", err)
		return 0, fmt.Errorf("Произошла ошибка при проверке транзакций.")
	}

	if len(transactions) == 0 {
		return 0, nil
	}

	// Конвертируем BTC → RUB и пополняем баланс только за новые транзакции
	totalBTC := 0.0
	for _, tx := range transactions {
		if tx.Confirmed {
			totalBTC += tx.AmountBTC
		}
	}

	if totalBTC == 0 {
		return 0, nil
	}

	rate, err := s.GetBTCRUBRate()
	if err != nil {
		s.logger.Warnf("Failed to get BTC/RUB rate, using fallback: %v", err)
		rate = 3900027.0
	}

	s.logger.Warnf("RATE: %v", rate)
	s.logger.Warnf("totalBTC: %v", totalBTC)
	s.logger.Warnf("total rub: %v", totalBTC*rate)

	totalRUB := totalBTC * rate
	userModel := models.User{
		TelegramID:     user.TelegramID,
		CardNumber:     user.CardNumber,
		Balance:        user.Balance + totalRUB,
		SystemWalletID: user.SystemWalletID,
	}
	if err := s.repo.UpdateUser(ctx, &userModel, nil); err != nil {
		return 0, fmt.Errorf("Не удалось обновить баланс: %v", err)
	}

	return totalRUB, nil
}

func (s *Service) checkUserTransactions(ctx context.Context, address string) ([]models.Transaction, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	url := fmt.Sprintf("%s/address/%s/txs", testnetAPIURL, address)

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
		s.logger.Errorf("Failed to decode JSON: %v\nRaw response: %s", err, string(body))
		return nil, fmt.Errorf("invalid API response format")
	}

	var newTransactions []models.Transaction

	for _, tx := range apiResponse {
		for _, output := range tx.Vout {
			if output.Address == address {
				amountBTC := float64(output.Value) / 1e8

				isNew, err := s.processTransaction(ctx, tx.TxID, address, amountBTC, tx.Status.Confirmed)
				if err != nil {
					s.logger.Errorf("Transaction processing failed: %v", err)
					continue
				}

				// Добавляем только новые транзакции
				if isNew {
					newTransactions = append(newTransactions, models.Transaction{
						TxID:      tx.TxID,
						Address:   address,
						AmountBTC: amountBTC,
						Confirmed: tx.Status.Confirmed,
					})
				}
			}
		}
	}

	return newTransactions, nil
}

func (s *Service) processTransaction(ctx context.Context, txID, address string, amountBTC float64, confirmed bool) (bool, error) {
	if amountBTC <= 0 {
		return false, fmt.Errorf("invalid amount: %.8f BTC", amountBTC)
	}

	existingTx, err := s.repo.GetTransaction(ctx, txID)
	if err != nil {
		return false, fmt.Errorf("failed to check existing transaction: %w", err)
	}
	if existingTx != nil {
		return false, nil // уже есть, пропускаем
	}

	user, err := s.GetUserByAddress(ctx, address)
	if err != nil {
		return false, fmt.Errorf("failed to get user: %v", err)
	}

	tx := &models.Transaction{
		TxID:      txID,
		UserID:    user.TelegramID,
		Address:   address,
		AmountBTC: amountBTC,
		Confirmed: confirmed,
	}

	if err := s.CreateOrUpdateTransaction(ctx, tx); err != nil {
		return false, fmt.Errorf("failed to save transaction: %v", err)
	}

	return true, nil // новая транзакция
}

func (s *Service) GetBTCRUBRate() (float64, error) {
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
