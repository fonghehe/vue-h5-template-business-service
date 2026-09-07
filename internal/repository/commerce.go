package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/fonghehe/vue-h5-template-business-service/internal/apierr"
	"github.com/fonghehe/vue-h5-template-business-service/internal/model"
)

// CommerceRepository owns the SQL boundary for cart, promotion, order,
// inventory and payment. A transaction produces another repository bound to
// the transactional *gorm.DB, so service code cannot accidentally escape it.
type CommerceRepository struct {
	db *gorm.DB
}

func NewCommerceRepository(db *gorm.DB) *CommerceRepository {
	return &CommerceRepository{db: db}
}

func (r *CommerceRepository) Transaction(ctx context.Context, fn func(*CommerceRepository) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(NewCommerceRepository(tx))
	})
}

func (r *CommerceRepository) Dialect() string { return r.db.Name() }

func (r *CommerceRepository) ListCart(ctx context.Context, userID uint) ([]model.CartItem, error) {
	var items []model.CartItem
	err := r.db.WithContext(ctx).
		Preload("SKU").Preload("SKU.Product").
		Where("user_id = ?", userID).Order("id ASC").Find(&items).Error
	if err != nil {
		return nil, apierr.Wrap(apierr.CodeInternal, "could not load cart", err)
	}
	return items, nil
}

func (r *CommerceRepository) FindCartItem(ctx context.Context, userID, itemID uint) (model.CartItem, error) {
	var item model.CartItem
	err := r.db.WithContext(ctx).Preload("SKU").Preload("SKU.Product").
		Where("id = ? AND user_id = ?", itemID, userID).First(&item).Error
	if err != nil {
		return model.CartItem{}, translate(err, "cart item not found")
	}
	return item, nil
}

func (r *CommerceRepository) UpsertCartItem(ctx context.Context, item *model.CartItem) error {
	err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}, {Name: "sku_id"}},
		DoUpdates: clause.Assignments(map[string]any{"quantity": item.Quantity, "updated_at": time.Now()}),
	}).Create(item).Error
	if err != nil {
		return translate(err, "could not save cart item")
	}
	// PostgreSQL and SQLite both support RETURNING through GORM, but an update
	// conflict does not consistently hydrate ID. Reload for a stable response.
	return r.db.WithContext(ctx).Where("user_id = ? AND sku_id = ?", item.UserID, item.SKUID).First(item).Error
}

func (r *CommerceRepository) UpdateCartQuantity(ctx context.Context, userID, itemID uint, quantity int) (model.CartItem, error) {
	result := r.db.WithContext(ctx).Model(&model.CartItem{}).
		Where("id = ? AND user_id = ?", itemID, userID).Update("quantity", quantity)
	if result.Error != nil {
		return model.CartItem{}, apierr.Wrap(apierr.CodeInternal, "could not update cart item", result.Error)
	}
	if result.RowsAffected != 1 {
		return model.CartItem{}, apierr.NotFound("cart item not found")
	}
	return r.FindCartItem(ctx, userID, itemID)
}

func (r *CommerceRepository) DeleteCartItem(ctx context.Context, userID, itemID uint) error {
	result := r.db.WithContext(ctx).Where("id = ? AND user_id = ?", itemID, userID).Delete(&model.CartItem{})
	if result.Error != nil {
		return apierr.Wrap(apierr.CodeInternal, "could not remove cart item", result.Error)
	}
	if result.RowsAffected != 1 {
		return apierr.NotFound("cart item not found")
	}
	return nil
}

func (r *CommerceRepository) ClearCart(ctx context.Context, userID uint) error {
	if err := r.db.WithContext(ctx).Where("user_id = ?", userID).Delete(&model.CartItem{}).Error; err != nil {
		return apierr.Wrap(apierr.CodeInternal, "could not clear cart", err)
	}
	return nil
}

func (r *CommerceRepository) FindSKU(ctx context.Context, skuID uint) (model.ProductSKU, error) {
	var sku model.ProductSKU
	err := r.db.WithContext(ctx).Preload("Product").Preload("Inventory").First(&sku, skuID).Error
	if err != nil {
		return model.ProductSKU{}, translate(err, "SKU not found")
	}
	return sku, nil
}

func (r *CommerceRepository) CreateSKUWithInventory(ctx context.Context, sku *model.ProductSKU, available int64) error {
	return r.Transaction(ctx, func(tx *CommerceRepository) error {
		if err := tx.db.WithContext(ctx).Create(sku).Error; err != nil {
			return translate(err, "could not create SKU")
		}
		inventory := model.Inventory{SKUID: sku.ID, Available: available, Reserved: 0, Version: 1}
		if err := tx.db.WithContext(ctx).Create(&inventory).Error; err != nil {
			return translate(err, "could not create inventory")
		}
		return nil
	})
}

