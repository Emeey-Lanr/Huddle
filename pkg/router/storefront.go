package router

import (
	"huddle/pkg/controller"
	"huddle/pkg/middleware"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// StorefrontRoutes are public, read-only routes for storefront websites,
// identified by the store's publishable key in X-Store-Key.
func StorefrontRoutes(r *gin.Engine, db *gorm.DB, rdb *redis.Client) {
	storefront := &controller.StorefrontBase{Db: db}
	orders := &controller.OrderController{Db: db, Redis: rdb}
	group := r.Group("/storefront")
	group.Use(middleware.StoreKeyMiddleware(db))

	{
		group.GET("/store", storefront.Store)
		group.GET("/products", storefront.Products)
		group.GET("/products/:id", storefront.Product)
		// Guests can check out; a customer token, if sent, links the order to the account.
		group.POST("/checkout", middleware.OptionalCustomerAuth, orders.Checkout)
		group.GET("/orders/:id", orders.StorefrontOrder)
	}
}
