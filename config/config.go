package config

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/viper"
)

type Config struct {
	TelegramBotToken string `mapstructure:"TELEGRAM_BOT_TOKEN"`
	AdminChatID      int64  `mapstructure:"ADMIN_CHAT_ID"`
	MasterKeySeed    string `mapstructure:"MASTER_KEY_SEED"`
	DB_URL           string `mapstructure:"DB_URL"`
}

func LoadConfig(path string) (config Config, err error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return config, fmt.Errorf("ошибка получения абсолютного пути: %w", err)
	}

	viper.AddConfigPath(filepath.Dir(absPath))
	viper.SetConfigName(filepath.Base(absPath))
	viper.SetConfigType("env")
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		return config, fmt.Errorf("ошибка чтения конфигурации: %w", err)
	}

	if err := viper.Unmarshal(&config); err != nil {
		return config, fmt.Errorf("ошибка преобразования конфига: %w", err)
	}

	return config, nil
}