func (r *CommerceRepository) SaveSKU(ctx context.Context, sku *model.ProductSKU) error {
	result := r.db.WithContext(ctx).Model(&model.ProductSKU{}).Where("id = ?", sku.ID).Updates(map[string]any{
		"sku_code": sku.SKUCode, "name": sku.Name, "attributes": sku.Attributes,
		"price": sku.Price, "original_price": sku.OriginalPrice, "status": sku.Status,
	})
	if result.Error != nil {
		return translate(result.Error, "could not update SKU")
	}
	if result.RowsAffected != 1 {
		return apierr.NotFound("SKU not found")
	}
	return nil
}

func (r *CommerceRepository) LockSKUForCheckout(ctx context.Context, skuID uint) (model.ProductSKU, error) {
	var sku model.ProductSKU
	db := r.db.WithContext(ctx).Preload("Product")
	if r.Dialect() == "postgres" {
		db = db.Clauses(clause.Locking{Strength: "SHARE"})
	}
	if err := db.First(&sku, skuID).Error; err != nil {
		return model.ProductSKU{}, translate(err, "SKU not found")
	}
	return sku, nil
}

// ReserveInventory is an atomic compare-and-update. There is no application
// read/check/write race: PostgreSQL evaluates available >= quantity while
// holding the row update lock. RowsAffected=0 means insufficient stock.
func (r *CommerceRepository) ReserveInventory(ctx context.Context, skuID uint, quantity int64) error {
	result := r.db.WithContext(ctx).Model(&model.Inventory{}).
		Where("sku_id = ? AND available >= ?", skuID, quantity).
		Updates(map[string]any{
			"available": gorm.Expr("available - ?", quantity),
			"reserved":  gorm.Expr("reserved + ?", quantity),
			"version":   gorm.Expr("version + 1"),
		})
	if result.Error != nil {
		return apierr.Wrap(apierr.CodeInternal, "could not reserve inventory", result.Error)
	}
	if result.RowsAffected != 1 {
		return apierr.Conflict("insufficient inventory")
	}
	return nil
}

// ReleaseInventory returns a pending order's reservation to sellable stock.
func (r *CommerceRepository) ReleaseInventory(ctx context.Context, skuID uint, quantity int64) error {
	result := r.db.WithContext(ctx).Model(&model.Inventory{}).
		Where("sku_id = ? AND reserved >= ?", skuID, quantity).
		Updates(map[string]any{
			"available": gorm.Expr("available + ?", quantity),
			"reserved":  gorm.Expr("reserved - ?", quantity),
			"version":   gorm.Expr("version + 1"),
		})
	if result.Error != nil {
		return apierr.Wrap(apierr.CodeInternal, "could not release inventory", result.Error)
	}
	if result.RowsAffected != 1 {
		return apierr.Conflict("inventory reservation is inconsistent")
	}
	return nil
}

// CommitInventory converts reserved units into sold units after payment.
func (r *CommerceRepository) CommitInventory(ctx context.Context, skuID uint, quantity int64) error {
	result := r.db.WithContext(ctx).Model(&model.Inventory{}).
		Where("sku_id = ? AND reserved >= ?", skuID, quantity).
		Updates(map[string]any{
			"reserved": gorm.Expr("reserved - ?", quantity),
			"version":  gorm.Expr("version + 1"),
		})
	if result.Error != nil {
		return apierr.Wrap(apierr.CodeInternal, "could not commit inventory", result.Error)
	}
	if result.RowsAffected != 1 {
		return apierr.Conflict("inventory reservation is inconsistent")
	}
	return nil
}

func (r *CommerceRepository) Inventory(ctx context.Context, skuID uint) (model.Inventory, error) {
	var inventory model.Inventory
	if err := r.db.WithContext(ctx).First(&inventory, "sku_id = ?", skuID).Error; err != nil {
		return model.Inventory{}, translate(err, "inventory not found")
	}
	return inventory, nil
}

