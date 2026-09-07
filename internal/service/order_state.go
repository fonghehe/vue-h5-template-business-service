package service

import (
	"fmt"
	"time"

	"github.com/fonghehe/vue-h5-template-business-service/internal/apierr"
	"github.com/fonghehe/vue-h5-template-business-service/internal/model"
)

var allowedOrderTransitions = map[string]map[string]struct{}{
	model.OrderPendingPayment: {
		model.OrderPaid:      {},
		model.OrderCancelled: {},
		model.OrderExpired:   {},
	},
	model.OrderPaid: {
		model.OrderProcessing: {},
	},
	model.OrderProcessing: {
		model.OrderCompleted: {},
	},
}

// TransitionOrder is the single order state-machine entry point. Callers must
// persist the returned history in the same transaction as the mutated order.
func TransitionOrder(order *model.Order, target, reason string, now time.Time) (model.OrderStatusHistory, error) {
	if order == nil {
		return model.OrderStatusHistory{}, apierr.Internal("order state is unavailable")
	}
	if _, ok := allowedOrderTransitions[order.Status][target]; !ok {
		return model.OrderStatusHistory{}, apierr.Conflict(fmt.Sprintf("order cannot transition from %s to %s", order.Status, target))
	}

	previous := order.Status
	order.Status = target
	order.Version++
	switch target {
	case model.OrderPaid:
		paidAt := now
		order.PaidAt = &paidAt
	case model.OrderCancelled:
		cancelledAt := now
		order.CancelledAt = &cancelledAt
	}

	return model.OrderStatusHistory{
		OrderID: order.ID, FromStatus: previous, ToStatus: target, Reason: reason, CreatedAt: now,
	}, nil
}
