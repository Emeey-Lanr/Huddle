package router

import (
	"huddle/doc"
	"huddle/pkg/middleware"
	"huddle/pkg/storedomain"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

func RoutesSetup(db *gorm.DB, rdb *redis.Client) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.AuditLogMiddleware())
	// Which store has which custom domain, shared by CORS and the store key check.
	domains := storedomain.NewCache(db, time.Minute)

	// Before any route: answers CORS preflight requests from storefronts.
	r.Use(middleware.CORSMiddleware(domains))

	// Entry Route
	r.GET("/", func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, gin.H{"name": "Welcome to Huddle E-commerce API"})
	})

	// Registered Route Groups
	AdminRouter(r, db, rdb)
	StoreRouter(r, db, rdb, domains)
	ProductRouter(r, db, rdb)
	CustomerRoutes(r, db, rdb)
	CartRoutes(r, db, rdb, domains)
	StorefrontRoutes(r, db, rdb, domains)
	WebhookRoutes(r, db)

	// Serves API documentation
	doc.SwaggerDocs(r)

	return r
}