package model

import "time"

const (
	SKUStatusActive   = "active"
	SKUStatusInactive = "inactive"

	PromotionFixedDiscount     = "FIXED_DISCOUNT"
	PromotionPercentage        = "PERCENTAGE"
	PromotionThresholdDiscount = "THRESHOLD_DISCOUNT"

	CouponUsageReserved = "RESERVED"
	CouponUsageConsumed = "CONSUMED"
	CouponUsageReleased = "RELEASED"

	OrderPendingPayment = "PENDING_PAYMENT"
	OrderPaid           = "PAID"
	OrderProcessing     = "PROCESSING"
	OrderCompleted      = "COMPLETED"
	OrderCancelled      = "CANCELLED"
	OrderExpired        = "EXPIRED"

	PaymentPending   = "PENDING"
	PaymentSucceeded = "SUCCEEDED"
	PaymentFailed    = "FAILED"
)

// SKUAttributes is stored as PostgreSQL JSONB. Keys are stable attribute names
// (for example colour/storage) and values are the selected option.
type SKUAttributes map[string]string

// ProductSKU is the sellable unit. Price fields are integer minor units; the
// legacy string prices on Product are presentation compatibility only and are
// never trusted by checkout.
type ProductSKU struct {
	ID            uint          `gorm:"primaryKey"                                      json:"id"`
	ProductID     uint          `gorm:"index;index:idx_sku_product_status_price,priority:1;not null" json:"productId"`
	SKUCode       string        `gorm:"column:sku_code;size:80;uniqueIndex;not null"     json:"skuCode"`
	Name          string        `gorm:"size:255;not null"                               json:"name"`
	Attributes    SKUAttributes `gorm:"serializer:json;type:jsonb;not null"             json:"attributes"`
	Price         int64         `gorm:"index:idx_sku_product_status_price,priority:3;not null;check:chk_sku_price,price >= 0" json:"price"`
	OriginalPrice int64         `gorm:"not null;check:chk_sku_original_price,original_price >= 0" json:"originalPrice"`
	Status        string        `gorm:"size:24;index;index:idx_sku_product_status_price,priority:2;not null" json:"status"`
	CreatedAt     time.Time     `json:"createdAt"`
	UpdatedAt     time.Time     `json:"updatedAt"`
	Product       Product       `gorm:"foreignKey:ProductID" json:"-"`
	Inventory     *Inventory    `gorm:"foreignKey:SKUID;references:ID" json:"inventory,omitempty"`
}

