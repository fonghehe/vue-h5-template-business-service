package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fonghehe/vue-h5-template-business-service/internal/apierr"
	"github.com/fonghehe/vue-h5-template-business-service/internal/model"
)

func TestOrderStateMachineAllowsOnlyDocumentedTransitions(t *testing.T) {
	now := time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		from string
		to   string
	}{
		{model.OrderPendingPayment, model.OrderPaid},
		{model.OrderPendingPayment, model.OrderCancelled},
		{model.OrderPendingPayment, model.OrderExpired},
		{model.OrderPaid, model.OrderProcessing},
		{model.OrderProcessing, model.OrderCompleted},
	}
	for _, test := range tests {
		t.Run(test.from+"_to_"+test.to, func(t *testing.T) {
			order := model.Order{ID: 7, Status: test.from, Version: 1}
			history, err := TransitionOrder(&order, test.to, "test", now)
			require.NoError(t, err)
			assert.Equal(t, test.to, order.Status)
			assert.Equal(t, int64(2), order.Version)
			assert.Equal(t, test.from, history.FromStatus)
			assert.Equal(t, test.to, history.ToStatus)
		})
	}
}

func TestOrderStateMachineRejectsIllegalTransitionWithConflict(t *testing.T) {
	order := model.Order{Status: model.OrderPendingPayment, Version: 3}

	_, err := TransitionOrder(&order, model.OrderCompleted, "skip states", time.Now())

	var apiError *apierr.Error
	require.ErrorAs(t, err, &apiError)
	assert.Equal(t, apierr.CodeConflict, apiError.Code)
	assert.Equal(t, model.OrderPendingPayment, order.Status)
	assert.Equal(t, int64(3), order.Version)
}
