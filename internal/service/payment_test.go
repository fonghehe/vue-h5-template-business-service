package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fonghehe/vue-h5-template-business-service/internal/apierr"
	"github.com/fonghehe/vue-h5-template-business-service/internal/model"
)

func TestPaymentWebhookIsIdempotent(t *testing.T) {
	fixture := newCommerceFixture(t)
	ctx := context.Background()
	_, err := fixture.services.Cart.Add(ctx, fixture.userID, fixture.skuID, 2)
	require.NoError(t, err)
	order, _, err := fixture.services.Orders.Create(ctx, fixture.userID, "payment-order", CreateOrderInput{CouponCode: "WELCOME10"})
	require.NoError(t, err)
	payment, err := fixture.services.Payments.Create(ctx, fixture.userID, order.ID)
	require.NoError(t, err)

	callback := PaymentCallback{
		EventID: "evt-paid-once", OrderNo: order.OrderNo, Reference: payment.Reference,
		Amount: order.PayableAmount, Status: PaymentCallbackSuccess,
	}
	provider := fixture.services.Payments.provider.(*MockPaymentProvider)
	signature := provider.Sign(callback)
	for attempt := 0; attempt < 10; attempt++ {
		paid, duplicate, webhookErr := fixture.services.Payments.HandleWebhook(ctx, callback, signature)
		require.NoError(t, webhookErr)
		assert.Equal(t, model.OrderPaid, paid.Status)
		assert.Equal(t, attempt > 0, duplicate)
	}

	var paidHistory int64
	require.NoError(t, fixture.db.Model(&model.OrderStatusHistory{}).
		Where("order_id = ? AND to_status = ?", order.ID, model.OrderPaid).Count(&paidHistory).Error)
	assert.Equal(t, int64(1), paidHistory)
	var webhookEvents int64
	require.NoError(t, fixture.db.Model(&model.PaymentWebhookEvent{}).Count(&webhookEvents).Error)
	assert.Equal(t, int64(1), webhookEvents)
	inventory, err := fixture.repo.Inventory(ctx, fixture.skuID)
	require.NoError(t, err)
	assert.Equal(t, fixture.inventory.Available-2, inventory.Available)
	assert.Zero(t, inventory.Reserved)
	var usage model.CouponUsage
	require.NoError(t, fixture.db.Where("order_id = ?", order.ID).First(&usage).Error)
	assert.Equal(t, model.CouponUsageConsumed, usage.Status)
}

func TestPaymentWebhookRejectsAmountReferenceAndSignatureMismatch(t *testing.T) {
	fixture := newCommerceFixture(t)
	ctx := context.Background()
	_, err := fixture.services.Cart.Add(ctx, fixture.userID, fixture.skuID, 1)
	require.NoError(t, err)
	order, _, err := fixture.services.Orders.Create(ctx, fixture.userID, "payment-invalid", CreateOrderInput{})
	require.NoError(t, err)
	payment, err := fixture.services.Payments.Create(ctx, fixture.userID, order.ID)
	require.NoError(t, err)
	provider := fixture.services.Payments.provider.(*MockPaymentProvider)

	badAmount := PaymentCallback{EventID: "evt-bad-amount", OrderNo: order.OrderNo, Reference: payment.Reference, Amount: order.PayableAmount + 1, Status: PaymentCallbackSuccess}
	_, _, err = fixture.services.Payments.HandleWebhook(ctx, badAmount, provider.Sign(badAmount))
	require.Error(t, err)
	badReference := PaymentCallback{EventID: "evt-bad-ref", OrderNo: order.OrderNo, Reference: "unknown", Amount: order.PayableAmount, Status: PaymentCallbackSuccess}
	_, _, err = fixture.services.Payments.HandleWebhook(ctx, badReference, provider.Sign(badReference))
	require.Error(t, err)
	valid := PaymentCallback{EventID: "evt-valid", OrderNo: order.OrderNo, Reference: payment.Reference, Amount: order.PayableAmount, Status: PaymentCallbackSuccess}
	_, _, err = fixture.services.Payments.HandleWebhook(ctx, valid, "not-a-signature")
	require.Error(t, err)

	current, err := fixture.services.Orders.Get(ctx, fixture.userID, order.ID)
	require.NoError(t, err)
	assert.Equal(t, model.OrderPendingPayment, current.Status)
}