func (r *CommerceRepository) LockCouponByCode(ctx context.Context, code string) (model.Coupon, error) {
	var coupon model.Coupon
	db := r.db.WithContext(ctx)
	if r.Dialect() == "postgres" {
		db = db.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	if err := db.Where("UPPER(code) = ?", strings.ToUpper(strings.TrimSpace(code))).First(&coupon).Error; err != nil {
		return model.Coupon{}, translate(err, "coupon not found")
	}
	return coupon, nil
}

func (r *CommerceRepository) CouponUsageCounts(ctx context.Context, couponID, userID uint) (int64, int64, error) {
	active := []string{model.CouponUsageReserved, model.CouponUsageConsumed}
	var total, byUser int64
	base := r.db.WithContext(ctx).Model(&model.CouponUsage{}).
		Where("coupon_id = ? AND status IN ?", couponID, active)
	if err := base.Count(&total).Error; err != nil {
		return 0, 0, apierr.Wrap(apierr.CodeInternal, "could not validate coupon usage", err)
	}
	if err := base.Where("user_id = ?", userID).Count(&byUser).Error; err != nil {
		return 0, 0, apierr.Wrap(apierr.CodeInternal, "could not validate coupon user limit", err)
	}
	return total, byUser, nil
}

func (r *CommerceRepository) CreateOrder(ctx context.Context, order *model.Order, items []model.OrderItem, history model.OrderStatusHistory) error {
	if err := r.db.WithContext(ctx).Create(order).Error; err != nil {
		return translate(err, "could not create order")
	}
	for i := range items {
		items[i].OrderID = order.ID
	}
	if err := r.db.WithContext(ctx).Create(&items).Error; err != nil {
		return apierr.Wrap(apierr.CodeInternal, "could not create order items", err)
	}
	history.OrderID = order.ID
	if err := r.db.WithContext(ctx).Create(&history).Error; err != nil {
		return apierr.Wrap(apierr.CodeInternal, "could not create order history", err)
	}
	return nil
}

func (r *CommerceRepository) CreateCouponUsage(ctx context.Context, usage *model.CouponUsage) error {
	if err := r.db.WithContext(ctx).Create(usage).Error; err != nil {
		return translate(err, "coupon has already been used for this order")
	}
	return nil
}

func (r *CommerceRepository) UpdateCouponUsage(ctx context.Context, orderID uint, from, to string) error {
	result := r.db.WithContext(ctx).Model(&model.CouponUsage{}).
		Where("order_id = ? AND status = ?", orderID, from).Update("status", to)
	if result.Error != nil {
		return apierr.Wrap(apierr.CodeInternal, "could not update coupon usage", result.Error)
	}
	if result.RowsAffected != 1 {
		return apierr.Conflict("coupon reservation is inconsistent")
	}
	return nil
}

func preloadOrder(db *gorm.DB) *gorm.DB {
	return db.Preload("Items").Preload("StatusHistory", func(tx *gorm.DB) *gorm.DB {
		return tx.Order("id ASC")
	})
}

func (r *CommerceRepository) FindOrderByIdempotency(ctx context.Context, userID uint, key string) (model.Order, error) {
	var order model.Order
	err := preloadOrder(r.db.WithContext(ctx)).
		Where("user_id = ? AND idempotency_key = ?", userID, key).First(&order).Error
	if err != nil {
		return model.Order{}, translate(err, "order not found")
	}
	return order, nil
}

func (r *CommerceRepository) FindOrderForUser(ctx context.Context, userID, orderID uint) (model.Order, error) {
	var order model.Order
	err := preloadOrder(r.db.WithContext(ctx)).
		Where("id = ? AND user_id = ?", orderID, userID).First(&order).Error
	if err != nil {
		return model.Order{}, translate(err, "order not found")
	}
	return order, nil
}

func (r *CommerceRepository) FindOrderByID(ctx context.Context, orderID uint) (model.Order, error) {
	var order model.Order
	err := preloadOrder(r.db.WithContext(ctx)).First(&order, orderID).Error
	if err != nil {
		return model.Order{}, translate(err, "order not found")
	}
	return order, nil
}

func (r *CommerceRepository) ListOrdersForUser(ctx context.Context, userID uint, page, pageSize int) ([]model.Order, int64, error) {
	db := r.db.WithContext(ctx).Model(&model.Order{}).Where("user_id = ?", userID)
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, apierr.Wrap(apierr.CodeInternal, "could not count orders", err)
	}
	var orders []model.Order
	err := preloadOrder(db).Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&orders).Error
	if err != nil {
		return nil, 0, apierr.Wrap(apierr.CodeInternal, "could not load orders", err)
	}
	return orders, total, nil
}

