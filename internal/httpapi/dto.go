// Explicit transport DTOs keep persistence changes out of the public JSON
// contract. Legacy model JSON tags remain for cache compatibility only.
package httpapi

import (
	"time"

	"github.com/fonghehe/vue-h5-template-business-service/internal/model"
	"github.com/fonghehe/vue-h5-template-business-service/internal/service"
)

type UserDTO struct {
	ID       uint     `json:"id"`
	Username string   `json:"username"`
	RealName string   `json:"realName"`
	Avatar   string   `json:"avatar"`
	Roles    []string `json:"roles"`
}

func toUser(value model.User) UserDTO {
	result := UserDTO{
		ID:       value.ID,
		Username: value.Username,
		RealName: value.RealName,
		Avatar:   value.Avatar,
		Roles:    value.Roles,
	}
	return result
}

type ProductDTO struct {
	ID          uint            `json:"id"`
	Name        string          `json:"name"`
	CategoryID  string          `json:"categoryId"`
	Brand       string          `json:"brand"`
	Cover       string          `json:"cover"`
	Title       string          `json:"title"`
	ImgURL      string          `json:"imgUrl"`
	Price       string          `json:"price"`
	VipPrice    string          `json:"vipPrice"`
	ShopDesc    string          `json:"shopDesc"`
	Delivery    string          `json:"delivery"`
	ShopName    string          `json:"shopName"`
	Description string          `json:"description"`
	CreatedAt   time.Time       `json:"createdAt"`
	UpdatedAt   time.Time       `json:"updatedAt"`
	SKUs        []ProductSKUDTO `json:"skus,omitempty"`
}

func toProduct(value model.Product) ProductDTO {
	result := ProductDTO{
		ID:          value.ID,
		Name:        value.Name,
		CategoryID:  value.CategoryID,
		Brand:       value.Brand,
		Cover:       value.Cover,
		Title:       value.Title,
		ImgURL:      value.ImgURL,
		Price:       value.Price,
		VipPrice:    value.VipPrice,
		ShopDesc:    value.ShopDesc,
		Delivery:    value.Delivery,
		ShopName:    value.ShopName,
		Description: value.Description,
		CreatedAt:   value.CreatedAt,
		UpdatedAt:   value.UpdatedAt,
	}
	if value.SKUs != nil {
		result.SKUs = make([]ProductSKUDTO, len(value.SKUs))
		for i, v := range value.SKUs {
			result.SKUs[i] = toProductSKU(v)
		}
	}
	return result
}

type ProductSKUDTO struct {
	ID            uint              `json:"id"`
	ProductID     uint              `json:"productId"`
	SKUCode       string            `json:"skuCode"`
	Name          string            `json:"name"`
	Attributes    map[string]string `json:"attributes"`
	Price         int64             `json:"price"`
	OriginalPrice int64             `json:"originalPrice"`
	Status        string            `json:"status"`
	CreatedAt     time.Time         `json:"createdAt"`
	UpdatedAt     time.Time         `json:"updatedAt"`
	Inventory     *InventoryDTO     `json:"inventory,omitempty"`
}

func toProductSKU(value model.ProductSKU) ProductSKUDTO {
	result := ProductSKUDTO{
		ID:            value.ID,
		ProductID:     value.ProductID,
		SKUCode:       value.SKUCode,
		Name:          value.Name,
		Attributes:    value.Attributes,
		Price:         value.Price,
		OriginalPrice: value.OriginalPrice,
		Status:        value.Status,
		CreatedAt:     value.CreatedAt,
		UpdatedAt:     value.UpdatedAt,
	}
	if value.Inventory != nil {
		v := toInventory(*value.Inventory)
		result.Inventory = &v
	}
	return result
}

