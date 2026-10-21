package middleware

import (
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	corsAllowMethods = "GET, POST, PUT, DELETE, OPTIONS"
	corsAllowHeaders = "Content-Type, Authorization, X-Store-Key, X-Store-Id"
)

// CORSMiddleware lets browsers on allowed storefront origins call the API with
// credentials (the cart cookie). CORS_ALLOWED_ORIGINS is a comma-separated
// list of exact origins ("https://chiomasbags.com") or subdomain wildcards
// ("https://*.huddle.ng"). Requests from other origins get no CORS headers,
// and their preflight requests are rejected.
func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin == "" {
			c.Next()
			return
		}

		allowed := originAllowed(origin, os.Getenv("CORS_ALLOWED_ORIGINS"))
		c.Writer.Header().Add("Vary", "Origin")
		if allowed {
			h := c.Writer.Header()
			h.Set("Access-Control-Allow-Origin", origin)
			h.Set("Access-Control-Allow-Credentials", "true")
			h.Set("Access-Control-Allow-Methods", corsAllowMethods)
			h.Set("Access-Control-Allow-Headers", corsAllowHeaders)
			h.Set("Access-Control-Max-Age", "600")
		}

		if c.Request.Method == http.MethodOptions {
			if allowed {
				c.AbortWithStatus(http.StatusNoContent)
			} else {
				c.AbortWithStatus(http.StatusForbidden)
			}
			return
		}
		c.Next()
	}
}

func originAllowed(origin, allowList string) bool {
	for _, entry := range strings.Split(allowList, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if entry == origin {
			return true
		}
		// "https://*.huddle.ng" matches "https://shop.huddle.ng" but not "https://huddle.ng"
		// or "https://evilhuddle.ng".
		if scheme, host, ok := strings.Cut(entry, "://*."); ok {
			prefix := scheme + "://"
			if strings.HasPrefix(origin, prefix) {
				sub := strings.TrimPrefix(origin, prefix)
				if strings.HasSuffix(sub, "."+host) && len(sub) > len(host)+1 && !strings.ContainsAny(sub, "/@") {
					return true
				}
			}
		}
	}
	return false
}
