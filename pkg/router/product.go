package router

import (
	"huddle/pkg/controller"
	"huddle/pkg/middleware"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)


func ProductRouter ( r *gin.Engine,db *gorm.DB, rdb *redis.Client) {
     
	 productRoutes := r.Group("/product")
	 productRoutes.Use(middleware.AuthMiddleware)

	 productsController := controller.ProductBase{Db: db, Rdb: rdb}
	 {
		productRoutes.POST("/", middleware.StoreContextMiddleware, productsController.Add)
		productRoutes.GET("/")
		productRoutes.GET("/:id")
		productRoutes.PUT("/:id")
		productRoutes.DELETE("/:id")
		
	 }
 
	//  Images

	 {
		productRoutes.POST("/:id/images", productsController.AddImages)
	
	 }
}