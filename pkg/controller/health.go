package controller

import (
	"context"
	"huddle/pkg/response"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// healthCheckTimeout bounds the whole check, so a monitor gets an answer even
// when a dependency hangs.
const healthCheckTimeout = 2 * time.Second

// HealthController reports whether the API can reach Postgres and Redis, for
// load balancers and uptime monitors.
type HealthController struct {
	Db    *gorm.DB
	Redis *redis.Client
}

func (h *HealthController) Check(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), healthCheckTimeout)
	defer cancel()

	checks := map[string]string{"postgres": "ok", "redis": "ok"}
	healthy := true
	// Error details are logged, never shown: the route is public.
	if err := pingPostgres(ctx, h.Db); err != nil {
		log.Printf("healthz: postgres: %v", err)
		checks["postgres"] = "down"
		healthy = false
	}
	if err := h.Redis.Ping(ctx).Err(); err != nil {
		log.Printf("healthz: redis: %v", err)
		checks["redis"] = "down"
		healthy = false
	}

	c.Header("Cache-Control", "no-store")
	if !healthy {
		c.JSON(http.StatusServiceUnavailable, response.ResponseEnvelope{Success: false, Message: "Unhealthy", Data: checks})
		return
	}
	response.Success(c, "Healthy", checks)
}

func pingPostgres(ctx context.Context, db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	return sqlDB.PingContext(ctx)
}
