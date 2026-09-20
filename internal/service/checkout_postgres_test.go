package service

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/fonghehe/vue-h5-template-business-service/internal/apierr"
	"github.com/fonghehe/vue-h5-template-business-service/internal/config"
	"github.com/fonghehe/vue-h5-template-business-service/internal/database"
	"github.com/fonghehe/vue-h5-template-business-service/internal/model"
	"github.com/fonghehe/vue-h5-template-business-service/internal/repository"
)

func newPostgresCheckoutFixture(t *testing.T) commerceFixture {
	t.Helper()
	db := newPostgresCommerceDB(t)
	user := model.User{Username: "checkout-user", PasswordHash: "unused", Roles: model.Roles{model.RoleUser}, Status: "active"}
	product := model.Product{Name: "Checkout", Title: "Checkout", Price: "1", VipPrice: "1", Status: model.StatusOnSale}
	require.NoError(t, db.Create(&user).Error)
	require.NoError(t, db.Create(&product).Error)
	sku := model.ProductSKU{ProductID: product.ID, SKUCode: "CHECKOUT", Name: "Default", Attributes: model.SKUAttributes{},
		Price: 100, OriginalPrice: 100, Status: model.SKUStatusActive}
	repo := repository.NewCommerceRepository(db)
	require.NoError(t, repo.CreateSKUWithInventory(context.Background(), &sku, 100))
	services := New(config.Load(), db, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil)
	return commerceFixture{db: db, repo: repo, services: services, userID: user.ID, skuID: sku.ID}
}

func TestPostgresPaymentRechecksExpiryAfterLockWait(t *testing.T) {
	fixture := newPostgresCheckoutFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err := fixture.services.Cart.Add(ctx, fixture.userID, fixture.skuID, 1)
	require.NoError(t, err)
	order, _, err := fixture.services.Orders.Create(ctx, fixture.userID, "payment-lock-wait", CreateOrderInput{})
	require.NoError(t, err)
	payment, err := fixture.services.Payments.Create(ctx, fixture.userID, order.ID)
	require.NoError(t, err)
	var now atomic.Int64
	now.Store(time.Now().UTC().UnixNano())
	fixture.services.Payments.now = func() time.Time { return time.Unix(0, now.Load()).UTC() }
	provider := fixture.services.Payments.provider.(*MockPaymentProvider)
	callback := PaymentCallback{EventID: "waited-payment", OrderNo: order.OrderNo, Reference: payment.Reference,
		Amount: order.PayableAmount, Status: PaymentCallbackSuccess}
	blocker := fixture.db.Begin()
	require.NoError(t, blocker.Error)
	t.Cleanup(func() { _ = blocker.Rollback().Error })
	var locked model.Order
	require.NoError(t, blocker.Clauses(clause.Locking{Strength: "UPDATE"}).First(&locked, order.ID).Error)
	completed := make(chan error, 1)
	go func() {
		_, _, err := fixture.services.Payments.HandleWebhook(ctx, callback, provider.Sign(callback))
		completed <- err
	}()
	waitForCheckoutLocks(t, fixture.db, 1, nil)
	now.Store(order.ExpiresAt.Add(time.Minute).UnixNano())
	require.NoError(t, blocker.Commit().Error)
	var apiError *apierr.Error
	require.ErrorAs(t, <-completed, &apiError)
	assert.Equal(t, apierr.CodeConflict, apiError.Code)
	stored, err := fixture.services.Orders.Get(ctx, fixture.userID, order.ID)
	require.NoError(t, err)
	assert.Equal(t, model.OrderPendingPayment, stored.Status)
	inventory, err := fixture.repo.Inventory(ctx, fixture.skuID)
	require.NoError(t, err)
	assert.Equal(t, int64(1), inventory.Reserved)
}

func TestPostgresFailedDatabaseInitializationClosesPool(t *testing.T) {
	db := newPostgresCommerceDB(t)
	// This schema belongs exclusively to this test; replace its empty users
	// table with an incompatible signature to exercise initialization failure.
	require.NoError(t, db.Exec("DROP TABLE users").Error)
	require.NoError(t, db.Exec("CREATE TABLE users (id bigint PRIMARY KEY)").Error)
	var schema string
	require.NoError(t, db.Raw("SELECT current_schema()").Scan(&schema).Error)
	parsed, err := url.Parse(os.Getenv("TEST_DATABASE_URL"))
	require.NoError(t, err)
	query := parsed.Query()
	query.Set("search_path", schema)
	query.Set("application_name", schema+"_failed_init")
	parsed.RawQuery = query.Encode()
	_, err = database.Open(database.Options{Driver: "postgres", DSN: parsed.String(), MaxOpenConns: 1,
		MaxIdleConns: 1, AutoMigrate: true, Quiet: true})
	require.ErrorContains(t, err, "incompatible database")
	// Connections can disappear asynchronously in PostgreSQL after Close.
	require.Eventually(t, func() bool {
		var connections int
		queryErr := db.Raw("SELECT count(*) FROM pg_stat_activity WHERE application_name = ?", schema+"_failed_init").Scan(&connections).Error
		return queryErr == nil && connections == 0
	}, time.Second, 10*time.Millisecond)
}

