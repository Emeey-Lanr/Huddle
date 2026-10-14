package router

import (
	"huddle/pkg/bachs"
	"huddle/pkg/controller"
	"huddle/pkg/middleware"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// bachsClient is nil when Bachs is not configured; paying then returns 503.
func bachsClient() *bachs.Client {
	client, _ := bachs.FromEnv()
	return client
}

// WebhookRoutes receive events from payment providers. They are not behind
// the store key: each provider's signature is checked instead.
func WebhookRoutes(r *gin.Engine, db *gorm.DB) {
	payments := &controller.PaymentController{Db: db}
	r.POST("/webhooks/bachs", payments.BachsWebhook)
}

// StorefrontRoutes are public, read-only routes for storefront websites,
// identified by the store's publishable key in X-Store-Key.
func StorefrontRoutes(r *gin.Engine, db *gorm.DB, rdb *redis.Client) {
	storefront := &controller.StorefrontBase{Db: db}
	orders := &controller.OrderController{Db: db, Redis: rdb}
	payments := &controller.PaymentController{Db: db, Bachs: bachsClient()}
	group := r.Group("/storefront")
	group.Use(middleware.StoreKeyMiddleware(db))

	{
		group.GET("/store", storefront.Store)
		group.GET("/products", storefront.Products)
		group.GET("/products/:id", storefront.Product)
		// Guests can check out; a customer token, if sent, links the order to the account.
		group.POST("/checkout", middleware.OptionalCustomerAuth, orders.Checkout)
		group.GET("/orders/:id", orders.StorefrontOrder)
		group.POST("/orders/:id/pay", payments.Pay)
	}
}