func TestPaymentFailureLeavesOrderPendingForRetryOrTimeout(t *testing.T) {
	fixture := newCommerceFixture(t)
	ctx := context.Background()
	_, err := fixture.services.Cart.Add(ctx, fixture.userID, fixture.skuID, 1)
	require.NoError(t, err)
	order, _, err := fixture.services.Orders.Create(ctx, fixture.userID, "payment-failure", CreateOrderInput{})
	require.NoError(t, err)
	payment, err := fixture.services.Payments.Create(ctx, fixture.userID, order.ID)
	require.NoError(t, err)
	callback := PaymentCallback{EventID: "evt-failed", OrderNo: order.OrderNo, Reference: payment.Reference,
		Amount: order.PayableAmount, Status: PaymentCallbackFailure}
	provider := fixture.services.Payments.provider.(*MockPaymentProvider)

	result, duplicate, err := fixture.services.Payments.HandleWebhook(ctx, callback, provider.Sign(callback))

	require.NoError(t, err)
	assert.False(t, duplicate)
	assert.Equal(t, model.OrderPendingPayment, result.Status)
	storedPayment, err := fixture.repo.FindPaymentByOrder(ctx, order.ID)
	require.NoError(t, err)
	assert.Equal(t, model.PaymentFailed, storedPayment.Status)
	inventory, err := fixture.repo.Inventory(ctx, fixture.skuID)
	require.NoError(t, err)
	assert.Equal(t, int64(1), inventory.Reserved)
}

func TestPaymentWebhookRejectsEventIDReusedWithDifferentPayload(t *testing.T) {
	fixture := newCommerceFixture(t)
	ctx := context.Background()
	_, err := fixture.services.Cart.Add(ctx, fixture.userID, fixture.skuID, 1)
	require.NoError(t, err)
	order, _, err := fixture.services.Orders.Create(ctx, fixture.userID, "event-payload", CreateOrderInput{})
	require.NoError(t, err)
	payment, err := fixture.services.Payments.Create(ctx, fixture.userID, order.ID)
	require.NoError(t, err)
	provider := fixture.services.Payments.provider.(*MockPaymentProvider)
	callback := PaymentCallback{EventID: "same-event", OrderNo: order.OrderNo, Reference: payment.Reference,
		Amount: order.PayableAmount, Status: PaymentCallbackFailure}
	_, _, err = fixture.services.Payments.HandleWebhook(ctx, callback, provider.Sign(callback))
	require.NoError(t, err)
	callback.Status = PaymentCallbackSuccess
	_, _, err = fixture.services.Payments.HandleWebhook(ctx, callback, provider.Sign(callback))
	var apiError *apierr.Error
	require.ErrorAs(t, err, &apiError)
	assert.Equal(t, apierr.CodeConflict, apiError.Code)
	current, err := fixture.services.Orders.Get(ctx, fixture.userID, order.ID)
	require.NoError(t, err)
	assert.Equal(t, model.OrderPendingPayment, current.Status)
}

func TestPaymentFailureCannotMutatePaidOrder(t *testing.T) {
	fixture := newCommerceFixture(t)
	ctx := context.Background()
	_, err := fixture.services.Cart.Add(ctx, fixture.userID, fixture.skuID, 1)
	require.NoError(t, err)
	order, _, err := fixture.services.Orders.Create(ctx, fixture.userID, "failure-after-paid", CreateOrderInput{})
	require.NoError(t, err)
	payment, err := fixture.services.Payments.Create(ctx, fixture.userID, order.ID)
	require.NoError(t, err)
	provider := fixture.services.Payments.provider.(*MockPaymentProvider)
	paid := PaymentCallback{EventID: "paid-first", OrderNo: order.OrderNo, Reference: payment.Reference,
		Amount: order.PayableAmount, Status: PaymentCallbackSuccess}
	_, _, err = fixture.services.Payments.HandleWebhook(ctx, paid, provider.Sign(paid))
	require.NoError(t, err)
	failure := paid
	failure.EventID = "late-failure"
	failure.Status = PaymentCallbackFailure
	_, _, err = fixture.services.Payments.HandleWebhook(ctx, failure, provider.Sign(failure))
	var apiError *apierr.Error
	require.ErrorAs(t, err, &apiError)
	assert.Equal(t, apierr.CodeConflict, apiError.Code)
	storedPayment, err := fixture.repo.FindPaymentByOrder(ctx, order.ID)
	require.NoError(t, err)
	assert.Equal(t, model.PaymentSucceeded, storedPayment.Status)
}

func TestPaymentIntentCannotBeCreatedAfterOrderExpires(t *testing.T) {
	fixture := newCommerceFixture(t)
	ctx := context.Background()
	_, err := fixture.services.Cart.Add(ctx, fixture.userID, fixture.skuID, 1)
	require.NoError(t, err)
	order, _, err := fixture.services.Orders.Create(ctx, fixture.userID, "intent-after-expiry", CreateOrderInput{})
	require.NoError(t, err)
	fixture.services.Payments.now = func() time.Time { return order.ExpiresAt }
	_, err = fixture.services.Payments.Create(ctx, fixture.userID, order.ID)
	var apiError *apierr.Error
	require.ErrorAs(t, err, &apiError)
	assert.Equal(t, apierr.CodeConflict, apiError.Code)
}
