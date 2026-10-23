package router

import (
	"huddle/doc"
	"huddle/pkg/config"
	"huddle/pkg/controller"
	"huddle/pkg/middleware"
	"huddle/pkg/storedomain"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

func RoutesSetup(db *gorm.DB, rdb *redis.Client) *gin.Engine {
	r := gin.New()
	// Only these proxies may report the client's IP (X-Forwarded-For); by
	// default Gin believes anyone, which would let clients dodge the rate limiter.
	if err := r.SetTrustedProxies(config.TrustedProxies(os.Getenv)); err != nil {
		log.Printf("TRUSTED_PROXIES ignored: %v", err)
		r.SetTrustedProxies(nil)
	}
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

	// For load balancers and uptime monitors: 200 when Postgres and Redis answer, 503 otherwise.
	health := &controller.HealthController{Db: db, Redis: rdb}
	r.GET("/healthz", health.Check)

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