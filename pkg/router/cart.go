package router

import (
	"huddle/pkg/controller"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

func CartRoutes(r *gin.Engine, db *gorm.DB, rdb *redis.Client) {
	cartController := &controller.CartController{Db: db, Redis: rdb}
	cartGroup := r.Group("/cart")

	{
		cartGroup.POST("/", cartController.AddToCart)
	}
}
