package controller

import (
	"huddle/pkg/model"
	"huddle/pkg/response"
	"huddle/pkg/services/cart"
	"huddle/pkg/utility"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type CartController struct {
	Db    *gorm.DB
	Redis *redis.Client
}

func (b *CartController) AddToCart(c *gin.Context) {
	var req model.AddCartItemRequest

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

	cookie, _ := c.Cookie("cart_session")

	sessionId, cartResp, serviceErr := cart.AddToCart(&req, c.GetString("X-Store-Id"), cookie, b.Db, b.Redis)
	if serviceErr != nil {
		response.Error(c, serviceErr.StatusCode, serviceErr.Message, serviceErr.Err)
		return
	}

	// Set session cookie if not present or updated
	c.SetCookie("cart_session", sessionId, 7*24*3600, "/", "", false, true)

	response.Success(c, "Item added to cart successfully", cartResp)
}

func (b *CartController) GetCart(c *gin.Context) {
	cookie, _ := c.Cookie("cart_session")

	cartResp, serviceErr := cart.GetCart(c.GetString("X-Store-Id"), cookie, b.Redis)
	if serviceErr != nil {
		response.Error(c, serviceErr.StatusCode, serviceErr.Message, serviceErr.Err)
		return
	}

	response.Success(c, "Cart retrieved successfully", cartResp)
}

func (b *CartController) UpdateCartItem(c *gin.Context) {
	productId := c.Param("id")
	if productId == "" {
		response.Error(c, http.StatusBadRequest, "Product ID is required", nil)
		return
	}

	var req model.UpdateCartItemRequest
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

	cookie, _ := c.Cookie("cart_session")

	cartResp, serviceErr := cart.UpdateCartItemQuantity(productId, req.Quantity, c.GetString("X-Store-Id"), cookie, b.Db, b.Redis)
	if serviceErr != nil {
		response.Error(c, serviceErr.StatusCode, serviceErr.Message, serviceErr.Err)
		return
	}

	response.Success(c, "Cart item updated successfully", cartResp)
}

func (b *CartController) RemoveCartItem(c *gin.Context) {
	productId := c.Param("id")
	if productId == "" {
		response.Error(c, http.StatusBadRequest, "Product ID is required", nil)
		return
	}

	cookie, _ := c.Cookie("cart_session")

	cartResp, serviceErr := cart.RemoveCartItem(productId, c.GetString("X-Store-Id"), cookie, b.Redis)
	if serviceErr != nil {
		response.Error(c, serviceErr.StatusCode, serviceErr.Message, serviceErr.Err)
		return
	}

	response.Success(c, "Item removed from cart successfully", cartResp)
}

func (b *CartController) ClearCart(c *gin.Context) {
	cookie, _ := c.Cookie("cart_session")

	serviceErr := cart.ClearCart(c.GetString("X-Store-Id"), cookie, b.Redis)
	if serviceErr != nil {
		response.Error(c, serviceErr.StatusCode, serviceErr.Message, serviceErr.Err)
		return
	}

	response.Success(c, "Shopping cart cleared successfully", nil)
}