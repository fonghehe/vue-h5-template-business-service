package database

import (
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
	{
		version: "2026092301_extend_product_catalogue",
		run: func(db *gorm.DB) error {
			if err := db.AutoMigrate(&model.Product{}, &model.ProductSKU{}, &model.Inventory{}); err != nil {
				return err
			}
			// Existing products keep their public contract while gaining canonical
			// commerce fields. Checkout still requires an explicit SKU.
			return db.Exec(`UPDATE products
				SET name = CASE WHEN name = '' THEN title ELSE name END,
				    cover = CASE WHEN cover = '' THEN img_url ELSE cover END
				WHERE name = '' OR cover = ''`).Error
		},
	},
	{
		version: "2026092302_create_cart_and_coupon",
		run: func(db *gorm.DB) error {
			return db.AutoMigrate(&model.CartItem{}, &model.Coupon{}, &model.CouponUsage{})
		},
	},
	{
		version: "2026092303_create_orders_and_payments",
		run: func(db *gorm.DB) error {
			return db.AutoMigrate(
				&model.Order{},
				&model.OrderItem{},
				&model.OrderStatusHistory{},
				&model.Payment{},
				&model.PaymentWebhookEvent{},
			)
		},
	},
	{
		version: "2026092304_backfill_legacy_skus",
		run:     backfillLegacySKUInventory,
	},
}

// Migrate applies every pending migration inside its own transaction.
func Migrate(db *gorm.DB) error {
	if err := rejectIncompatibleSchema(db); err != nil {
		return err
	}
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

// rejectIncompatibleSchema prevents GORM from trying to retrofit this service's
// NOT NULL fields onto tables owned by another application. The initial
// migrations require these identity columns even on the oldest supported
// business-service schema; their absence is not a safe in-place upgrade.
func rejectIncompatibleSchema(db *gorm.DB) error {
	for _, signature := range []struct {
		table  string
		column string
	}{
		{table: "users", column: "username"},
		{table: "products", column: "title"},
	} {
		if db.Migrator().HasTable(signature.table) && !db.Migrator().HasColumn(signature.table, signature.column) {
			return fmt.Errorf("DATABASE_URL points to an incompatible database: existing %s table lacks %s; use a dedicated database for this service instead of migrating or deleting the existing tables", signature.table, signature.column)
		}
	}
	return nil
}

func isApplied(db *gorm.DB, version string) (bool, error) {
	var count int64
	if err := db.Model(&schemaMigration{}).Where("version = ?", version).Count(&count).Error; err != nil {
		return false, fmt.Errorf("check migration %s: %w", version, err)
	}
	return count > 0, nil
}
