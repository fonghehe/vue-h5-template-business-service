package service

import (
	"context"
	"fmt"
	"io"
	"log/slog"
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

	"github.com/fonghehe/vue-h5-template-business-service/internal/config"
	"github.com/fonghehe/vue-h5-template-business-service/internal/database"
	"github.com/fonghehe/vue-h5-template-business-service/internal/model"
	"github.com/fonghehe/vue-h5-template-business-service/internal/repository"
)

// This runs in CI against PostgreSQL. Separate users/carts exercise the entire
// order transaction rather than only the inventory repository primitive.
func TestPostgresConcurrentOrdersNeverOversell(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not configured")
	}
	base, err := database.Open(database.Options{Driver: "postgres", DSN: dsn,
		MaxOpenConns: 5, MaxIdleConns: 2, Quiet: true})
	require.NoError(t, err)
	baseSQL, err := base.DB()
	require.NoError(t, err)
	schema := "orders_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
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
	db, err := database.Open(database.Options{Driver: "postgres", DSN: parsed.String(),
		MaxOpenConns: 30, MaxIdleConns: 20, ConnMaxLifetime: time.Minute, AutoMigrate: true, Quiet: true})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	product := model.Product{Name: "Concurrent order", Title: "Concurrent order", Price: "1.00",
		VipPrice: "1.00", Status: model.StatusOnSale, SearchText: "concurrent order"}
	require.NoError(t, db.Create(&product).Error)
	sku := model.ProductSKU{ProductID: product.ID, SKUCode: "ORDER-CONCURRENT-1", Name: "Default",
		Attributes: model.SKUAttributes{}, Price: 100, OriginalPrice: 100, Status: model.SKUStatusActive}
	require.NoError(t, db.Create(&sku).Error)
	require.NoError(t, db.Create(&model.Inventory{SKUID: sku.ID, Available: 10, Version: 1}).Error)
	users := make([]model.User, 100)
	for i := range users {
		users[i] = model.User{Username: fmt.Sprintf("buyer-%d", i), PasswordHash: "not-used",
			Roles: model.Roles{model.RoleUser}, Status: "active"}
	}
	require.NoError(t, db.CreateInBatches(&users, 50).Error)
	for _, user := range users {
		require.NoError(t, db.Create(&model.CartItem{UserID: user.ID, SKUID: sku.ID, Quantity: 1}).Error)
	}

	repo := repository.NewCommerceRepository(db)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	orders := NewOrderService(config.Config{OrderPaymentTTL: 15 * time.Minute}, repo, logger, nil)
	var succeeded, rejected, unexpected atomic.Int64
	start := make(chan struct{})
	var group sync.WaitGroup
	for _, user := range users {
		group.Add(1)
		go func(userID uint) {
			defer group.Done()
			<-start
			_, _, createErr := orders.Create(context.Background(), userID, fmt.Sprintf("checkout-%d", userID), CreateOrderInput{})
			switch {
			case createErr == nil:
				succeeded.Add(1)
			case repository.IsConflict(createErr):
				rejected.Add(1)
			default:
				unexpected.Add(1)
			}
		}(user.ID)
	}
	close(start)
	group.Wait()
	assert.Equal(t, int64(10), succeeded.Load())
	assert.Equal(t, int64(90), rejected.Load())
	assert.Zero(t, unexpected.Load())
	var orderCount int64
	require.NoError(t, db.Model(&model.Order{}).Count(&orderCount).Error)
	assert.Equal(t, int64(10), orderCount)
	inventory, err := repo.Inventory(context.Background(), sku.ID)
	require.NoError(t, err)
	assert.Zero(t, inventory.Available)
	assert.Equal(t, int64(10), inventory.Reserved)
}
