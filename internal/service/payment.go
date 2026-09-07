package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/fonghehe/vue-h5-template-business-service/internal/apierr"
	"github.com/fonghehe/vue-h5-template-business-service/internal/metrics"
	"github.com/fonghehe/vue-h5-template-business-service/internal/model"
	"github.com/fonghehe/vue-h5-template-business-service/internal/repository"
	"github.com/fonghehe/vue-h5-template-business-service/internal/response"
)

const (
	PaymentCallbackSuccess = "SUCCESS"
	PaymentCallbackFailure = "FAILURE"
)

type PaymentIntent struct {
	Provider  string `json:"provider"`
	Reference string `json:"reference"`
	OrderNo   string `json:"orderNo"`
	Amount    int64  `json:"amount"`
	Status    string `json:"status"`
}

type PaymentCallback struct {
	EventID   string `json:"eventId"`
	OrderNo   string `json:"orderNo"`
	Reference string `json:"reference"`
	Amount    int64  `json:"amount"`
	Status    string `json:"status"`
}

// PaymentProvider isolates provider-specific intent creation and webhook
// verification. Order state and inventory remain owned by this service.
type PaymentProvider interface {
	Name() string
	CreatePayment(context.Context, model.Order) (PaymentIntent, error)
	VerifyWebhook(PaymentCallback, string) error
}

type MockPaymentProvider struct {
	secret []byte
}

func NewMockPaymentProvider(secret string) *MockPaymentProvider {
	return &MockPaymentProvider{secret: []byte(secret)}
}

func (p *MockPaymentProvider) Name() string { return "mock" }

func (p *MockPaymentProvider) CreatePayment(_ context.Context, order model.Order) (PaymentIntent, error) {
	return PaymentIntent{
		Provider: p.Name(), Reference: "mock_" + strings.ReplaceAll(uuid.NewString(), "-", ""),
		OrderNo: order.OrderNo, Amount: order.PayableAmount, Status: model.PaymentPending,
	}, nil
}

func (p *MockPaymentProvider) VerifyWebhook(callback PaymentCallback, signature string) error {
	if strings.TrimSpace(callback.EventID) == "" || strings.TrimSpace(callback.OrderNo) == "" || strings.TrimSpace(callback.Reference) == "" {
		return apierr.Validation("payment callback identifiers are required")
	}
	if callback.Amount < 0 {
		return apierr.Validation("payment callback amount must not be negative")
	}
	if callback.Status != PaymentCallbackSuccess && callback.Status != PaymentCallbackFailure {
		return apierr.Validation("payment callback status must be SUCCESS or FAILURE")
	}
	expected := p.Sign(callback)
	expectedBytes, _ := hex.DecodeString(expected)
	provided, err := hex.DecodeString(strings.TrimSpace(signature))
	if err != nil || !hmac.Equal(expectedBytes, provided) {
		return apierr.Unauthorized("invalid payment callback signature")
	}
	return nil
}

// Sign is public so local integration tests and the documented mock-payment
// workflow can produce exactly the callback a real provider would sign.
func (p *MockPaymentProvider) Sign(callback PaymentCallback) string {
	mac := hmac.New(sha256.New, p.secret)
	_, _ = mac.Write([]byte(callbackCanonical(callback)))
	return hex.EncodeToString(mac.Sum(nil))
}

func callbackCanonical(callback PaymentCallback) string {
	// Length-prefix strings so separator characters inside provider identifiers
	// cannot produce two different callbacks with the same canonical payload.
	return fmt.Sprintf("%d:%s|%d:%s|%d:%s|%s|%d:%s",
		len(callback.EventID), callback.EventID,
		len(callback.OrderNo), callback.OrderNo,
		len(callback.Reference), callback.Reference,
		strconv.FormatInt(callback.Amount, 10),
		len(callback.Status), callback.Status,
	)
}

type PaymentService struct {
	repo     *repository.CommerceRepository
	provider PaymentProvider
	logger   *slog.Logger
	now      func() time.Time
	metrics  *metrics.Metrics
}

func NewPaymentService(repo *repository.CommerceRepository, provider PaymentProvider, logger *slog.Logger, metricSet *metrics.Metrics) *PaymentService {
	if logger == nil {
		logger = slog.Default()
	}
	return &PaymentService{repo: repo, provider: provider, logger: logger, now: time.Now, metrics: metricSet}
}

