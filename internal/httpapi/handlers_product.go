package httpapi

import (
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/fonghehe/vue-h5-template-business-service/internal/apierr"
	"github.com/fonghehe/vue-h5-template-business-service/internal/httpapi/middleware"
	"github.com/fonghehe/vue-h5-template-business-service/internal/model"
	"github.com/fonghehe/vue-h5-template-business-service/internal/response"
	"github.com/fonghehe/vue-h5-template-business-service/internal/service"
)

// Limits for list endpoints. Page sizes are capped so that a single request can
// never pull the whole catalogue into memory.
const (
	defaultPageSize = 10
	maxPageSize     = 50
)

type favoriteRequest struct {
	ProductID uint `json:"productId" binding:"required"`
	Favorite  bool `json:"favorite"`
}

// listProducts returns one page of the public catalogue.
//
// GET /api/product/list?page=1&pageSize=10&keyword=...&sort=...
func (s *Server) listProducts(c *gin.Context) {
	page, pageSize := pagination(c, defaultPageSize, maxPageSize)
	result, err := s.services.Products.List(c.Request.Context(), service.ProductQuery{
		Page:     page,
		PageSize: pageSize,
		Keyword:  strings.TrimSpace(c.Query("keyword")),
		Sort:     strings.TrimSpace(c.Query("sort")),
	})
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, result)
}

// productDetail returns a single on-sale product.
//
// GET /api/product/detail?id=1
func (s *Server) productDetail(c *gin.Context) {
	id, err := pathID(c.Query("id"))
	if err != nil {
		response.Fail(c, apierr.Validation("id must be a positive integer"))
		return
	}
	product, err := s.services.Products.Detail(c.Request.Context(), id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, product)
}

// toggleFavorite bookmarks or unbookmarks a product.
//
// POST /api/product/favorite
func (s *Server) toggleFavorite(c *gin.Context) {
	var input favoriteRequest
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Fail(c, apierr.Validation("productId is required"))
		return
	}
	result, err := s.services.Favorites.Set(c.Request.Context(), middleware.CurrentUserID(c), input.ProductID, input.Favorite)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, result)
}

// adminListProducts returns a page of the catalogue including hidden items.
//
// GET /api/admin/products
func (s *Server) adminListProducts(c *gin.Context) {
	page, pageSize := pagination(c, defaultPageSize, maxPageSize)
	query := service.ProductQuery{
		Page:          page,
		PageSize:      pageSize,
		Keyword:       strings.TrimSpace(c.Query("keyword")),
		Sort:          strings.TrimSpace(c.Query("sort")),
		IncludeHidden: true,
		Status:        strings.TrimSpace(c.Query("status")),
	}
	if featured := strings.TrimSpace(c.Query("featured")); featured != "" {
		if value, err := strconv.ParseBool(featured); err == nil {
			query.Featured = &value
		}
	}
	result, err := s.services.Products.List(c.Request.Context(), query)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, result)
}

// adminGetProduct returns a product regardless of status.
//
// GET /api/admin/products/:id
func (s *Server) adminGetProduct(c *gin.Context) {
	id, err := pathID(c.Param("id"))
	if err != nil {
		response.Fail(c, apierr.Validation("id must be a positive integer"))
		return
	}
	product, err := s.services.Products.AdminDetail(c.Request.Context(), id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, product)
}

// adminCreateProduct adds a product to the catalogue.
//
// POST /api/admin/products
func (s *Server) adminCreateProduct(c *gin.Context) {
	var input productPayload
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Fail(c, apierr.Validation("invalid product payload"))
		return
	}
	product, err := s.services.Products.Create(c.Request.Context(), input.toService())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Created(c, product)
}

// adminUpdateProduct partially updates a product.
//
// PATCH /api/admin/products/:id
func (s *Server) adminUpdateProduct(c *gin.Context) {
	id, err := pathID(c.Param("id"))
	if err != nil {
		response.Fail(c, apierr.Validation("id must be a positive integer"))
		return
	}

	var input productPayload
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Fail(c, apierr.Validation("invalid product payload"))
		return
	}
	// A partial update must not blank out fields the operator did not send, so
	// missing values are filled from the stored record first.
	existing, err := s.services.Products.AdminDetail(c.Request.Context(), id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	product, err := s.services.Products.Update(c.Request.Context(), id, input.mergeInto(existing))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, product)
}

// adminDeleteProduct soft-removes a product.
//
// DELETE /api/admin/products/:id
func (s *Server) adminDeleteProduct(c *gin.Context) {
	id, err := pathID(c.Param("id"))
	if err != nil {
		response.Fail(c, apierr.Validation("id must be a positive integer"))
		return
	}
	if err := s.services.Products.Delete(c.Request.Context(), id); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"deleted": true, "id": id})
}

// productPayload is the operator-facing product representation. Pointers
// distinguish "field absent" from "field set to empty", which is what makes
// PATCH semantics correct.
type productPayload struct {
	Title       *string `json:"title"       binding:"omitempty,max=255"`
	ImgURL      *string `json:"imgUrl"      binding:"omitempty,max=500"`
	Price       *string `json:"price"       binding:"omitempty,max=32"`
	VipPrice    *string `json:"vipPrice"    binding:"omitempty,max=32"`
	ShopDesc    *string `json:"shopDesc"    binding:"omitempty,max=120"`
	Delivery    *string `json:"delivery"    binding:"omitempty,max=60"`
	ShopName    *string `json:"shopName"    binding:"omitempty,max=120"`
	Description *string `json:"description" binding:"omitempty,max=5000"`
	Stock       *int    `json:"stock"       binding:"omitempty,gte=0"`
	Status      *string `json:"status"      binding:"omitempty,oneof=draft on_sale sold_out"`
	Featured    *bool   `json:"featured"`
}

// toService converts the payload into the service input, using zero values for
// anything the caller omitted.
func (p productPayload) toService() service.ProductInput {
	return service.ProductInput{
		Title:       derefString(p.Title),
		ImgURL:      derefString(p.ImgURL),
		Price:       derefString(p.Price),
		VipPrice:    derefString(p.VipPrice),
		ShopDesc:    derefString(p.ShopDesc),
		Delivery:    derefString(p.Delivery),
		ShopName:    derefString(p.ShopName),
		Description: derefString(p.Description),
		Stock:       derefInt(p.Stock),
		Status:      derefString(p.Status),
		Featured:    derefBool(p.Featured),
	}
}

// mergeInto fills omitted fields from an existing product so that PATCH only
// changes what was actually sent.
func (p productPayload) mergeInto(existing model.Product) service.ProductInput {
	input := p.toService()
	if p.Title == nil {
		input.Title = existing.Title
	}
	if p.ImgURL == nil {
		input.ImgURL = existing.ImgURL
	}
	if p.Price == nil {
		input.Price = existing.Price
	}
	if p.VipPrice == nil {
		input.VipPrice = existing.VipPrice
	}
	if p.ShopDesc == nil {
		input.ShopDesc = existing.ShopDesc
	}
	if p.Delivery == nil {
		input.Delivery = existing.Delivery
	}
	if p.ShopName == nil {
		input.ShopName = existing.ShopName
	}
	if p.Description == nil {
		input.Description = existing.Description
	}
	if p.Stock == nil {
		input.Stock = existing.Stock
	}
	if p.Status == nil {
		input.Status = existing.Status
	}
	if p.Featured == nil {
		input.Featured = existing.Featured
	}
	return input
}

func derefString(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func derefInt(value *int) int {
	if value == nil {
		return 0
	}
	return *value
}

func derefBool(value *bool) bool {
	if value == nil {
		return false
	}
	return *value
}
