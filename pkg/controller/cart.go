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
	var req model.CartItem

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

	sessionId, cartResp, serviceErr := cart.AddToCart(&req, cookie, b.Db, b.Redis)
	if serviceErr != nil {
		response.Error(c, serviceErr.StatusCode, serviceErr.Message, serviceErr.Err)
		return
	}

	// Set session cookie if not present or updated
	c.SetCookie("cart_session", sessionId, 7*24*3600, "/", "", false, true)

	response.Success(c, "Item added to cart successfully", cartResp)
}