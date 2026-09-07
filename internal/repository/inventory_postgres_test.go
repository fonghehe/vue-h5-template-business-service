package repository

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fonghehe/vue-h5-template-business-service/internal/database"
	"github.com/fonghehe/vue-h5-template-business-service/internal/model"
)

// TestPostgresInventoryConcurrencyPreventsOverselling is the authoritative
// concurrency test. It uses an isolated PostgreSQL schema when
// TEST_DATABASE_URL is configured (CI always configures it).
func TestPostgresInventoryConcurrencyPreventsOverselling(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not configured")
	}

	base, err := database.Open(database.Options{
		Driver: "postgres", DSN: dsn, MaxOpenConns: 5, MaxIdleConns: 2, ConnMaxLifetime: time.Minute, Quiet: true,
	})
	require.NoError(t, err)
	baseSQL, err := base.DB()
	require.NoError(t, err)

	schema := "inventory_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	require.NoError(t, base.Exec(fmt.Sprintf(`CREATE SCHEMA "%s"`, schema)).Error)
	t.Cleanup(func() {
		_ = base.Exec(fmt.Sprintf(`DROP SCHEMA IF EXISTS "%s" CASCADE`, schema)).Error
		_ = baseSQL.Close()
	})

	parsed, err := url.Parse(dsn)
	require.NoError(t, err)
	query := parsed.Query()
	query.Set("search_path", schema)
	parsed.RawQuery = query.Encode()

	db, err := database.Open(database.Options{
		Driver: "postgres", DSN: parsed.String(), MaxOpenConns: 30, MaxIdleConns: 20,
		ConnMaxLifetime: time.Minute, AutoMigrate: true, Quiet: true,
	})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	product := model.Product{
		Name: "Concurrency Product", CategoryID: "test", Brand: "test", Cover: "https://example.com/test.png",
		Title: "Concurrency Product", ImgURL: "https://example.com/test.png", Price: "1.00", VipPrice: "1.00",
		ShopDesc: "test", Delivery: "test", ShopName: "test", Description: "test", Status: model.StatusOnSale,
		SearchText: "concurrency product",
	}
	require.NoError(t, db.Create(&product).Error)
	sku := model.ProductSKU{ProductID: product.ID, SKUCode: "CONCURRENCY-1", Name: "Default",
		Attributes: model.SKUAttributes{"variant": "default"}, Price: 100, OriginalPrice: 100, Status: model.SKUStatusActive}
	require.NoError(t, db.Create(&sku).Error)
	require.NoError(t, db.Create(&model.Inventory{SKUID: sku.ID, Available: 10, Reserved: 0, Version: 1}).Error)

	repo := NewCommerceRepository(db)
	var succeeded atomic.Int64
	var rejected atomic.Int64
	var unexpected atomic.Int64
	start := make(chan struct{})
	var group sync.WaitGroup
	for range 100 {
		group.Add(1)
		go func() {
			defer group.Done()
			<-start
			err := repo.ReserveInventory(context.Background(), sku.ID, 1)
			switch {
			case err == nil:
				succeeded.Add(1)
			case IsConflict(err):
				rejected.Add(1)
			default:
				unexpected.Add(1)
			}
		}()
	}
	close(start)
	group.Wait()

	assert.Equal(t, int64(10), succeeded.Load())
	assert.Equal(t, int64(90), rejected.Load())
	assert.Zero(t, unexpected.Load())
	inventory, err := repo.Inventory(context.Background(), sku.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(0), inventory.Available)
	assert.Equal(t, int64(10), inventory.Reserved)
}
