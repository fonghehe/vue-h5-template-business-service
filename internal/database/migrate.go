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
	version       string
	legacyVersion string // Previous date-based ID, accepted for existing databases.
	run           func(*gorm.DB) error
}

// migrations is append-only. Never edit an entry that has shipped: add a new
// migration instead, so existing databases converge on the same schema.
var migrations = []migration{
	{
		version:       "001_create_users",
		legacyVersion: "2026090101_create_users",
		run: func(db *gorm.DB) error {
			return db.AutoMigrate(&model.User{})
		},
	},
	{
		version:       "002_create_products",
		legacyVersion: "2026090102_create_products",
		run: func(db *gorm.DB) error {
			return db.AutoMigrate(&model.Product{})
		},
	},
	{
		version:       "003_create_favorites",
		legacyVersion: "2026090103_create_favorites",
		run: func(db *gorm.DB) error {
			return db.AutoMigrate(&model.Favorite{})
		},
	},
	{
		version:       "004_extend_product_catalogue",
		legacyVersion: "2026092301_extend_product_catalogue",
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
		version:       "005_create_cart_and_coupon",
		legacyVersion: "2026092302_create_cart_and_coupon",
		run: func(db *gorm.DB) error {
			return db.AutoMigrate(&model.CartItem{}, &model.Coupon{}, &model.CouponUsage{})
		},
	},
	{
		version:       "006_create_orders_and_payments",
		legacyVersion: "2026092303_create_orders_and_payments",
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
		version:       "007_backfill_legacy_skus",
		legacyVersion: "2026092304_backfill_legacy_skus",
		run:           backfillLegacySKUInventory,
	},
	{
		version:       "008_create_refresh_sessions",
		legacyVersion: "2026100201_create_refresh_sessions",
		run:           func(db *gorm.DB) error { return db.AutoMigrate(&model.RefreshSession{}) },
	},
	{
		version:       "009_enforce_commerce_constraints",
		legacyVersion: "2026100202_enforce_commerce_constraints",
		run: func(db *gorm.DB) error {
			return db.AutoMigrate(&model.ProductSKU{}, &model.Inventory{}, &model.CartItem{}, &model.Coupon{}, &model.CouponUsage{}, &model.Order{}, &model.OrderItem{}, &model.Payment{})
		},
	},
}

// Migrate applies the complete pending batch in one transaction.
func Migrate(db *gorm.DB) error {
	// Keep the lock and all migration work on the same transaction/connection.
	// The schema-specific key allows isolated test schemas to migrate in parallel.
	return db.Transaction(func(tx *gorm.DB) error {
		if tx.Name() == "postgres" {
			if err := tx.Exec("SELECT pg_advisory_xact_lock(hashtextextended(current_database() || ':' || current_schema() || ':business-migrations', 0))").Error; err != nil {
				return fmt.Errorf("lock migrations: %w", err)
			}
		}
		return migrate(tx)
	})
}

func migrate(db *gorm.DB) error {
	if err := rejectIncompatibleSchema(db); err != nil {
		return err
	}
	if err := db.AutoMigrate(&schemaMigration{}); err != nil {
		return fmt.Errorf("create migration table: %w", err)
	}
	for _, item := range migrations {
		applied, err := isApplied(db, item)
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

// MigrationStatus is the operator-facing schema version ledger.
type MigrationStatus struct {
	Version string
	Applied bool
}

func Status(db *gorm.DB) ([]MigrationStatus, error) {
	result := make([]MigrationStatus, 0, len(migrations))
	hasLedger := db.Migrator().HasTable(&schemaMigration{})
	for _, item := range migrations {
		applied := false
		if hasLedger {
			var err error
			applied, err = isApplied(db, item)
			if err != nil {
				return nil, err
			}
		}
		result = append(result, MigrationStatus{Version: item.version, Applied: applied})
	}
	return result, nil
}

// CheckSchema prevents a healthy-looking server from booting against an empty
// or outdated database when automatic migrations are disabled.
func CheckSchema(db *gorm.DB) error {
	status, err := Status(db)
	if err != nil {
		return err
	}
	for _, item := range status {
		if !item.Applied {
			return fmt.Errorf("pending migration %s: run make migrate before starting the server", item.Version)
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

func isApplied(db *gorm.DB, item migration) (bool, error) {
	versions := []string{item.version}
	if item.legacyVersion != "" {
		versions = append(versions, item.legacyVersion)
	}
	var count int64
	if err := db.Model(&schemaMigration{}).Where("version IN ?", versions).Count(&count).Error; err != nil {
		return false, fmt.Errorf("check migration %s: %w", item.version, err)
	}
	return count > 0, nil
}
