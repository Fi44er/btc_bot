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

func (s *Service) HandleCheckTransactions(ctx context.Context, userID int64, notifyCallback models.NotifyCallback) (float64, error) {
	s.logger.Infof("SERVICE: Starting HandleCheckTransactions for user %d", userID)
	user, err := s.GetUser(ctx, userID)
	if err != nil || user == nil || user.SystemWallet == nil || user.SystemWallet.Address == "" {
		s.logger.Errorf("Cannot check transactions for user %d: user or wallet not found", userID)
		return 0, fmt.Errorf("у вас нет активного адреса для проверки")
	}

	newTransactions, err := s.checkUserTransactions(ctx, user, notifyCallback)
	if err != nil {
		s.logger.Errorf("Error checking transactions for user %d: %v", userID, err)
		return 0, fmt.Errorf("произошла ошибка при проверке транзакций")
	}

	if len(newTransactions) == 0 {
		return 0, nil
	}

	totalBTC := 0.0
	for _, tx := range newTransactions {
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
		rate = 3900027.0 // Лучше вынести в конфиг
	}

	totalRUB := totalBTC * rate

	currentUser, err := s.GetUser(ctx, userID)
	if err != nil {
		return 0, fmt.Errorf("не удалось получить пользователя перед обновлением баланса: %v", err)
	}

	currentUser.Balance += totalRUB

	if err := s.repo.UpdateUser(ctx, currentUser, nil); err != nil {
		return 0, fmt.Errorf("не удалось обновить баланс: %v", err)
	}

	s.logger.Infof("Successfully added %.2f RUB to user %d balance.", totalRUB, userID)
	return totalRUB, nil
}

func (s *Service) checkUserTransactions(ctx context.Context, user *models.User, notifyCallback models.NotifyCallback) ([]models.Transaction, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	url := fmt.Sprintf("%s/address/%s/txs", mainnetAPIURL, user.SystemWallet.Address)

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
		} `json:"status"`
	}

	if err := json.Unmarshal(body, &apiResponse); err != nil {
		s.logger.Errorf("Failed to decode JSON: %v\nRaw response: %s", err, string(body))
		return nil, fmt.Errorf("invalid API response format")
	}

	var processedNewTransactions []models.Transaction

	for _, txData := range apiResponse {
		for _, output := range txData.Vout {
			if output.Address == user.SystemWallet.Address {
				amountBTC := float64(output.Value) / 1e8

				isNew, err := s.processTransaction(ctx, txData.TxID, user.TelegramID, user.SystemWallet.Address, amountBTC, txData.Status.Confirmed)
				if err != nil {
					s.logger.Errorf("Transaction processing failed: %v", err)
					continue
				}

				if isNew && txData.Status.Confirmed {
					newTxModel := models.Transaction{
						TxID:      txData.TxID,
						UserID:    user.TelegramID,
						Address:   user.SystemWallet.Address,
						AmountBTC: amountBTC,
						Confirmed: true,
					}

					if notifyCallback != nil {
						s.logger.Infof("SERVICE: New confirmed transaction %s found for user %d. CALLING NOTIFY CALLBACK.", newTxModel.TxID, user.TelegramID)
						notifyCallback(user, &newTxModel)
					} else {
						s.logger.Error("SERVICE: NOTIFY CALLBACK IS NIL! Cannot notify bot.")
					}

					processedNewTransactions = append(processedNewTransactions, newTxModel)
				}
			}
		}
	}

	return processedNewTransactions, nil
}

func (s *Service) processTransaction(ctx context.Context, txID string, userID int64, address string, amountBTC float64, confirmed bool) (bool, error) {
	if amountBTC <= 0 {
		return false, nil
	}

	existingTx, err := s.repo.GetTransaction(ctx, txID)
	if err != nil {
		return false, fmt.Errorf("failed to check existing transaction: %w", err)
	}
	if existingTx != nil {
		return false, nil
	}

	tx := &models.Transaction{
		TxID:      txID,
		UserID:    userID,
		Address:   address,
		AmountBTC: amountBTC,
		Confirmed: confirmed,
	}

	if err := s.repo.CreateOrUpdateTransaction(ctx, tx); err != nil {
		return false, fmt.Errorf("failed to save transaction: %v", err)
	}

	return true, nil
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
