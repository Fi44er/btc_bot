package models

import "time"

type User struct {
	TelegramID     int64     `gorm:"primaryKey" json:"telegram_id"`
	DepositAddress string    `gorm:"unique" json:"deposit_address"`
	CardNumber     string    `json:"card_number"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type Transaction struct {
	TxID      string    `gorm:"primaryKey" json:"tx_id"`
	UserID    int64     `json:"user_id"`
	Address   string    `json:"address"`
	AmountBTC float64   `json:"amount_btc"`
	Confirmed bool      `json:"confirmed"`
	CreatedAt time.Time `json:"created_at"`
}
