package httpapi

import (
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/fonghehe/vue-h5-template-business-service/internal/apierr"
	"github.com/fonghehe/vue-h5-template-business-service/internal/httpapi/middleware"
	"github.com/fonghehe/vue-h5-template-business-service/internal/response"
	"github.com/fonghehe/vue-h5-template-business-service/internal/service"
)

type cartItemRequest struct {
	SKUID    uint `json:"skuId" binding:"required"`
	Quantity int  `json:"quantity" binding:"required,gte=1,lte=99"`
}

type cartQuantityRequest struct {
	Quantity int `json:"quantity" binding:"required,gte=1,lte=99"`
}

func (s *Server) getCart(c *gin.Context) {
	items, err := s.services.Cart.List(c.Request.Context(), middleware.CurrentUserID(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	result := make([]CartItemDTO, len(items))
	for i, v := range items {
		result[i] = toCartItem(v)
	}
	response.OK(c, gin.H{"items": result})
}

func (s *Server) addCartItem(c *gin.Context) {
	var input cartItemRequest
	if err := bindJSON(c, &input, false, "Invalid request payload"); err != nil {
		response.Fail(c, apierr.Validation("skuId and quantity are required; quantity must be between 1 and 99"))
		return
	}
	item, err := s.services.Cart.Add(c.Request.Context(), middleware.CurrentUserID(c), input.SKUID, input.Quantity)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Created(c, toCartItem(item))
}

func (s *Server) updateCartItem(c *gin.Context) {
	id, err := pathID(c.Param("id"))
	if err != nil {
		response.Fail(c, apierr.Validation("id must be a positive integer"))
		return
	}
	var input cartQuantityRequest
	if err := bindJSON(c, &input, false, "Invalid request payload"); err != nil {
		response.Fail(c, apierr.Validation("quantity must be between 1 and 99"))
		return
	}
	item, err := s.services.Cart.Update(c.Request.Context(), middleware.CurrentUserID(c), id, input.Quantity)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, toCartItem(item))
}

func (s *Server) deleteCartItem(c *gin.Context) {
	id, err := pathID(c.Param("id"))
	if err != nil {
		response.Fail(c, apierr.Validation("id must be a positive integer"))
		return
	}
	if err := s.services.Cart.Delete(c.Request.Context(), middleware.CurrentUserID(c), id); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"deleted": true, "id": id})
}

func (s *Server) clearCart(c *gin.Context) {
	if err := s.services.Cart.Clear(c.Request.Context(), middleware.CurrentUserID(c)); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"cleared": true})
}

type createOrderRequest struct {
	CouponCode string `json:"couponCode" binding:"omitempty,max=64"`
}

func (s *Server) createOrder(c *gin.Context) {
	var input createOrderRequest
	if err := bindJSON(c, &input, true, "invalid order payload"); err != nil {
		response.Fail(c, err)
		return
	}

	order, replayed, err := s.services.Orders.Create(c.Request.Context(), middleware.CurrentUserID(c),
		c.GetHeader("Idempotency-Key"), service.CreateOrderInput{CouponCode: strings.TrimSpace(input.CouponCode)})
	if err != nil {
		response.Fail(c, err)
		return
	}
	if replayed {
		response.OK(c, toOrder(order))
		return
	}
	response.Created(c, toOrder(order))
}

func (s *Server) listOrders(c *gin.Context) {
	page, pageSize := pagination(c, 10, 50)
	orders, err := s.services.Orders.List(c.Request.Context(), middleware.CurrentUserID(c), page, pageSize)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, mapPage(orders, toOrder))
}

func (s *Server) getOrder(c *gin.Context) {
	id, err := pathID(c.Param("id"))
	if err != nil {
		response.Fail(c, apierr.Validation("id must be a positive integer"))
		return
	}
	order, err := s.services.Orders.Get(c.Request.Context(), middleware.CurrentUserID(c), id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, toOrder(order))
}

func (s *Server) cancelOrder(c *gin.Context) {
	id, err := pathID(c.Param("id"))
	if err != nil {
		response.Fail(c, apierr.Validation("id must be a positive integer"))
		return
	}
	order, err := s.services.Orders.Cancel(c.Request.Context(), middleware.CurrentUserID(c), id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, toOrder(order))
}

func (s *Server) createPayment(c *gin.Context) {
	id, err := pathID(c.Param("id"))
	if err != nil {
		response.Fail(c, apierr.Validation("id must be a positive integer"))
		return
	}
	payment, err := s.services.Payments.Create(c.Request.Context(), middleware.CurrentUserID(c), id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Created(c, toPayment(payment))
}

func (s *Server) mockPaymentWebhook(c *gin.Context) {
	var callback service.PaymentCallback
	if err := bindJSON(c, &callback, false, "Invalid payment callback"); err != nil {
		response.Fail(c, apierr.Validation("invalid payment callback"))
		return
	}
	order, duplicate, err := s.services.Payments.HandleWebhook(c.Request.Context(), callback, c.GetHeader("X-Mock-Signature"))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"order": toOrder(order), "duplicate": duplicate})
}

type orderStatusRequest struct {
	Status string `json:"status" binding:"required,oneof=PROCESSING COMPLETED"`
}

func (s *Server) adminAdvanceOrder(c *gin.Context) {
	id, err := pathID(c.Param("id"))
	if err != nil {
		response.Fail(c, apierr.Validation("id must be a positive integer"))
		return
	}
	var input orderStatusRequest
	if err := bindJSON(c, &input, false, "Invalid request payload"); err != nil {
		response.Fail(c, apierr.Validation("status must be PROCESSING or COMPLETED"))
		return
	}
	order, err := s.services.Orders.Advance(c.Request.Context(), id, input.Status)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, toOrder(order))
}
