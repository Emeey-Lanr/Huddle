package router

import (
	"huddle/pkg/controller"
	"huddle/pkg/middleware"
	"huddle/pkg/storedomain"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

func CartRoutes(r *gin.Engine, db *gorm.DB, rdb *redis.Client, domains *storedomain.Cache) {
	cartController := &controller.CartController{Db: db, Redis: rdb}
	cartGroup := r.Group("/cart")
	// Storefronts identify the store with its publishable key (X-Store-Key).
	// Paths have no trailing slash: browsers do not follow the trailing-slash
	// redirect during a CORS request.
	cartGroup.Use(middleware.StoreKeyMiddleware(db, domains))

	{
		cartGroup.POST("", cartController.AddToCart)
		cartGroup.GET("", cartController.GetCart)
		cartGroup.PUT("/items/:id", cartController.UpdateCartItem)
		cartGroup.DELETE("/items/:id", cartController.RemoveCartItem)
		cartGroup.DELETE("", cartController.ClearCart)
	}
}

