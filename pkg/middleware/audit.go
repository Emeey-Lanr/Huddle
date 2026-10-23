package middleware

import (
	"huddle/pkg/audit"
	"time"

	"github.com/gin-gonic/gin"
)

func AuditLogMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		rawQuery := c.Request.URL.RawQuery

		c.Next()

		latency := time.Since(start)
		statusCode := c.Writer.Status()
		// Monitors call the health check every few seconds; only failures are worth a log line.
		if path == "/healthz" && statusCode < 400 {
			return
		}
		clientIP := c.ClientIP()
		method := c.Request.Method
		userAgent := c.Request.UserAgent()

		actorID, exists := c.Get("adminId")
		actorStr := "anonymous"
		if exists && actorID != nil {
			actorStr = actorID.(string)
		}

		if rawQuery != "" {
			path = path + "?" + rawQuery
		}

		statusStr := "SUCCESS"
		if statusCode >= 400 {
			statusStr = "FAILED"
		}

		audit.LogEvent(c.Request.Context(), "HTTP_REQUEST", actorStr, path, statusStr, map[string]interface{}{
			"method":      method,
			"path":        path,
			"status_code": statusCode,
			"latency_ms":  latency.Milliseconds(),
			"client_ip":   clientIP,
			"user_agent":  userAgent,
		})
	}
}
