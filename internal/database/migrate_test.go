package database

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fonghehe/vue-h5-template-business-service/internal/model"
)

func TestMigrationRejectsForeignSchemaWithoutChangingIt(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "foreign.db")
	db, err := Open(Options{Driver: "sqlite", DSN: dsn,
		MaxOpenConns: 1, MaxIdleConns: 1, AutoMigrate: false, Seed: false, Quiet: true})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	require.NoError(t, db.Exec("CREATE TABLE users (id integer PRIMARY KEY, name text NOT NULL)").Error)
	require.NoError(t, db.Exec("INSERT INTO users (id, name) VALUES (1, 'existing user')").Error)
	require.NoError(t, db.Exec("CREATE TABLE schema_migrations (version text PRIMARY KEY, applied_at integer)").Error)
	require.NoError(t, db.Exec("INSERT INTO schema_migrations (version) VALUES ('other_app_initial')").Error)

	err = Migrate(db)
	require.ErrorContains(t, err, "DATABASE_URL points to an incompatible database")
	require.ErrorContains(t, err, "users table lacks username")
	_, err = Open(Options{Driver: "sqlite", DSN: dsn, MaxOpenConns: 1,
		MaxIdleConns: 1, AutoMigrate: false, Seed: false, Quiet: true})
	require.ErrorContains(t, err, "DATABASE_URL points to an incompatible database",
		"disabling migrations must not allow the service to use a foreign schema")
	assert.False(t, db.Migrator().HasColumn("users", "username"))
	var userCount int64
	require.NoError(t, db.Table("users").Count(&userCount).Error)
	assert.Equal(t, int64(1), userCount)
	var versions []string
	require.NoError(t, db.Table("schema_migrations").Order("version").Pluck("version", &versions).Error)
	assert.Equal(t, []string{"other_app_initial"}, versions)
}

func TestMigrationBackfillsExistingProductsWithoutSeed(t *testing.T) {
	db, err := Open(Options{Driver: "sqlite", DSN: filepath.Join(t.TempDir(), "migration.db"),
		MaxOpenConns: 1, MaxIdleConns: 1, AutoMigrate: true, Seed: false, Quiet: true})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	product := model.Product{Name: "Legacy", Title: "Legacy", Price: "19.90", Stock: 7,
		Status: model.StatusOnSale}
	require.NoError(t, db.Create(&product).Error)
	// Simulate an existing installation before the data-backfill migration.
	require.NoError(t, db.Where("version = ?", "2026092304_backfill_legacy_skus").
		Delete(&schemaMigration{}).Error)
	require.NoError(t, Migrate(db))
	require.NoError(t, Migrate(db), "re-running migrations must be safe")
	var skus []model.ProductSKU
	require.NoError(t, db.Where("product_id = ?", product.ID).Find(&skus).Error)
	require.Len(t, skus, 1)
	assert.Equal(t, int64(1990), skus[0].Price)
	var inventory model.Inventory
	require.NoError(t, db.First(&inventory, "sku_id = ?", skus[0].ID).Error)
	assert.Equal(t, int64(7), inventory.Available)
	var coupons int64
	require.NoError(t, db.Model(&model.Coupon{}).Count(&coupons).Error)
	assert.Zero(t, coupons, "a production migration must not seed demo coupons")
}
