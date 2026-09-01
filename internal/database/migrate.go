package database

import (
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/fonghehe/vue-h5-template-business-service/internal/model"
)

// schemaMigration records the migrations already applied. Recording them in the
// database (rather than relying on AutoMigrate alone) lets us run ordered,
// one-off data backfills that AutoMigrate cannot express.
type schemaMigration struct {
	Version   string `gorm:"primaryKey;size:64"`
	AppliedAt int64  `gorm:"autoCreateTime"`
}

// migration is a single ordered schema change.
type migration struct {
	version string
	run     func(*gorm.DB) error
}

// migrations is append-only. Never edit an entry that has shipped: add a new
// migration instead, so existing databases converge on the same schema.
var migrations = []migration{
	{
		version: "2026090101_create_users",
		run: func(db *gorm.DB) error {
			return db.AutoMigrate(&model.User{})
		},
	},
	{
		version: "2026090102_create_products",
		run: func(db *gorm.DB) error {
			return db.AutoMigrate(&model.Product{})
		},
	},
	{
		version: "2026090103_create_favorites",
		run: func(db *gorm.DB) error {
			return db.AutoMigrate(&model.Favorite{})
		},
	},
}

// Migrate applies every pending migration inside its own transaction.
func Migrate(db *gorm.DB) error {
	if err := db.AutoMigrate(&schemaMigration{}); err != nil {
		return fmt.Errorf("create migration table: %w", err)
	}
	for _, item := range migrations {
		applied, err := isApplied(db, item.version)
		if err != nil {
			return err
		}
		if applied {
			continue
		}
		if err := db.Transaction(func(tx *gorm.DB) error {
			if err := item.run(tx); err != nil {
				return err
			}
			return tx.Create(&schemaMigration{Version: item.version}).Error
		}); err != nil {
			return fmt.Errorf("apply migration %s: %w", item.version, err)
		}
	}
	return nil
}

func isApplied(db *gorm.DB, version string) (bool, error) {
	var record schemaMigration
	err := db.Where("version = ?", version).First(&record).Error
	switch {
	case err == nil:
		return true, nil
	case errors.Is(err, gorm.ErrRecordNotFound):
		return false, nil
	default:
		return false, fmt.Errorf("check migration %s: %w", version, err)
	}
}
