package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/fonghehe/vue-h5-template-business-service/internal/apierr"
	"github.com/fonghehe/vue-h5-template-business-service/internal/config"
	"github.com/fonghehe/vue-h5-template-business-service/internal/metrics"
	"github.com/fonghehe/vue-h5-template-business-service/internal/model"
	"github.com/fonghehe/vue-h5-template-business-service/internal/repository"
	"github.com/fonghehe/vue-h5-template-business-service/internal/response"
)

const (
	MaxCartQuantity = 99
	MaxOrderItems   = 100
)

type CartService struct {
	repo *repository.CommerceRepository
}

func NewCartService(repo *repository.CommerceRepository) *CartService {
	return &CartService{repo: repo}
}

func (s *CartService) List(ctx context.Context, userID uint) ([]model.CartItem, error) {
	return s.repo.ListCart(ctx, userID)
}

func (s *CartService) Add(ctx context.Context, userID, skuID uint, quantity int) (model.CartItem, error) {
	if err := validateCartQuantity(quantity); err != nil {
		return model.CartItem{}, err
	}
	sku, err := s.repo.FindSKU(ctx, skuID)
	if err != nil {
		return model.CartItem{}, err
	}
	if sku.Status != model.SKUStatusActive || sku.Product.Status != model.StatusOnSale {
		return model.CartItem{}, apierr.Conflict("SKU is not available for sale")
	}
	item := model.CartItem{UserID: userID, SKUID: skuID, Quantity: quantity}
	if err := s.repo.UpsertCartItem(ctx, &item); err != nil {
		return model.CartItem{}, err
	}
	return s.repo.FindCartItem(ctx, userID, item.ID)
}

func (s *CartService) Update(ctx context.Context, userID, itemID uint, quantity int) (model.CartItem, error) {
	if err := validateCartQuantity(quantity); err != nil {
		return model.CartItem{}, err
	}
	item, err := s.repo.FindCartItem(ctx, userID, itemID)
	if err != nil {
		return model.CartItem{}, err
	}
	if item.SKU.Status != model.SKUStatusActive || item.SKU.Product.Status != model.StatusOnSale {
		return model.CartItem{}, apierr.Conflict("SKU is not available for sale")
	}
	return s.repo.UpdateCartQuantity(ctx, userID, itemID, quantity)
}

func (s *CartService) Delete(ctx context.Context, userID, itemID uint) error {
	return s.repo.DeleteCartItem(ctx, userID, itemID)
}

func (s *CartService) Clear(ctx context.Context, userID uint) error {
	return s.repo.ClearCart(ctx, userID)
}

func validateCartQuantity(quantity int) error {
	if quantity <= 0 {
		return apierr.Validation("quantity must be greater than zero")
	}
	if quantity > MaxCartQuantity {
		return apierr.Validation(fmt.Sprintf("quantity must not exceed %d", MaxCartQuantity))
	}
	return nil
}

type CreateOrderInput struct {
	CouponCode string
}

type OrderService struct {
	config  config.Config
	repo    *repository.CommerceRepository
	logger  *slog.Logger
	now     func() time.Time
	metrics *metrics.Metrics
}

func NewOrderService(cfg config.Config, repo *repository.CommerceRepository, logger *slog.Logger, metricSet *metrics.Metrics) *OrderService {
	if logger == nil {
		logger = slog.Default()
	}
	return &OrderService{config: cfg, repo: repo, logger: logger, now: time.Now, metrics: metricSet}
}

