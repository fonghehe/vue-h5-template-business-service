package httpapi

import (
	"io"
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
	response.OK(c, gin.H{"items": items})
}

func (s *Server) addCartItem(c *gin.Context) {
	var input cartItemRequest
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Fail(c, apierr.Validation("skuId and quantity are required; quantity must be between 1 and 99"))
		return
	}
	item, err := s.services.Cart.Add(c.Request.Context(), middleware.CurrentUserID(c), input.SKUID, input.Quantity)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Created(c, item)
}

func (s *Server) updateCartItem(c *gin.Context) {
	id, err := pathID(c.Param("id"))
	if err != nil {
		response.Fail(c, apierr.Validation("id must be a positive integer"))
		return
	}
	var input cartQuantityRequest
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Fail(c, apierr.Validation("quantity must be between 1 and 99"))
		return
	}
	item, err := s.services.Cart.Update(c.Request.Context(), middleware.CurrentUserID(c), id, input.Quantity)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, item)
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
	if c.Request.ContentLength != 0 {
		if err := c.ShouldBindJSON(&input); err != nil && err != io.EOF {
			response.Fail(c, apierr.Validation("invalid order payload"))
			return
		}
	}
	order, replayed, err := s.services.Orders.Create(c.Request.Context(), middleware.CurrentUserID(c),
		c.GetHeader("Idempotency-Key"), service.CreateOrderInput{CouponCode: strings.TrimSpace(input.CouponCode)})
	if err != nil {
		response.Fail(c, err)
		return
	}
	if replayed {
		response.OK(c, order)
		return
	}
	response.Created(c, order)
}

func (s *Server) listOrders(c *gin.Context) {
	page, pageSize := pagination(c, 10, 50)
	orders, err := s.services.Orders.List(c.Request.Context(), middleware.CurrentUserID(c), page, pageSize)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, orders)
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
	response.OK(c, order)
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
	response.OK(c, order)
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
	response.Created(c, payment)
}

func (s *Server) mockPaymentWebhook(c *gin.Context) {
	var callback service.PaymentCallback
	if err := c.ShouldBindJSON(&callback); err != nil {
		response.Fail(c, apierr.Validation("invalid payment callback"))
		return
	}
	order, duplicate, err := s.services.Payments.HandleWebhook(c.Request.Context(), callback, c.GetHeader("X-Mock-Signature"))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"order": order, "duplicate": duplicate})
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
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Fail(c, apierr.Validation("status must be PROCESSING or COMPLETED"))
		return
	}
	order, err := s.services.Orders.Advance(c.Request.Context(), id, input.Status)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, order)
}
