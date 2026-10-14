package controller

import (
	"huddle/pkg/audit"
	"huddle/pkg/model"
	"huddle/pkg/response"
	"huddle/pkg/services/order"
	"huddle/pkg/utility"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type OrderController struct {
	Db    *gorm.DB
	Redis *redis.Client
}

// Checkout places an order from the shopper's cart. Guests can check out;
// a signed-in customer's order is linked to their account.
func (b *OrderController) Checkout(c *gin.Context) {
	var req model.CheckoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid request payload", err)
		return
	}
	if err := utility.Validate.Struct(&req); err != nil {
		if validationErr := utility.ValidationErrors(err); validationErr != nil {
			response.Error(c, http.StatusBadRequest, "Validation failed", validationErr)
			return
		}
	}

	var customerId *uuid.UUID
	if id, ok := c.Get("customerId"); ok {
		parsed, err := utility.StringToUUID(id.(string))
		if err != nil {
			response.Error(c, http.StatusUnauthorized, "Invalid customer token", err)
			return
		}
		customerId = &parsed
	}

	session, _ := c.Cookie("cart_session")
	storePublicId := c.GetString("X-Store-Id")
	resp, serviceErr := order.Checkout(b.Db, b.Redis, storeFromKey(c), storePublicId, session, customerId, c.GetString("email"), req)
	if serviceErr != nil {
		response.Error(c, serviceErr.StatusCode, serviceErr.Message, serviceErr.Err)
		return
	}

	audit.LogEvent(c.Request.Context(), "ORDER_PLACED", storePublicId, resp.Id, "SUCCESS", map[string]interface{}{"total": resp.Total.String(), "currency": resp.Currency})
	response.Created(c, "Order placed successfully", resp)
}

// StorefrontOrder shows one order of this store; the order id is the receipt link.
func (b *OrderController) StorefrontOrder(c *gin.Context) {
	resp, serviceErr := order.GetStorefrontOrder(b.Db, storeFromKey(c), c.Param("id"))
	if serviceErr != nil {
		response.Error(c, serviceErr.StatusCode, serviceErr.Message, serviceErr.Err)
		return
	}
	response.Success(c, "Order retrieved successfully", resp)
}

// CustomerOrders lists the signed-in customer's orders.
func (b *OrderController) CustomerOrders(c *gin.Context) {
	id, _ := c.Get("customerId")
	customerId, err := utility.StringToUUID(id.(string))
	if err != nil {
		response.Error(c, http.StatusUnauthorized, "Invalid customer token", err)
		return
	}

	var q model.OrderQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid query parameters", err)
		return
	}
	resp, serviceErr := order.ListCustomerOrders(b.Db, customerId, q)
	if serviceErr != nil {
		response.Error(c, serviceErr.StatusCode, serviceErr.Message, serviceErr.Err)
		return
	}
	response.Success(c, "Orders retrieved successfully", resp)
}

// SellerOrders lists a store's orders for its owner.
func (b *OrderController) SellerOrders(c *gin.Context) {
	adminId, ok := adminUUID(c)
	if !ok {
		return
	}
	var q model.OrderQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid query parameters", err)
		return
	}
	resp, serviceErr := order.ListSellerOrders(b.Db, c.Param("id"), adminId, q)
	if serviceErr != nil {
		response.Error(c, serviceErr.StatusCode, serviceErr.Message, serviceErr.Err)
		return
	}
	response.Success(c, "Orders retrieved successfully", resp)
}

func (b *OrderController) SellerOrder(c *gin.Context) {
	adminId, ok := adminUUID(c)
	if !ok {
		return
	}
	resp, serviceErr := order.GetSellerOrder(b.Db, c.Param("id"), adminId, c.Param("orderId"))
	if serviceErr != nil {
		response.Error(c, serviceErr.StatusCode, serviceErr.Message, serviceErr.Err)
		return
	}
	response.Success(c, "Order retrieved successfully", resp)
}

// UpdateStatus lets the owner mark an order paid, fulfilled or cancelled.
func (b *OrderController) UpdateStatus(c *gin.Context) {
	adminId, ok := adminUUID(c)
	if !ok {
		return
	}
	var req model.OrderStatusUpdate
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid request payload", err)
		return
	}
	if err := utility.Validate.Struct(&req); err != nil {
		if validationErr := utility.ValidationErrors(err); validationErr != nil {
			response.Error(c, http.StatusBadRequest, "Validation failed", validationErr)
			return
		}
	}

	resp, serviceErr := order.UpdateOrderStatus(b.Db, c.Param("id"), adminId, c.Param("orderId"), req.Status)
	if serviceErr != nil {
		audit.LogEvent(c.Request.Context(), "ORDER_STATUS_CHANGED", adminId.String(), c.Param("orderId"), "FAILED", map[string]interface{}{"status": req.Status, "error": serviceErr.Message})
		response.Error(c, serviceErr.StatusCode, serviceErr.Message, serviceErr.Err)
		return
	}
	audit.LogEvent(c.Request.Context(), "ORDER_STATUS_CHANGED", adminId.String(), resp.Id, "SUCCESS", map[string]interface{}{"status": resp.Status})
	response.Success(c, "Order updated successfully", resp)
}
