package router

import (
	"huddle/pkg/controller"
	"huddle/pkg/middleware"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

func StoreRouter(r *gin.Engine, db *gorm.DB, rdb *redis.Client){
     storeBase := &controller.StoreBase{Db: db, Rdb: rdb}
	orders := &controller.OrderController{Db: db, Redis: rdb}
	payments := &controller.PaymentController{Db: db, Bachs: bachsClient()}

	storeRoutes := r.Group("/store")
	storeRoutes.Use(middleware.AuthMiddleware)
	
	{
		storeRoutes.POST("/create", storeBase.Create)
		storeRoutes.GET("/stores", storeBase.List)
		storeRoutes.GET("/:id", storeBase.Get)
		storeRoutes.PUT("/:id", storeBase.Update)
		storeRoutes.POST("/:id/key", storeBase.RegenerateKey)
		storeRoutes.GET("/:id/payments", payments.Setup)
		storeRoutes.POST("/:id/payments/onboarding", payments.Onboard)
		storeRoutes.GET("/:id/orders", orders.SellerOrders)
		storeRoutes.GET("/:id/orders/:orderId", orders.SellerOrder)
		storeRoutes.PATCH("/:id/orders/:orderId", orders.UpdateStatus)
	}

}