type InventoryDTO struct {
	SKUID     uint      `json:"skuId"`
	Available int64     `json:"available"`
	Reserved  int64     `json:"reserved"`
	Version   int64     `json:"version"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func toInventory(value model.Inventory) InventoryDTO {
	result := InventoryDTO{
		SKUID:     value.SKUID,
		Available: value.Available,
		Reserved:  value.Reserved,
		Version:   value.Version,
		CreatedAt: value.CreatedAt,
		UpdatedAt: value.UpdatedAt,
	}
	return result
}

type CartItemDTO struct {
	ID        uint          `json:"id"`
	UserID    uint          `json:"userId"`
	SKUID     uint          `json:"skuId"`
	Quantity  int           `json:"quantity"`
	CreatedAt time.Time     `json:"createdAt"`
	UpdatedAt time.Time     `json:"updatedAt"`
	SKU       ProductSKUDTO `json:"sku"`
}

func toCartItem(value model.CartItem) CartItemDTO {
	result := CartItemDTO{
		ID:        value.ID,
		UserID:    value.UserID,
		SKUID:     value.SKUID,
		Quantity:  value.Quantity,
		CreatedAt: value.CreatedAt,
		UpdatedAt: value.UpdatedAt,
		SKU:       toProductSKU(value.SKU),
	}
	return result
}

type OrderDTO struct {
	ID             uint                    `json:"id"`
	OrderNo        string                  `json:"orderNo"`
	UserID         uint                    `json:"userId"`
	Status         string                  `json:"status"`
	OriginalAmount int64                   `json:"originalAmount"`
	DiscountAmount int64                   `json:"discountAmount"`
	PayableAmount  int64                   `json:"payableAmount"`
	CouponID       *uint                   `json:"couponId,omitempty"`
	CreatedAt      time.Time               `json:"createdAt"`
	UpdatedAt      time.Time               `json:"updatedAt"`
	ExpiresAt      time.Time               `json:"expiresAt"`
	PaidAt         *time.Time              `json:"paidAt,omitempty"`
	CancelledAt    *time.Time              `json:"cancelledAt,omitempty"`
	Version        int64                   `json:"version"`
	Items          []OrderItemDTO          `json:"items,omitempty"`
	StatusHistory  []OrderStatusHistoryDTO `json:"statusHistory,omitempty"`
}

func toOrder(value model.Order) OrderDTO {
	result := OrderDTO{
		ID:             value.ID,
		OrderNo:        value.OrderNo,
		UserID:         value.UserID,
		Status:         value.Status,
		OriginalAmount: value.OriginalAmount,
		DiscountAmount: value.DiscountAmount,
		PayableAmount:  value.PayableAmount,
		CouponID:       value.CouponID,
		CreatedAt:      value.CreatedAt,
		UpdatedAt:      value.UpdatedAt,
		ExpiresAt:      value.ExpiresAt,
		PaidAt:         value.PaidAt,
		CancelledAt:    value.CancelledAt,
		Version:        value.Version,
	}
	if value.Items != nil {
		result.Items = make([]OrderItemDTO, len(value.Items))
		for i, v := range value.Items {
			result.Items[i] = toOrderItem(v)
		}
	}
	if value.StatusHistory != nil {
		result.StatusHistory = make([]OrderStatusHistoryDTO, len(value.StatusHistory))
		for i, v := range value.StatusHistory {
			result.StatusHistory[i] = toOrderStatusHistory(v)
		}
	}
	return result
}

type OrderItemDTO struct {
	ID          uint   `json:"id"`
	OrderID     uint   `json:"orderId"`
	ProductID   uint   `json:"productId"`
	SKUID       uint   `json:"skuId"`
	ProductName string `json:"productName"`
	SKUName     string `json:"skuName"`
	UnitPrice   int64  `json:"unitPrice"`
	Quantity    int    `json:"quantity"`
}

func toOrderItem(value model.OrderItem) OrderItemDTO {
	result := OrderItemDTO{
		ID:          value.ID,
		OrderID:     value.OrderID,
		ProductID:   value.ProductID,
		SKUID:       value.SKUID,
		ProductName: value.ProductName,
		SKUName:     value.SKUName,
		UnitPrice:   value.UnitPrice,
		Quantity:    value.Quantity,
	}
	return result
}

type OrderStatusHistoryDTO struct {
	ID         uint      `json:"id"`
	OrderID    uint      `json:"orderId"`
	FromStatus string    `json:"fromStatus"`
	ToStatus   string    `json:"toStatus"`
	Reason     string    `json:"reason"`
	CreatedAt  time.Time `json:"createdAt"`
}

func toOrderStatusHistory(value model.OrderStatusHistory) OrderStatusHistoryDTO {
	result := OrderStatusHistoryDTO{
		ID:         value.ID,
		OrderID:    value.OrderID,
		FromStatus: value.FromStatus,
		ToStatus:   value.ToStatus,
		Reason:     value.Reason,
		CreatedAt:  value.CreatedAt,
	}
	return result
}

type PaymentDTO struct {
	ID            uint       `json:"id"`
	OrderID       uint       `json:"orderId"`
	Provider      string     `json:"provider"`
	Reference     string     `json:"reference"`
	Amount        int64      `json:"amount"`
	Status        string     `json:"status"`
	FailureReason string     `json:"failureReason,omitempty"`
	PaidAt        *time.Time `json:"paidAt,omitempty"`
	CreatedAt     time.Time  `json:"createdAt"`
	UpdatedAt     time.Time  `json:"updatedAt"`
}

func toPayment(value model.Payment) PaymentDTO {
	result := PaymentDTO{
		ID:            value.ID,
		OrderID:       value.OrderID,
		Provider:      value.Provider,
		Reference:     value.Reference,
		Amount:        value.Amount,
		Status:        value.Status,
		FailureReason: value.FailureReason,
		PaidAt:        value.PaidAt,
		CreatedAt:     value.CreatedAt,
		UpdatedAt:     value.UpdatedAt,
	}
	return result
}

type LoginDTO struct {
	UserDTO
	AccessToken string `json:"accessToken"`
	ExpiresIn   int64  `json:"expiresIn"`
}

func toLogin(value service.LoginResult) LoginDTO {
	return LoginDTO{UserDTO: toUser(value.User), AccessToken: value.AccessToken, ExpiresIn: value.ExpiresIn}
}
func mapPage[I, O any](page service.Page[I], convert func(I) O) service.Page[O] {
	items := make([]O, len(page.Items))
	for i, v := range page.Items {
		items[i] = convert(v)
	}
	return service.Page[O]{Items: items, Total: page.Total, Page: page.Page, PageSize: page.PageSize, HasMore: page.HasMore}
}
