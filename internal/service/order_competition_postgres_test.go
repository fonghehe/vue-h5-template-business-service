package service

import (
	"context"
	"testing"
	"time"

	"github.com/fonghehe/vue-h5-template-business-service/internal/model"
	"github.com/fonghehe/vue-h5-template-business-service/internal/repository"
)

func TestPostgresPaymentCancelAndExpiryConsumeReservationOnce(t *testing.T) {
	for _, expired := range []bool{false, true} {
		name := "live"
		if expired {
			name = "expired"
		}
		t.Run(name, func(t *testing.T) {
			fixture := newPostgresCheckoutFixture(t)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			coupon := model.Coupon{Code: "RACE", Type: model.PromotionFixedDiscount, Value: 10,
				StartAt: time.Now().Add(-time.Hour), EndAt: time.Now().Add(time.Hour), UsageLimit: 100, PerUserLimit: 1}
			if err := fixture.db.WithContext(ctx).Create(&coupon).Error; err != nil {
				t.Fatal(err)
			}
			if _, err := fixture.services.Cart.Add(ctx, fixture.userID, fixture.skuID, 1); err != nil {
				t.Fatal(err)
			}
			order, _, err := fixture.services.Orders.Create(ctx, fixture.userID, "competing-finalizers", CreateOrderInput{CouponCode: coupon.Code})
			if err != nil {
				t.Fatal(err)
			}
			payment, err := fixture.services.Payments.Create(ctx, fixture.userID, order.ID)
			if err != nil {
				t.Fatal(err)
			}
			if expired {
				if err := fixture.db.WithContext(ctx).Model(&model.Order{}).Where("id = ?", order.ID).Update("expires_at", time.Now().Add(-time.Minute)).Error; err != nil {
					t.Fatal(err)
				}
			}
			provider := fixture.services.Payments.provider.(*MockPaymentProvider)
			callback := PaymentCallback{EventID: "competing-callback", OrderNo: order.OrderNo, Reference: payment.Reference, Amount: payment.Amount, Status: PaymentCallbackSuccess}
			type outcome struct {
				operation string
				changed   int
				err       error
			}
			start := make(chan struct{})
			outcomes := make(chan outcome, 3)
			go func() {
				<-start
				_, _, err := fixture.services.Payments.HandleWebhook(ctx, callback, provider.Sign(callback))
				outcomes <- outcome{"payment", 1, err}
			}()
			go func() {
				<-start
				_, err := fixture.services.Orders.Cancel(ctx, fixture.userID, order.ID)
				outcomes <- outcome{"cancel", 1, err}
			}()
			go func() {
				<-start
				count, err := fixture.services.Orders.ExpireBatch(ctx)
				outcomes <- outcome{"expiry", count, err}
			}()
			close(start)
			changes := 0
			for range 3 {
				result := <-outcomes
				if result.err == nil {
					changes += result.changed
				} else if result.operation == "expiry" || !repository.IsConflict(result.err) {
					t.Errorf("concurrent %s(%s order) err=%v, want success or state conflict", result.operation, name, result.err)
				}
			}
			if changes != 1 {
				t.Errorf("payment/cancel/expiry(%s) successful transitions=%d, want 1", name, changes)
			}
			stored, err := fixture.services.Orders.Get(ctx, fixture.userID, order.ID)
			if err != nil {
				t.Fatal(err)
			}
			if stored.Version != 2 || len(stored.StatusHistory) != 2 {
				t.Errorf("final order version=%d history=%d, want 2,2", stored.Version, len(stored.StatusHistory))
			}
			wantStock, wantUsage := int64(100), model.CouponUsageReleased
			switch stored.Status {
			case model.OrderPaid:
				if expired {
					t.Error("payment succeeded after expiry, want CANCELLED or EXPIRED")
				}
				wantStock, wantUsage = 99, model.CouponUsageConsumed
			case model.OrderCancelled:
			case model.OrderExpired:
				if !expired {
					t.Error("live order expired, want PAID or CANCELLED")
				}
			default:
				t.Errorf("final order status=%s, want PAID, CANCELLED or EXPIRED", stored.Status)
			}
			inventory, err := fixture.repo.Inventory(ctx, fixture.skuID)
			if err != nil {
				t.Fatal(err)
			}
			if inventory.Available != wantStock || inventory.Reserved != 0 {
				t.Errorf("final inventory=%+v, want available=%d reserved=0", inventory, wantStock)
			}
			var usage model.CouponUsage
			if err := fixture.db.WithContext(ctx).Where("order_id = ?", order.ID).First(&usage).Error; err != nil {
				t.Fatal(err)
			}
			if usage.Status != wantUsage {
				t.Errorf("final coupon status=%s, want %s", usage.Status, wantUsage)
			}
		})
	}
}