func (s *OrderService) Create(ctx context.Context, userID uint, idempotencyKey string, input CreateOrderInput) (model.Order, bool, error) {
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if idempotencyKey == "" || len(idempotencyKey) > 128 {
		return model.Order{}, false, apierr.Validation("Idempotency-Key is required and must not exceed 128 characters")
	}
	requestHash := orderRequestHash(input)
	if existing, err := s.repo.FindOrderByIdempotency(ctx, userID, idempotencyKey); err == nil {
		if existing.RequestHash != requestHash {
			return model.Order{}, false, apierr.Conflict("Idempotency-Key was already used with a different request")
		}
		return existing, true, nil
	} else if !repository.IsNotFound(err) {
		return model.Order{}, false, err
	}

	now := s.now().UTC()
	var created model.Order
	err := s.repo.Transaction(ctx, func(tx *repository.CommerceRepository) error {
		cart, err := tx.ListCart(ctx, userID)
		if err != nil {
			return err
		}
		if len(cart) == 0 {
			return apierr.Validation("cart is empty")
		}
		if len(cart) > MaxOrderItems {
			return apierr.Validation(fmt.Sprintf("cart must not contain more than %d items", MaxOrderItems))
		}
		// All checkouts lock SKU and inventory rows in one order to avoid
		// deadlocks when carts contain the same SKUs in different add order.
		sort.Slice(cart, func(i, j int) bool { return cart[i].SKUID < cart[j].SKUID })

		items := make([]model.OrderItem, 0, len(cart))
		var originalAmount int64
		for _, cartItem := range cart {
			if err := validateCartQuantity(cartItem.Quantity); err != nil {
				return err
			}
			sku, err := tx.LockSKUForCheckout(ctx, cartItem.SKUID)
			if err != nil {
				return err
			}
			if sku.Status != model.SKUStatusActive || sku.Product.Status != model.StatusOnSale {
				return apierr.Conflict(fmt.Sprintf("SKU %d is not available for sale", sku.ID))
			}
			quantity := int64(cartItem.Quantity)
			if sku.Price > math.MaxInt64/quantity || originalAmount > math.MaxInt64-sku.Price*quantity {
				return apierr.Validation("order amount is too large")
			}
			originalAmount += sku.Price * quantity
			items = append(items, model.OrderItem{
				ProductID: sku.ProductID, SKUID: sku.ID, ProductName: productDisplayName(sku.Product),
				SKUName: sku.Name, UnitPrice: sku.Price, Quantity: cartItem.Quantity,
			})
		}

		discount, coupon, err := s.discount(ctx, tx, userID, strings.TrimSpace(input.CouponCode), originalAmount, now)
		if err != nil {
			return err
		}
		for _, item := range items {
			if err := tx.ReserveInventory(ctx, item.SKUID, int64(item.Quantity)); err != nil {
				if s.metrics != nil {
					s.metrics.InventoryReservationFailed.Inc()
				}
				return err
			}
		}

		created = model.Order{
			OrderNo:        newOrderNo(now),
			UserID:         userID,
			Status:         model.OrderPendingPayment,
			OriginalAmount: originalAmount,
			DiscountAmount: discount,
			PayableAmount:  originalAmount - discount,
			IdempotencyKey: idempotencyKey,
			RequestHash:    requestHash,
			ExpiresAt:      now.Add(s.config.OrderPaymentTTL),
			Version:        1,
		}
		if coupon != nil {
			created.CouponID = &coupon.ID
		}
		history := model.OrderStatusHistory{FromStatus: "", ToStatus: model.OrderPendingPayment, Reason: "order created", CreatedAt: now}
		if err := tx.CreateOrder(ctx, &created, items, history); err != nil {
			return err
		}
		if coupon != nil {
			if err := tx.CreateCouponUsage(ctx, &model.CouponUsage{
				CouponID: coupon.ID, UserID: userID, OrderID: created.ID, Status: model.CouponUsageReserved,
			}); err != nil {
				return err
			}
		}
		return tx.ClearCart(ctx, userID)
	})
	if err != nil {
		// A concurrent request with the same key may have won the database unique
		// constraint. Its transaction is the canonical result; this transaction
		// has already rolled back all inventory and coupon changes.
		if existing, findErr := s.repo.FindOrderByIdempotency(ctx, userID, idempotencyKey); findErr == nil {
			if existing.RequestHash == requestHash {
				return existing, true, nil
			}
			return model.Order{}, false, apierr.Conflict("Idempotency-Key was already used with a different request")
		}
		return model.Order{}, false, repository.WrapTransaction(err)
	}

	order, err := s.repo.FindOrderForUser(ctx, userID, created.ID)
	if err != nil {
		return model.Order{}, false, err
	}
	s.logger.InfoContext(ctx, "order.created", "requestId", response.RequestIDFromContext(ctx), "userId", userID, "orderNo", order.OrderNo,
		"originalAmount", order.OriginalAmount, "discountAmount", order.DiscountAmount, "payableAmount", order.PayableAmount)
	if s.metrics != nil {
		s.metrics.OrdersCreated.Inc()
	}
	for _, item := range order.Items {
		s.logger.InfoContext(ctx, "inventory.reserved", "requestId", response.RequestIDFromContext(ctx), "userId", userID, "orderNo", order.OrderNo,
			"skuId", item.SKUID, "quantity", item.Quantity)
	}
	return order, false, nil
}

