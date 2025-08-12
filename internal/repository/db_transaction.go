package repository

import (
	"context"

	"gorm.io/gorm"
)

func (r *Repository) BeginTransaction(ctx context.Context) (*gorm.DB, error) {
	r.logger.Info("Starting transaction...")
	tx := r.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		r.logger.Errorf("Failed to start transaction: %v", tx.Error)
		return nil, tx.Error
	}
	return tx, nil
}

func (r *Repository) Commit(tx *gorm.DB) error {
	r.logger.Info("Committing transaction...")
	if err := tx.Commit().Error; err != nil {
		r.logger.Errorf("Failed to commit transaction: %v", err)
		return err
	}
	return nil
}

func (r *Repository) Rollback(tx *gorm.DB) {
	r.logger.Warn("Rolling back transaction...")
	_ = tx.Rollback().Error
}

func (r *Repository) WithTransaction(tx *gorm.DB) *gorm.DB {
	return tx
}