// Inventory is authoritative stock for one SKU. Available is stock that may
// still be reserved; Reserved belongs to pending orders. Version is advanced
// by every mutation for auditability and optimistic read clients.
type Inventory struct {
	SKUID     uint      `gorm:"column:sku_id;primaryKey"                                  json:"skuId"`
	Available int64     `gorm:"not null;check:chk_inventory_available,available >= 0"     json:"available"`
	Reserved  int64     `gorm:"not null;check:chk_inventory_reserved,reserved >= 0"       json:"reserved"`
	Version   int64     `gorm:"not null;default:1;check:chk_inventory_version,version > 0" json:"version"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// CartItem stores intent only. Price is deliberately absent: checkout always
// reloads the current SKU price inside the order transaction.
type CartItem struct {
	ID        uint       `gorm:"primaryKey"                                              json:"id"`
	UserID    uint       `gorm:"uniqueIndex:idx_cart_user_sku;index;not null"            json:"userId"`
	SKUID     uint       `gorm:"column:sku_id;uniqueIndex:idx_cart_user_sku;not null"    json:"skuId"`
	Quantity  int        `gorm:"not null;check:chk_cart_quantity,quantity > 0"           json:"quantity"`
	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
	SKU       ProductSKU `gorm:"foreignKey:SKUID"                                       json:"sku"`
}

// Coupon is intentionally small: enough for real fixed, percentage and
// threshold discounts without turning the service into a marketing platform.
// Percentage Value uses basis points: 1000 means 10%, 10000 means 100%.
type Coupon struct {
	ID            uint      `gorm:"primaryKey"                                      json:"id"`
	Code          string    `gorm:"size:64;uniqueIndex;not null"                    json:"code"`
	Type          string    `gorm:"size:32;index;not null"                          json:"type"`
	Value         int64     `gorm:"not null;check:chk_coupon_value,value > 0"        json:"value"`
	MinimumAmount int64     `gorm:"not null;default:0;check:chk_coupon_minimum,minimum_amount >= 0" json:"minimumAmount"`
	StartAt       time.Time `gorm:"index;not null"                                  json:"startAt"`
	EndAt         time.Time `gorm:"index;not null"                                  json:"endAt"`
	UsageLimit    int       `gorm:"not null;check:chk_coupon_usage_limit,usage_limit > 0" json:"usageLimit"`
	PerUserLimit  int       `gorm:"not null;check:chk_coupon_user_limit,per_user_limit > 0" json:"perUserLimit"`
	// UserID makes a coupon account-bound when non-nil. A nil value means the
	// code is public, while all usage is still counted per authenticated user.
	UserID    *uint     `gorm:"index" json:"userId,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// CouponUsage reserves a coupon while an order awaits payment. Cancellation
// and expiration release it; successful payment consumes it.
type CouponUsage struct {
	ID        uint      `gorm:"primaryKey"                                         json:"id"`
	CouponID  uint      `gorm:"uniqueIndex:idx_coupon_order;index;not null"        json:"couponId"`
	UserID    uint      `gorm:"index;not null"                                     json:"userId"`
	OrderID   uint      `gorm:"uniqueIndex:idx_coupon_order;uniqueIndex;not null"  json:"orderId"`
	Status    string    `gorm:"size:24;index;not null"                             json:"status"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// Order is the aggregate root for checkout. IdempotencyKey is unique per user
// and RequestHash prevents accidental reuse of a key for a different cart.
type Order struct {
	ID             uint                 `gorm:"primaryKey" json:"id"`
	OrderNo        string               `gorm:"column:order_no;size:40;uniqueIndex;not null" json:"orderNo"`
	UserID         uint                 `gorm:"uniqueIndex:idx_order_user_idempotency;index;not null" json:"userId"`
	Status         string               `gorm:"size:32;index;index:idx_order_expiration,priority:1;not null" json:"status"`
	OriginalAmount int64                `gorm:"not null;check:chk_order_original,original_amount >= 0" json:"originalAmount"`
	DiscountAmount int64                `gorm:"not null;check:chk_order_discount,discount_amount >= 0 AND discount_amount <= original_amount" json:"discountAmount"`
	PayableAmount  int64                `gorm:"not null;check:chk_order_payable,payable_amount >= 0 AND payable_amount = original_amount - discount_amount" json:"payableAmount"`
	CouponID       *uint                `gorm:"index" json:"couponId,omitempty"`
	IdempotencyKey string               `gorm:"size:128;uniqueIndex:idx_order_user_idempotency;not null" json:"-"`
	RequestHash    string               `gorm:"size:64;not null" json:"-"`
	CreatedAt      time.Time            `gorm:"index" json:"createdAt"`
	UpdatedAt      time.Time            `json:"updatedAt"`
	ExpiresAt      time.Time            `gorm:"index:idx_order_expiration,priority:2;not null" json:"expiresAt"`
	PaidAt         *time.Time           `json:"paidAt,omitempty"`
	CancelledAt    *time.Time           `json:"cancelledAt,omitempty"`
	Version        int64                `gorm:"not null;default:1" json:"version"`
	Items          []OrderItem          `gorm:"foreignKey:OrderID" json:"items,omitempty"`
	StatusHistory  []OrderStatusHistory `gorm:"foreignKey:OrderID" json:"statusHistory,omitempty"`
}

// OrderItem is an immutable snapshot. Historical orders never depend on the
// current product name, SKU attributes or price.
type OrderItem struct {
	ID          uint   `gorm:"primaryKey" json:"id"`
	OrderID     uint   `gorm:"index;not null" json:"orderId"`
	ProductID   uint   `gorm:"index;not null" json:"productId"`
	SKUID       uint   `gorm:"column:sku_id;index;not null" json:"skuId"`
	ProductName string `gorm:"size:255;not null" json:"productName"`
	SKUName     string `gorm:"column:sku_name;size:255;not null" json:"skuName"`
	UnitPrice   int64  `gorm:"not null;check:chk_order_item_price,unit_price >= 0" json:"unitPrice"`
	Quantity    int    `gorm:"not null;check:chk_order_item_quantity,quantity > 0" json:"quantity"`
}

type OrderStatusHistory struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	OrderID    uint      `gorm:"index;not null" json:"orderId"`
	FromStatus string    `gorm:"size:32;not null" json:"fromStatus"`
	ToStatus   string    `gorm:"size:32;not null" json:"toStatus"`
	Reason     string    `gorm:"size:255;not null" json:"reason"`
	CreatedAt  time.Time `json:"createdAt"`
}

type Payment struct {
	ID            uint       `gorm:"primaryKey" json:"id"`
	OrderID       uint       `gorm:"uniqueIndex;not null" json:"orderId"`
	Provider      string     `gorm:"size:40;not null" json:"provider"`
	Reference     string     `gorm:"size:100;uniqueIndex;not null" json:"reference"`
	Amount        int64      `gorm:"not null;check:chk_payment_amount,amount >= 0" json:"amount"`
	Status        string     `gorm:"size:24;index;not null" json:"status"`
	FailureReason string     `gorm:"size:255;not null;default:''" json:"failureReason,omitempty"`
	PaidAt        *time.Time `json:"paidAt,omitempty"`
	CreatedAt     time.Time  `json:"createdAt"`
	UpdatedAt     time.Time  `json:"updatedAt"`
}

// PaymentWebhookEvent is the durable deduplication boundary. Redis may speed
// this up later, but correctness comes from this database unique constraint.
type PaymentWebhookEvent struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Provider    string    `gorm:"size:40;uniqueIndex:idx_webhook_provider_event;not null" json:"provider"`
	EventID     string    `gorm:"size:100;uniqueIndex:idx_webhook_provider_event;not null" json:"eventId"`
	PaymentID   uint      `gorm:"index;not null" json:"paymentId"`
	PayloadHash string    `gorm:"size:64;not null" json:"-"`
	CreatedAt   time.Time `json:"createdAt"`
}
