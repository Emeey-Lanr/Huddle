package router

import (
	"huddle/pkg/controller"
	"huddle/pkg/middleware"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

func CustomerRoutes(r *gin.Engine, db *gorm.DB, rdb *redis.Client) {
	customerBase := &controller.CustomerBase{DB: db, Redis: rdb}
	orders := &controller.OrderController{Db: db, Redis: rdb}
	customerGroup := r.Group("/customer")

	{
		customerGroup.POST("/signup", middleware.RateLimiter, customerBase.Signup)
		customerGroup.POST("/signin", middleware.RateLimiter, customerBase.Signin)
		customerGroup.GET("/profile", middleware.CustomerAuthMiddleware, customerBase.GetProfile)
		customerGroup.GET("/orders", middleware.CustomerAuthMiddleware, orders.CustomerOrders)
	}
}