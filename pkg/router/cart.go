package router

import (
	"huddle/pkg/controller"
	"huddle/pkg/middleware"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

func CartRoutes(r *gin.Engine, db *gorm.DB, rdb *redis.Client) {
	cartController := &controller.CartController{Db: db, Redis: rdb}
	cartGroup := r.Group("/cart")
	// Every cart call names its store with the X-Store-Id header.
	cartGroup.Use(middleware.StoreContextMiddleware)

	{
		cartGroup.POST("/", cartController.AddToCart)
		cartGroup.GET("/", cartController.GetCart)
		cartGroup.PUT("/items/:id", cartController.UpdateCartItem)
		cartGroup.DELETE("/items/:id", cartController.RemoveCartItem)
		cartGroup.DELETE("/", cartController.ClearCart)
	}
}

