package service

import (
	"context"
	"testing"

	"github.com/fonghehe/vue-h5-template-business-service/internal/model"
	"github.com/fonghehe/vue-h5-template-business-service/internal/repository"
)

func TestPaymentRejectsAnotherProvidersRecord(t *testing.T) {
	for _, operation := range []string{"create", PaymentCallbackSuccess, PaymentCallbackFailure} {
		t.Run(operation, func(t *testing.T) {
			fixture := newCommerceFixture(t)
			ctx := context.Background()
			if _, err := fixture.services.Cart.Add(ctx, fixture.userID, fixture.skuID, 1); err != nil {
				t.Fatal(err)
			}
			order, _, err := fixture.services.Orders.Create(ctx, fixture.userID, "provider-ownership", CreateOrderInput{CouponCode: "WELCOME10"})
			if err != nil {
				t.Fatal(err)
			}
			payment, err := fixture.services.Payments.Create(ctx, fixture.userID, order.ID)
			if err != nil {
				t.Fatal(err)
			}
			if err := fixture.db.Model(&model.Payment{}).Where("id = ?", payment.ID).Update("provider", "another-provider").Error; err != nil {
				t.Fatal(err)
			}
			if operation == "create" {
				_, err = fixture.services.Payments.Create(ctx, fixture.userID, order.ID)
			} else {
				provider := fixture.services.Payments.provider.(*MockPaymentProvider)
				callback := PaymentCallback{EventID: "wrong-provider", OrderNo: order.OrderNo, Reference: payment.Reference, Amount: payment.Amount, Status: operation}
				_, _, err = fixture.services.Payments.HandleWebhook(ctx, callback, provider.Sign(callback))
			}
			if !repository.IsConflict(err) {
				t.Errorf("Payments.%s(other provider) error = %v, want conflict", operation, err)
			}
			stored, err := fixture.services.Orders.Get(ctx, fixture.userID, order.ID)
			if err != nil {
				t.Fatal(err)
			}
			if stored.Status != model.OrderPendingPayment || len(stored.StatusHistory) != 1 {
				t.Errorf("Payments.%s(other provider) order = %+v, want unchanged pending order", operation, stored)
			}
			saved, err := fixture.repo.FindPaymentByOrder(ctx, order.ID)
			if err != nil {
				t.Fatal(err)
			}
			if saved.Status != model.PaymentPending {
				t.Errorf("Payments.%s(other provider) payment status = %s, want PENDING", operation, saved.Status)
			}
			inventory, err := fixture.repo.Inventory(ctx, fixture.skuID)
			if err != nil {
				t.Fatal(err)
			}
			if inventory.Reserved != 1 || inventory.Available != fixture.inventory.Available-1 {
				t.Errorf("Payments.%s(other provider) inventory = %+v, want unchanged reservation", operation, inventory)
			}
			var usage model.CouponUsage
			if err := fixture.db.Where("order_id = ?", order.ID).First(&usage).Error; err != nil {
				t.Fatal(err)
			}
			if usage.Status != model.CouponUsageReserved {
				t.Errorf("Payments.%s(other provider) coupon = %s, want RESERVED", operation, usage.Status)
			}
			var count int64
			if err := fixture.db.Model(&model.PaymentWebhookEvent{}).Count(&count).Error; err != nil {
				t.Fatal(err)
			}
			if count != 0 {
				t.Errorf("Payments.%s(other provider) events = %d, want 0", operation, count)
			}
		})
	}
}