func (s *OrderService) discount(ctx context.Context, tx *repository.CommerceRepository, userID uint, code string, subtotal int64, now time.Time) (int64, *model.Coupon, error) {
	if code == "" {
		return 0, nil, nil
	}
	coupon, err := tx.LockCouponByCode(ctx, code)
	if err != nil {
		return 0, nil, err
	}
	total, byUser, err := tx.CouponUsageCounts(ctx, coupon.ID, userID)
	if err != nil {
		return 0, nil, err
	}
	discount, err := CalculateCouponDiscount(coupon, subtotal, userID, total, byUser, now)
	if err != nil {
		return 0, nil, err
	}
	return discount, &coupon, nil
}

func CalculateCouponDiscount(coupon model.Coupon, subtotal int64, userID uint, totalUsage, userUsage int64, now time.Time) (int64, error) {
	if now.Before(coupon.StartAt) || !now.Before(coupon.EndAt) {
		return 0, apierr.Conflict("coupon is not active")
	}
	if coupon.UserID != nil && *coupon.UserID != userID {
		return 0, apierr.NotFound("coupon not found")
	}
	if subtotal < coupon.MinimumAmount {
		return 0, apierr.Conflict("order does not meet the coupon minimum amount")
	}
	if totalUsage >= int64(coupon.UsageLimit) {
		return 0, apierr.Conflict("coupon usage limit has been reached")
	}
	if userUsage >= int64(coupon.PerUserLimit) {
		return 0, apierr.Conflict("coupon per-user limit has been reached")
	}

	var discount int64
	switch coupon.Type {
	case model.PromotionFixedDiscount, model.PromotionThresholdDiscount:
		discount = coupon.Value
	case model.PromotionPercentage:
		if coupon.Value <= 0 || coupon.Value > 10000 {
			return 0, apierr.Conflict("coupon percentage is invalid")
		}
		// Divide before multiplying so even a valid near-MaxInt64 order cannot
		// overflow while applying basis points.
		discount = (subtotal/10000)*coupon.Value + (subtotal%10000)*coupon.Value/10000
	default:
		return 0, apierr.Conflict("coupon type is invalid")
	}
	if discount > subtotal {
		discount = subtotal
	}
	return discount, nil
}

func (s *OrderService) Get(ctx context.Context, userID, orderID uint) (model.Order, error) {
	return s.repo.FindOrderForUser(ctx, userID, orderID)
}

func (s *OrderService) Advance(ctx context.Context, orderID uint, target string) (model.Order, error) {
	if target != model.OrderProcessing && target != model.OrderCompleted {
		return model.Order{}, apierr.Validation("admin transition target must be PROCESSING or COMPLETED")
	}
	now := s.now().UTC()
	err := s.repo.Transaction(ctx, func(tx *repository.CommerceRepository) error {
		order, err := tx.LockOrderByID(ctx, orderID)
		if err != nil {
			return err
		}
		history, err := TransitionOrder(&order, target, "advanced by operator", now)
		if err != nil {
			return err
		}
		return tx.SaveOrderAndHistory(ctx, &order, &history)
	})
	if err != nil {
		return model.Order{}, repository.WrapTransaction(err)
	}
	return s.repo.FindOrderByID(ctx, orderID)
}

func (s *OrderService) List(ctx context.Context, userID uint, page, pageSize int) (Page[model.Order], error) {
	orders, total, err := s.repo.ListOrdersForUser(ctx, userID, page, pageSize)
	if err != nil {
		return Page[model.Order]{}, err
	}
	return NewPage(orders, total, page, pageSize), nil
}

