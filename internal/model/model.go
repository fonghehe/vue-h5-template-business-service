// Package model holds the persisted entities of the business service.
//
// The JSON field names in this package are part of the public API contract:
// they mirror the schemas in the frontend's openapi/schema.yaml, so renaming
// one of them is a breaking change for every client.
package model

import (
	"time"

	"gorm.io/gorm"
)

// User roles recognised by the service.
const (
	RoleAdmin = "admin"
	RoleUser  = "user"
)

// Status values for catalog entities.
const (
	StatusDraft   = "draft"
	StatusOnSale  = "on_sale"
	StatusSoldOut = "sold_out"
)

// User is an account able to authenticate against the service.
type User struct {
	ID           uint      `gorm:"primaryKey"                                   json:"id"`
	Username     string    `gorm:"size:64;uniqueIndex;not null"                 json:"username"`
	PasswordHash string    `gorm:"size:255;not null"                            json:"-"`
	RealName     string    `gorm:"size:80;not null"                             json:"realName"`
	Avatar       string    `gorm:"size:500;not null"                            json:"avatar"`
	Roles        Roles     `gorm:"serializer:json;not null"                     json:"roles"`
	Status       string    `gorm:"size:24;not null;default:active"              json:"-"`
	CreatedAt    time.Time `json:"-"`
	UpdatedAt    time.Time `json:"-"`
}

// Roles is a persisted list of role names.
type Roles []string

// Has reports whether the user carries the given role.
func (r Roles) Has(role string) bool {
	for _, candidate := range r {
		if candidate == role {
			return true
		}
	}
	return false
}

// Public strips secret fields and returns the client-facing representation
// described by the User schema in openapi/schema.yaml.
func (u User) Public() User {
	u.PasswordHash = ""
	return u
}

// Product is a catalog item. Monetary values are stored as strings so that no
// rounding can occur between the database and the client.
type Product struct {
	ID uint `gorm:"primaryKey"                            json:"id"`
	// Name/CategoryID/Brand/Cover are the canonical commerce catalogue fields.
	// The legacy fields below remain in the response so existing vue-h5-template
	// clients keep working while new clients use the SKU-based contract.
	Name        string         `gorm:"size:255;not null;default:''"          json:"name"`
	CategoryID  string         `gorm:"size:64;index;not null;default:''"     json:"categoryId"`
	Brand       string         `gorm:"size:120;index;not null;default:''"    json:"brand"`
	Cover       string         `gorm:"size:500;not null;default:''"          json:"cover"`
	Title       string         `gorm:"size:255;not null"                     json:"title"`
	ImgURL      string         `gorm:"column:img_url;size:500;not null"      json:"imgUrl"`
	Price       string         `gorm:"size:32;not null"                      json:"price"`
	VipPrice    string         `gorm:"column:vip_price;size:32;not null"     json:"vipPrice"`
	ShopDesc    string         `gorm:"column:shop_desc;size:120;not null"    json:"shopDesc"`
	Delivery    string         `gorm:"size:60;not null"                      json:"delivery"`
	ShopName    string         `gorm:"column:shop_name;size:120;not null"    json:"shopName"`
	Description string         `gorm:"type:text;not null"                    json:"description"`
	Stock       int            `gorm:"not null;default:0"                    json:"-"`
	Sales       int            `gorm:"not null;default:0"                    json:"-"`
	Featured    bool           `gorm:"index;not null;default:false"          json:"-"`
	Status      string         `gorm:"size:24;index;not null;default:draft"  json:"-"`
	SearchText  string         `gorm:"type:text;not null"                    json:"-"`
	CreatedAt   time.Time      `json:"createdAt"`
	UpdatedAt   time.Time      `json:"updatedAt"`
	DeletedAt   gorm.DeletedAt `gorm:"index"                                 json:"-"`
	SKUs        []ProductSKU   `gorm:"foreignKey:ProductID"                  json:"skus,omitempty"`
}

// Favorite links a user to a product they bookmarked.
type Favorite struct {
	ID        uint      `gorm:"primaryKey"                            json:"id"`
	UserID    uint      `gorm:"uniqueIndex:idx_fav_user_product;not null" json:"userId"`
	ProductID uint      `gorm:"uniqueIndex:idx_fav_user_product;not null" json:"productId"`
	CreatedAt time.Time `json:"createdAt"`
}