func (r *CommerceRepository) LockOrderByID(ctx context.Context, orderID uint) (model.Order, error) {
	var order model.Order
	db := preloadOrder(r.db.WithContext(ctx))
	if r.Dialect() == "postgres" {
		db = db.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	if err := db.First(&order, orderID).Error; err != nil {
		return model.Order{}, translate(err, "order not found")
	}
	return order, nil
}

func (r *CommerceRepository) LockOrderByNo(ctx context.Context, orderNo string) (model.Order, error) {
	var order model.Order
	db := preloadOrder(r.db.WithContext(ctx))
	if r.Dialect() == "postgres" {
		db = db.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	if err := db.Where("order_no = ?", orderNo).First(&order).Error; err != nil {
		return model.Order{}, translate(err, "order not found")
	}
	return order, nil
}

func (r *CommerceRepository) SaveOrderAndHistory(ctx context.Context, order *model.Order, history *model.OrderStatusHistory) error {
	result := r.db.WithContext(ctx).Model(&model.Order{}).
		Where("id = ? AND version = ?", order.ID, order.Version-1).
		Updates(map[string]any{
			"status": order.Status, "version": order.Version, "paid_at": order.PaidAt, "cancelled_at": order.CancelledAt,
		})
	if result.Error != nil {
		return apierr.Wrap(apierr.CodeInternal, "could not update order", result.Error)
	}
	if result.RowsAffected != 1 {
		return apierr.Conflict("order changed concurrently")
	}
	if history != nil {
		history.OrderID = order.ID
		if err := r.db.WithContext(ctx).Create(history).Error; err != nil {
			return apierr.Wrap(apierr.CodeInternal, "could not append order history", err)
		}
	}
	return nil
}

func (r *CommerceRepository) CreatePayment(ctx context.Context, payment *model.Payment) error {
	if err := r.db.WithContext(ctx).Create(payment).Error; err != nil {
		return translate(err, "payment already exists")
	}
	return nil
}

func (r *CommerceRepository) FindPaymentByOrder(ctx context.Context, orderID uint) (model.Payment, error) {
	var payment model.Payment
	if err := r.db.WithContext(ctx).Where("order_id = ?", orderID).First(&payment).Error; err != nil {
		return model.Payment{}, translate(err, "payment not found")
	}
	return payment, nil
}

func (r *CommerceRepository) LockPaymentByReference(ctx context.Context, reference string) (model.Payment, error) {
	var payment model.Payment
	db := r.db.WithContext(ctx)
	if r.Dialect() == "postgres" {
		db = db.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	if err := db.Where("reference = ?", reference).First(&payment).Error; err != nil {
		return model.Payment{}, translate(err, "payment not found")
	}
	return payment, nil
}

func (r *CommerceRepository) SavePayment(ctx context.Context, payment *model.Payment) error {
	if err := r.db.WithContext(ctx).Save(payment).Error; err != nil {
		return apierr.Wrap(apierr.CodeInternal, "could not update payment", err)
	}
	return nil
}

func (r *CommerceRepository) RecordWebhookEvent(ctx context.Context, event *model.PaymentWebhookEvent) (bool, error) {
	result := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "provider"}, {Name: "event_id"}},
		DoNothing: true,
	}).Create(event)
	if result.Error != nil {
		return false, apierr.Wrap(apierr.CodeInternal, "could not record payment callback", result.Error)
	}
	return result.RowsAffected == 1, nil
}

func (r *CommerceRepository) FindWebhookEvent(ctx context.Context, provider, eventID string) (model.PaymentWebhookEvent, error) {
	var event model.PaymentWebhookEvent
	err := r.db.WithContext(ctx).Where("provider = ? AND event_id = ?", provider, eventID).First(&event).Error
	if err != nil {
		return model.PaymentWebhookEvent{}, translate(err, "payment callback not found")
	}
	return event, nil
}

// LockExpiredOrderIDs claims a batch for one worker. PostgreSQL's SKIP LOCKED
// lets replicas process disjoint rows without blocking one another.
func (r *CommerceRepository) LockExpiredOrderIDs(ctx context.Context, now time.Time, limit int) ([]uint, error) {
	var ids []uint
	db := r.db.WithContext(ctx).Model(&model.Order{}).
		Where("status = ? AND expires_at < ?", model.OrderPendingPayment, now).
		Order("expires_at ASC").Limit(limit)
	if r.Dialect() == "postgres" {
		db = db.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"})
	}
	if err := db.Pluck("id", &ids).Error; err != nil {
		return nil, apierr.Wrap(apierr.CodeInternal, "could not claim expired orders", err)
	}
	return ids, nil
}

func IsNotFound(err error) bool {
	var apiError *apierr.Error
	return errors.As(err, &apiError) && apiError.Code == apierr.CodeNotFound
}

func IsConflict(err error) bool {
	var apiError *apierr.Error
	return errors.As(err, &apiError) && apiError.Code == apierr.CodeConflict
}

func WrapTransaction(err error) error {
	if err == nil {
		return nil
	}
	var apiError *apierr.Error
	if errors.As(err, &apiError) {
		return err
	}
	return apierr.Wrap(apierr.CodeInternal, "commerce transaction failed", fmt.Errorf("transaction: %w", err))
}