func holdCheckoutSKU(t *testing.T, fixture commerceFixture) *gorm.DB {
	t.Helper()
	tx := fixture.db.Begin()
	require.NoError(t, tx.Error)
	t.Cleanup(func() { _ = tx.Rollback().Error })
	var sku model.ProductSKU
	require.NoError(t, tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&sku, fixture.skuID).Error)
	return tx
}

// Observe real PostgreSQL lock waits rather than coordinating with sleeps or
// assumptions about goroutine scheduling. The fixture has a unique app name.
func waitForCheckoutLocks(t *testing.T, db *gorm.DB, count int, completed <-chan error) (bool, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		var waiting int
		require.NoError(t, db.WithContext(ctx).Raw(`SELECT count(*) FROM pg_stat_activity
			WHERE application_name = current_setting('application_name') AND wait_event_type = 'Lock'`).Scan(&waiting).Error)
		if waiting >= count {
			return false, nil
		}
		select {
		case err := <-completed:
			return true, err
		case <-ctx.Done():
			t.Fatal("checkout did not reach the expected database lock boundary")
		case <-ticker.C:
		}
	}
}

func TestPostgresConcurrentCheckoutsConsumeCartOnce(t *testing.T) {
	for _, sameKey := range []bool{false, true} {
		t.Run(fmt.Sprintf("sameKey=%t", sameKey), func(t *testing.T) {
			fixture := newPostgresCheckoutFixture(t)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			_, err := fixture.services.Cart.Add(ctx, fixture.userID, fixture.skuID, 1)
			require.NoError(t, err)
			blocker := holdCheckoutSKU(t, fixture)
			type result struct {
				order    model.Order
				replayed bool
				err      error
			}
			results := make(chan result, 2)
			for index := range 2 {
				key := fmt.Sprintf("concurrent-%d", index)
				if sameKey {
					key = "shared-checkout"
				}
				go func() {
					order, replayed, err := fixture.services.Orders.Create(ctx, fixture.userID, key, CreateOrderInput{})
					results <- result{order, replayed, err}
				}()
			}
			waitForCheckoutLocks(t, fixture.db, 2, nil)
			require.NoError(t, blocker.Commit().Error)
			created, replayed, rejected := 0, 0, 0
			for range 2 {
				result := <-results
				if result.err != nil {
					var apiError *apierr.Error
					require.ErrorAs(t, result.err, &apiError)
					assert.Equal(t, apierr.CodeValidation, apiError.Code)
					rejected++
				} else if result.replayed {
					replayed++
				} else {
					created++
				}
			}
			assert.Equal(t, 1, created)
			if sameKey {
				assert.Equal(t, 1, replayed)
				assert.Zero(t, rejected)
			} else {
				assert.Equal(t, 1, rejected)
			}
			var count int64
			require.NoError(t, fixture.db.Model(&model.Order{}).Count(&count).Error)
			assert.Equal(t, int64(1), count)
			inventory, err := fixture.repo.Inventory(ctx, fixture.skuID)
			require.NoError(t, err)
			assert.Equal(t, int64(1), inventory.Reserved)
		})
	}
}

func TestPostgresCartAddDuringCheckoutSurvives(t *testing.T) {
	fixture := newPostgresCheckoutFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err := fixture.services.Cart.Add(ctx, fixture.userID, fixture.skuID, 1)
	require.NoError(t, err)
	var first model.ProductSKU
	require.NoError(t, fixture.db.First(&first, fixture.skuID).Error)
	second := first
	second.ID, second.SKUCode = 0, "ADDED-DURING-CHECKOUT"
	require.NoError(t, fixture.repo.CreateSKUWithInventory(ctx, &second, 10))
	blocker := holdCheckoutSKU(t, fixture)
	checkoutDone := make(chan error, 1)
	go func() {
		_, _, err := fixture.services.Orders.Create(ctx, fixture.userID, "checkout-and-add", CreateOrderInput{})
		checkoutDone <- err
	}()
	waitForCheckoutLocks(t, fixture.db, 1, nil)
	addDone := make(chan error, 1)
	go func() {
		_, err := fixture.services.Cart.Add(ctx, fixture.userID, second.ID, 1)
		addDone <- err
	}()
	completed, addErr := waitForCheckoutLocks(t, fixture.db, 2, addDone)
	require.NoError(t, blocker.Commit().Error)
	require.NoError(t, <-checkoutDone)
	if !completed {
		addErr = <-addDone
	}
	require.NoError(t, addErr)
	cart, err := fixture.services.Cart.List(ctx, fixture.userID)
	require.NoError(t, err)
	require.Len(t, cart, 1)
	assert.Equal(t, second.ID, cart[0].SKUID)
}