func (s *OrderService) Cancel(ctx context.Context, userID, orderID uint) (model.Order, error) {
	now := s.now().UTC()
	err := s.repo.Transaction(ctx, func(tx *repository.CommerceRepository) error {
		order, err := tx.LockOrderByID(ctx, orderID)
		if err != nil {
			return err
		}
		if order.UserID != userID {
			return apierr.NotFound("order not found")
		}
		history, err := TransitionOrder(&order, model.OrderCancelled, "cancelled by user", now)
		if err != nil {
			return err
		}
		for _, item := range order.Items {
			if err := tx.ReleaseInventory(ctx, item.SKUID, int64(item.Quantity)); err != nil {
				return err
			}
		}
		if order.CouponID != nil {
			if err := tx.UpdateCouponUsage(ctx, order.ID, model.CouponUsageReserved, model.CouponUsageReleased); err != nil {
				return err
			}
		}
		return tx.SaveOrderAndHistory(ctx, &order, &history)
	})
	if err != nil {
		return model.Order{}, repository.WrapTransaction(err)
	}
	order, err := s.repo.FindOrderForUser(ctx, userID, orderID)
	if err == nil {
		s.logger.InfoContext(ctx, "order.cancelled", "requestId", response.RequestIDFromContext(ctx), "userId", userID, "orderNo", order.OrderNo)
		for _, item := range order.Items {
			s.logger.InfoContext(ctx, "inventory.released", "requestId", response.RequestIDFromContext(ctx), "userId", userID, "orderNo", order.OrderNo,
				"skuId", item.SKUID, "quantity", item.Quantity, "reason", "cancelled")
		}
	}
	return order, err
}

func (s *OrderService) ExpireBatch(ctx context.Context) (int, error) {
	now := s.now().UTC()
	expiredOrders := make([]model.Order, 0, s.config.OrderExpirationBatchSize)
	err := s.repo.Transaction(ctx, func(tx *repository.CommerceRepository) error {
		ids, err := tx.LockExpiredOrderIDs(ctx, now, s.config.OrderExpirationBatchSize)
		if err != nil {
			return err
		}
		for _, id := range ids {
			order, err := tx.LockOrderByID(ctx, id)
			if err != nil {
				return err
			}
			if order.Status != model.OrderPendingPayment || !order.ExpiresAt.Before(now) {
				continue
			}
			history, err := TransitionOrder(&order, model.OrderExpired, "payment timeout", now)
			if err != nil {
				return err
			}
			for _, item := range order.Items {
				if err := tx.ReleaseInventory(ctx, item.SKUID, int64(item.Quantity)); err != nil {
					return err
				}
			}
			if order.CouponID != nil {
				if err := tx.UpdateCouponUsage(ctx, order.ID, model.CouponUsageReserved, model.CouponUsageReleased); err != nil {
					return err
				}
			}
			if err := tx.SaveOrderAndHistory(ctx, &order, &history); err != nil {
				return err
			}
			expiredOrders = append(expiredOrders, order)
		}
		return nil
	})
	if err != nil {
		return 0, repository.WrapTransaction(err)
	}
	for _, order := range expiredOrders {
		if s.metrics != nil {
			s.metrics.OrdersExpired.Inc()
		}
		s.logger.InfoContext(ctx, "order.expired", "requestId", response.RequestIDFromContext(ctx), "userId", order.UserID, "orderNo", order.OrderNo)
		for _, item := range order.Items {
			s.logger.InfoContext(ctx, "inventory.released", "requestId", response.RequestIDFromContext(ctx), "userId", order.UserID,
				"orderNo", order.OrderNo, "skuId", item.SKUID, "quantity", item.Quantity, "reason", "expired")
		}
	}
	return len(expiredOrders), nil
}

func productDisplayName(product model.Product) string {
	if strings.TrimSpace(product.Name) != "" {
		return product.Name
	}
	return product.Title
}

func orderRequestHash(input CreateOrderInput) string {
	sum := sha256.Sum256([]byte(strings.ToUpper(strings.TrimSpace(input.CouponCode))))
	return hex.EncodeToString(sum[:])
}

func newOrderNo(now time.Time) string {
	return "ORD" + now.Format("20060102150405") + strings.ToUpper(strings.ReplaceAll(uuid.NewString(), "-", "")[:12])
}
