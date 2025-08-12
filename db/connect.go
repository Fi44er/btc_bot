package db

import (
	"time"

	"github.com/Fi44er/btc_bot/internal/models"
	"github.com/Fi44er/btc_bot/utils"
	"gorm.io/driver/postgres"

	"gorm.io/gorm"
	gormLogger "gorm.io/gorm/logger"
)

func ConnectDb(url string, log *utils.Logger) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.New(postgres.Config{
		DSN:                  url,
		PreferSimpleProtocol: true,
	}), &gorm.Config{
		Logger: gormLogger.Default.LogMode(gormLogger.Error),
	})

	if err != nil {
		return nil, err
	}

	log.Info("✅ Database connection successfully")

	log.Info("📦 Setting database connection pool...")
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	sqlDB.SetMaxIdleConns(20)
	sqlDB.SetMaxOpenConns(200)
	sqlDB.SetConnMaxLifetime(time.Hour)

	return db, nil
}

func Migrate(db *gorm.DB, trigger bool, log *utils.Logger) error {

	if trigger {
		log.Info("📦 Migrating database...")
		models := []interface{}{
			&models.SystemWallet{},
			&models.User{},
			&models.Transaction{},
			&models.Withdrawal{},
		}

		log.Info("📦 Creating types...")

		if err := db.AutoMigrate(models...); err != nil {
			log.Errorf("✖ Failed to migrate database: %v", err)
			return err
		}
	}

	log.Info("✅ Database connection successfully")
	return nil
}
