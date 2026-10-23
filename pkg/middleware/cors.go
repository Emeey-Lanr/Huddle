package middleware

import (
	"log"
	"net/http"
	"os"

	"huddle/pkg/storedomain"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	corsAllowMethods = "GET, POST, PUT, DELETE, OPTIONS"
	corsAllowHeaders = "Content-Type, Authorization, X-Store-Key, X-Store-Id"
)

// CORSMiddleware lets browsers on allowed storefront origins call the API with
// credentials (the cart cookie). Allowed origins are those in
// CORS_ALLOWED_ORIGINS, a comma-separated list of exact origins
// ("https://chiomasbags.com") or subdomain wildcards ("https://*.huddle.ng"),
// and https://<a store's custom domain>. Requests from other origins get no
// CORS headers, and their preflight requests are rejected.
func CORSMiddleware(domains *storedomain.Cache) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin == "" {
			c.Next()
			return
		}

		allowed := originAllowed(origin, domains)
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

// originAllowed: a preflight carries no store key, so any store's domain is
// accepted here; StoreKeyMiddleware then checks the domain matches the key.
func originAllowed(origin string, domains *storedomain.Cache) bool {
	if storedomain.OriginAllowed(origin, os.Getenv("CORS_ALLOWED_ORIGINS")) {
		return true
	}
	host, ok := storedomain.HTTPSHost(origin)
	if !ok {
		return false
	}
	storeId, err := domains.StoreFor(host)
	if err != nil {
		log.Printf("cors: looking up domain %s: %v", host, err)
		return false
	}
	return storeId != uuid.Nil
}
