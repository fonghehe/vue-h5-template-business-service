package service

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/fonghehe/vue-h5-template-business-service/internal/apierr"
	"github.com/fonghehe/vue-h5-template-business-service/internal/config"
	"github.com/fonghehe/vue-h5-template-business-service/internal/database"
	"github.com/fonghehe/vue-h5-template-business-service/internal/model"
	"github.com/fonghehe/vue-h5-template-business-service/internal/repository"
)

type commerceFixture struct {
	db        *gorm.DB
	repo      *repository.CommerceRepository
	services  *Container
	userID    uint
	skuID     uint
	inventory model.Inventory
}

func newCommerceFixture(t *testing.T) commerceFixture {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?_journal_mode=WAL&_busy_timeout=30000", filepath.Join(t.TempDir(), "commerce.db"))
	db, err := database.Open(database.Options{
		Driver: "sqlite", DSN: dsn, MaxOpenConns: 10, MaxIdleConns: 10, AutoMigrate: true, Seed: true, Quiet: true,
	})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	cfg := config.Load()
	cfg.AppEnv = config.EnvTest
	cfg.DatabaseDriver = "sqlite"
	cfg.DatabaseURL = dsn
	cfg.JWTSecret = "commerce-test-secret-that-is-long-enough"
	cfg.RateLimitEnabled = false
	cfg.OrderPaymentTTL = 15 * time.Minute
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	container := New(cfg, db, logger, nil, nil)
	repo := repository.NewCommerceRepository(db)

	var user model.User
	require.NoError(t, db.Where("username = ?", "user").First(&user).Error)
	var sku model.ProductSKU
	require.NoError(t, db.Order("id ASC").First(&sku).Error)
	inventory, err := repo.Inventory(context.Background(), sku.ID)
	require.NoError(t, err)
	return commerceFixture{db: db, repo: repo, services: container, userID: user.ID, skuID: sku.ID, inventory: inventory}
}

func TestOrderServiceCreatesSnapshotsAndReservesInventory(t *testing.T) {
	fixture := newCommerceFixture(t)
	ctx := context.Background()
	_, err := fixture.services.Cart.Add(ctx, fixture.userID, fixture.skuID, 2)
	require.NoError(t, err)

	order, replayed, err := fixture.services.Orders.Create(ctx, fixture.userID, "checkout-001", CreateOrderInput{})

	require.NoError(t, err)
	assert.False(t, replayed)
	assert.Equal(t, model.OrderPendingPayment, order.Status)
	require.Len(t, order.Items, 1)
	assert.NotEmpty(t, order.Items[0].ProductName)
	assert.NotEmpty(t, order.Items[0].SKUName)
	assert.Positive(t, order.Items[0].UnitPrice)
	assert.Equal(t, order.OriginalAmount, order.PayableAmount)
	assert.Equal(t, int64(0), order.DiscountAmount)

	remaining, err := fixture.repo.Inventory(ctx, fixture.skuID)
	require.NoError(t, err)
	assert.Equal(t, fixture.inventory.Available-2, remaining.Available)
	assert.Equal(t, int64(2), remaining.Reserved)
	cart, err := fixture.services.Cart.List(ctx, fixture.userID)
	require.NoError(t, err)
	assert.Empty(t, cart)
}

func TestOrderServiceRollsBackWhenInventoryCannotBeReserved(t *testing.T) {
	fixture := newCommerceFixture(t)
	ctx := context.Background()
	require.NoError(t, fixture.db.Model(&model.Inventory{}).Where("sku_id = ?", fixture.skuID).
		Updates(map[string]any{"available": 1, "reserved": 0}).Error)
	_, err := fixture.services.Cart.Add(ctx, fixture.userID, fixture.skuID, 2)
	require.NoError(t, err)

	_, _, err = fixture.services.Orders.Create(ctx, fixture.userID, "checkout-no-stock", CreateOrderInput{})

	var apiError *apierr.Error
	require.ErrorAs(t, err, &apiError)
	assert.Equal(t, apierr.CodeConflict, apiError.Code)
	var orders int64
	require.NoError(t, fixture.db.Model(&model.Order{}).Count(&orders).Error)
	assert.Zero(t, orders)
	inventory, inventoryErr := fixture.repo.Inventory(ctx, fixture.skuID)
	require.NoError(t, inventoryErr)
	assert.Equal(t, int64(1), inventory.Available)
	assert.Equal(t, int64(0), inventory.Reserved)
	cart, cartErr := fixture.services.Cart.List(ctx, fixture.userID)
	require.NoError(t, cartErr)
	assert.Len(t, cart, 1, "cart clearing must roll back with the order")
}

