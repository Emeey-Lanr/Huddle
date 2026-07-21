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
	customerGroup := r.Group("/customer")

	{
		customerGroup.POST("/signup", customerBase.Signup)
		customerGroup.POST("/signin", customerBase.Signin)
		customerGroup.GET("/profile", middleware.AuthMiddleware, customerBase.GetProfile)
	}
}