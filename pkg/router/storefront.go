package router

import (
	"huddle/pkg/controller"
	"huddle/pkg/middleware"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// StorefrontRoutes are public, read-only routes for storefront websites,
// identified by the store's publishable key in X-Store-Key.
func StorefrontRoutes(r *gin.Engine, db *gorm.DB) {
	storefront := &controller.StorefrontBase{Db: db}
	group := r.Group("/storefront")
	group.Use(middleware.StoreKeyMiddleware(db))

	{
		group.GET("/store", storefront.Store)
		group.GET("/products", storefront.Products)
		group.GET("/products/:id", storefront.Product)
	}
}
