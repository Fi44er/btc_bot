package models

type User struct {
	TelegramID int64   `gorm:"primaryKey" json:"telegram_id"`
	CardNumber string  `json:"card_number"`
	Balance    float64 `gorm:"default:0" json:"balance"`

	SystemWalletID *int64        `json:"system_wallet_id" gorm:"index"`
	SystemWallet   *SystemWallet `gorm:"foreignKey:SystemWalletID" json:"system_wallet,omitempty"`

	Transactions []Transaction `gorm:"foreignKey:UserID" json:"transactions"`
	Withdrawal   *Withdrawal   `gorm:"foreignKey:UserID" json:"withdrawal"`
	IsAdmin      bool          `json:"is_admin" gorm:"-"`
}

type Withdrawal struct {
	ID         uint    `gorm:"primaryKey" json:"id"`
	UserID     int64   `json:"user_id" gorm:"uniqueIndex:idx_user_pending"` // уникальный индекс для одной активной заявки
	CardNumber string  `json:"card_number"`
	Amount     float64 `json:"amount"`
	Status     string  `json:"status" gorm:"default:pending"` // pending, completed, rejected
	CreatedAt  string  `json:"created_at"`
}

type Transaction struct {
	TxID      string  `gorm:"primaryKey" json:"tx_id"`
	UserID    int64   `json:"user_id"`
	Address   string  `json:"address"`
	AmountBTC float64 `json:"amount_btc"`
	Confirmed bool    `json:"confirmed"`
}

type SystemWallet struct {
	ID         int64  `gorm:"primaryKey" json:"id"`
	Address    string `gorm:"unique" json:"address"`
	PrivateKey string `json:"-" gorm:"type:varchar(255)"`
}

type NotifyCallback func(*User, *Transaction, bool)