func (s *PaymentService) Create(ctx context.Context, userID, orderID uint) (model.Payment, error) {
	var payment model.Payment
	err := s.repo.Transaction(ctx, func(tx *repository.CommerceRepository) error {
		order, err := tx.LockOrderByID(ctx, orderID)
		if err != nil {
			return err
		}
		if order.UserID != userID {
			return apierr.NotFound("order not found")
		}
		if order.Status != model.OrderPendingPayment {
			return apierr.Conflict("payment can only be created for a pending order")
		}
		if !s.now().UTC().Before(order.ExpiresAt) {
			return apierr.Conflict("order has expired")
		}
		if existing, findErr := tx.FindPaymentByOrder(ctx, order.ID); findErr == nil {
			payment = existing
			return nil
		} else if !repository.IsNotFound(findErr) {
			return findErr
		}
		intent, err := s.provider.CreatePayment(ctx, order)
		if err != nil {
			return apierr.Wrap(apierr.CodeUnavailable, "payment provider is unavailable", err)
		}
		if intent.OrderNo != order.OrderNo || intent.Amount != order.PayableAmount || intent.Reference == "" {
			return apierr.Internal("payment provider returned an invalid intent")
		}
		payment = model.Payment{
			OrderID: order.ID, Provider: s.provider.Name(), Reference: intent.Reference,
			Amount: order.PayableAmount, Status: model.PaymentPending,
		}
		return tx.CreatePayment(ctx, &payment)
	})
	return payment, repository.WrapTransaction(err)
}

func (s *PaymentService) HandleWebhook(ctx context.Context, callback PaymentCallback, signature string) (model.Order, bool, error) {
	if err := s.provider.VerifyWebhook(callback, signature); err != nil {
		return model.Order{}, false, err
	}
	now := s.now().UTC()
	var result model.Order
	duplicate := false
	err := s.repo.Transaction(ctx, func(tx *repository.CommerceRepository) error {
		payment, err := tx.LockPaymentByReference(ctx, callback.Reference)
		if err != nil {
			return err
		}
		order, err := tx.LockOrderByNo(ctx, callback.OrderNo)
		if err != nil {
			return err
		}
		if payment.OrderID != order.ID || payment.Reference != callback.Reference {
			return apierr.Conflict("payment reference does not belong to the order")
		}
		if payment.Amount != callback.Amount || order.PayableAmount != callback.Amount {
			return apierr.Conflict("payment amount does not match the order")
		}
		payloadHash := sha256.Sum256([]byte(callbackCanonical(callback)))

		inserted, err := tx.RecordWebhookEvent(ctx, &model.PaymentWebhookEvent{
			Provider: s.provider.Name(), EventID: callback.EventID, PaymentID: payment.ID,
			PayloadHash: hex.EncodeToString(payloadHash[:]),
		})
		if err != nil {
			return err
		}
		if !inserted {
			prior, findErr := tx.FindWebhookEvent(ctx, s.provider.Name(), callback.EventID)
			if findErr != nil {
				return findErr
			}
			if prior.PaymentID != payment.ID || prior.PayloadHash != hex.EncodeToString(payloadHash[:]) {
				return apierr.Conflict("payment event ID was already used with a different callback")
			}
			duplicate = true
			result = order
			return nil
		}

		if callback.Status == PaymentCallbackFailure {
			if order.Status != model.OrderPendingPayment {
				return apierr.Conflict(fmt.Sprintf("order in status %s cannot receive a payment failure", order.Status))
			}
			if payment.Status == model.PaymentPending {
				payment.Status = model.PaymentFailed
				payment.FailureReason = "provider reported failure"
				if err := tx.SavePayment(ctx, &payment); err != nil {
					return err
				}
			}
			result = order
			return nil
		}

		if order.Status == model.OrderPaid && payment.Status == model.PaymentSucceeded {
			duplicate = true
			result = order
			return nil
		}
		if order.Status != model.OrderPendingPayment {
			return apierr.Conflict(fmt.Sprintf("order in status %s cannot be paid", order.Status))
		}
		if !now.Before(order.ExpiresAt) {
			return apierr.Conflict("order has expired")
		}
		for _, item := range order.Items {
			if err := tx.CommitInventory(ctx, item.SKUID, int64(item.Quantity)); err != nil {
				return err
			}
		}
		history, err := TransitionOrder(&order, model.OrderPaid, "mock payment confirmed", now)
		if err != nil {
			return err
		}
		if order.CouponID != nil {
			if err := tx.UpdateCouponUsage(ctx, order.ID, model.CouponUsageReserved, model.CouponUsageConsumed); err != nil {
				return err
			}
		}
		if err := tx.SaveOrderAndHistory(ctx, &order, &history); err != nil {
			return err
		}
		payment.Status = model.PaymentSucceeded
		payment.FailureReason = ""
		payment.PaidAt = &now
		if err := tx.SavePayment(ctx, &payment); err != nil {
			return err
		}
		result = order
		return nil
	})
	if err != nil {
		return model.Order{}, false, repository.WrapTransaction(err)
	}
	if result.ID != 0 {
		loaded, loadErr := s.repo.FindOrderForUser(ctx, result.UserID, result.ID)
		if loadErr == nil {
			result = loaded
		}
	}
	if callback.Status == PaymentCallbackSuccess && !duplicate {
		if s.metrics != nil {
			s.metrics.OrdersPaid.Inc()
		}
		s.logger.InfoContext(ctx, "order.paid", "requestId", response.RequestIDFromContext(ctx), "userId", result.UserID, "orderNo", result.OrderNo,
			"paymentReference", callback.Reference, "amount", callback.Amount)
	}
	return result, duplicate, nil
}
