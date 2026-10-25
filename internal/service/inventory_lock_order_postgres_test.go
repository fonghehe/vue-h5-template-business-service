package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/fonghehe/vue-h5-template-business-service/internal/model"
	"gorm.io/gorm/clause"
)

// Persist snapshots in reverse SKU order to model historical/imported data and
// unordered SQL results. Both services must acquire low SKU before high SKU.
func TestPostgresPaymentAndCancelUseOrderedInventoryLocks(t *testing.T) {
	for _, operation := range []string{"payment", "cancel"} {
		t.Run(operation, func(t *testing.T) {
			fixture := newPostgresCheckoutFixture(t)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			var first model.ProductSKU
			if err := fixture.db.First(&first, fixture.skuID).Error; err != nil {
				t.Fatal(err)
			}
			second := first
			second.ID, second.SKUCode = 0, "SECOND-LOCK-ORDER"
			if err := fixture.repo.CreateSKUWithInventory(ctx, &second, 100); err != nil {
				t.Fatal(err)
			}
			for _, id := range []uint{first.ID, second.ID} {
				if _, err := fixture.services.Cart.Add(ctx, fixture.userID, id, 1); err != nil {
					t.Fatal(err)
				}
			}
			order, _, err := fixture.services.Orders.Create(ctx, fixture.userID, "ordered-locks", CreateOrderInput{})
			if err != nil {
				t.Fatal(err)
			}
			// Reinsert immutable snapshots in descending SKU order; quantities and
			// reservations are unchanged. Everything is in this test's private schema.
			if err := fixture.db.Where("order_id = ?", order.ID).Delete(&model.OrderItem{}).Error; err != nil {
				t.Fatal(err)
			}
			for i := len(order.Items) - 1; i >= 0; i-- {
				item := order.Items[i]
				item.ID = 0
				if err := fixture.db.Create(&item).Error; err != nil {
					t.Fatal(err)
				}
			}
			payment, err := fixture.services.Payments.Create(ctx, fixture.userID, order.ID)
			if err != nil {
				t.Fatal(err)
			}
			blocker := fixture.db.WithContext(ctx).Begin()
			if blocker.Error != nil {
				t.Fatal(blocker.Error)
			}
			t.Cleanup(func() { blocker.Rollback() })
			var high model.Inventory
			if err := blocker.Clauses(clause.Locking{Strength: "UPDATE"}).First(&high, "sku_id = ?", second.ID).Error; err != nil {
				t.Fatal(err)
			}
			done := make(chan error, 1)
			go func() {
				if operation == "cancel" {
					_, err := fixture.services.Orders.Cancel(ctx, fixture.userID, order.ID)
					done <- err
				} else {
					provider := fixture.services.Payments.provider.(*MockPaymentProvider)
					callback := PaymentCallback{EventID: "ordered-payment", OrderNo: order.OrderNo, Reference: payment.Reference, Amount: payment.Amount, Status: PaymentCallbackSuccess}
					_, _, err := fixture.services.Payments.HandleWebhook(ctx, callback, provider.Sign(callback))
					done <- err
				}
			}()
			waitForCheckoutLocks(t, fixture.db, 1, nil)
			// A separate PostgreSQL transaction must observe low SKU already locked
			// while the operation waits on high SKU. NOWAIT makes this deterministic.
			probe := fixture.db.WithContext(ctx).Begin()
			if probe.Error != nil {
				t.Fatal(probe.Error)
			}
			var low model.Inventory
			lockErr := probe.Clauses(clause.Locking{Strength: "UPDATE", Options: "NOWAIT"}).First(&low, "sku_id = ?", first.ID).Error
			if err := probe.Rollback().Error; err != nil {
				t.Fatal(err)
			}
			var sqlState interface{ SQLState() string }
			if !errors.As(lockErr, &sqlState) || sqlState.SQLState() != "55P03" {
				t.Errorf("%s low SKU NOWAIT error = %v, want PostgreSQL lock_not_available (55P03)", operation, lockErr)
			}
			if err := blocker.Commit().Error; err != nil {
				t.Fatal(err)
			}
			if err := <-done; err != nil {
				t.Fatalf("%s after releasing high SKU: %v", operation, err)
			}
			for _, id := range []uint{first.ID, second.ID} {
				inventory, err := fixture.repo.Inventory(ctx, id)
				if err != nil {
					t.Fatal(err)
				}
				wantAvailable := int64(99)
				if operation == "cancel" {
					wantAvailable = 100
				}
				if inventory.Reserved != 0 || inventory.Available != wantAvailable {
					t.Errorf("%s SKU %d inventory = %+v, want reserved=0 available=%d", operation, id, inventory, wantAvailable)
				}
			}
		})
	}
}