func TestIdempotencyReturnsOriginalOrderWithoutDoubleReservation(t *testing.T) {
	fixture := newCommerceFixture(t)
	ctx := context.Background()
	_, err := fixture.services.Cart.Add(ctx, fixture.userID, fixture.skuID, 1)
	require.NoError(t, err)
	first, replayed, err := fixture.services.Orders.Create(ctx, fixture.userID, "same-network-request", CreateOrderInput{})
	require.NoError(t, err)
	assert.False(t, replayed)

	second, replayed, err := fixture.services.Orders.Create(ctx, fixture.userID, "same-network-request", CreateOrderInput{})

	require.NoError(t, err)
	assert.True(t, replayed)
	assert.Equal(t, first.ID, second.ID)
	var count int64
	require.NoError(t, fixture.db.Model(&model.Order{}).Count(&count).Error)
	assert.Equal(t, int64(1), count)
	inventory, err := fixture.repo.Inventory(ctx, fixture.skuID)
	require.NoError(t, err)
	assert.Equal(t, int64(1), inventory.Reserved)

	_, _, err = fixture.services.Orders.Create(ctx, fixture.userID, "same-network-request", CreateOrderInput{CouponCode: "WELCOME10"})
	require.Error(t, err, "the same key with a different request must conflict")
}

func TestCouponRules(t *testing.T) {
	now := time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC)
	base := model.Coupon{
		Type: model.PromotionThresholdDiscount, Value: 2000, MinimumAmount: 10000,
		StartAt: now.Add(-time.Hour), EndAt: now.Add(time.Hour), UsageLimit: 10, PerUserLimit: 1,
	}

	discount, err := CalculateCouponDiscount(base, 15000, 7, 0, 0, now)
	require.NoError(t, err)
	assert.Equal(t, int64(2000), discount)
	fixed := base
	fixed.Type = model.PromotionFixedDiscount
	fixed.MinimumAmount = 0
	fixed.Value = 500
	discount, err = CalculateCouponDiscount(fixed, 1200, 7, 0, 0, now)
	require.NoError(t, err)
	assert.Equal(t, int64(500), discount)
	percentage := base
	percentage.Type = model.PromotionPercentage
	percentage.MinimumAmount = 0
	percentage.Value = 1500
	discount, err = CalculateCouponDiscount(percentage, 12345, 7, 0, 0, now)
	require.NoError(t, err)
	assert.Equal(t, int64(1851), discount)

	expired := base
	expired.EndAt = now
	_, err = CalculateCouponDiscount(expired, 15000, 7, 0, 0, now)
	require.Error(t, err)
	_, err = CalculateCouponDiscount(base, 9999, 7, 0, 0, now)
	require.Error(t, err)
	_, err = CalculateCouponDiscount(base, 15000, 7, 10, 0, now)
	require.Error(t, err)
	_, err = CalculateCouponDiscount(base, 15000, 7, 0, 1, now)
	require.Error(t, err)

	owner := uint(8)
	base.UserID = &owner
	_, err = CalculateCouponDiscount(base, 15000, 7, 0, 0, now)
	require.Error(t, err)
}

func TestOrderExpirationReleasesInventoryAndCoupon(t *testing.T) {
	fixture := newCommerceFixture(t)
	ctx := context.Background()
	baseTime := time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC)
	fixture.services.Orders.now = func() time.Time { return baseTime }
	fixture.services.Orders.config.OrderPaymentTTL = time.Minute
	_, err := fixture.services.Cart.Add(ctx, fixture.userID, fixture.skuID, 3)
	require.NoError(t, err)
	order, _, err := fixture.services.Orders.Create(ctx, fixture.userID, "expiring-order", CreateOrderInput{CouponCode: "WELCOME10"})
	require.NoError(t, err)
	fixture.services.Orders.now = func() time.Time { return baseTime.Add(2 * time.Minute) }

	expired, err := fixture.services.Orders.ExpireBatch(ctx)

	require.NoError(t, err)
	assert.Equal(t, 1, expired)
	order, err = fixture.services.Orders.Get(ctx, fixture.userID, order.ID)
	require.NoError(t, err)
	assert.Equal(t, model.OrderExpired, order.Status)
	inventory, err := fixture.repo.Inventory(ctx, fixture.skuID)
	require.NoError(t, err)
	assert.Equal(t, fixture.inventory.Available, inventory.Available)
	assert.Zero(t, inventory.Reserved)
	var usage model.CouponUsage
	require.NoError(t, fixture.db.Where("order_id = ?", order.ID).First(&usage).Error)
	assert.Equal(t, model.CouponUsageReleased, usage.Status)
}

func TestCancelRejectsPaidOrForeignOrder(t *testing.T) {
	fixture := newCommerceFixture(t)
	ctx := context.Background()
	_, err := fixture.services.Cart.Add(ctx, fixture.userID, fixture.skuID, 1)
	require.NoError(t, err)
	order, _, err := fixture.services.Orders.Create(ctx, fixture.userID, "cancel-owner", CreateOrderInput{})
	require.NoError(t, err)

	_, err = fixture.services.Orders.Cancel(ctx, fixture.userID+999, order.ID)
	require.Error(t, err)
	_, err = fixture.services.Orders.Cancel(ctx, fixture.userID, order.ID)
	require.NoError(t, err)
	_, err = fixture.services.Orders.Cancel(ctx, fixture.userID, order.ID)
	require.Error(t, err)
}
