package middleware

import (
	"errors"
	"huddle/pkg/model"
	"huddle/pkg/response"
	"huddle/pkg/storedomain"
	"huddle/pkg/utility"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// StoreKeyMiddleware identifies the store from the X-Store-Key header (the
// store's API key) for public storefront routes. The key is publishable: it
// is visible in storefront pages, so it must only ever unlock public actions.
// It sets "X-Store-Id" (public store id) and "storeInternalId".
func StoreKeyMiddleware(db *gorm.DB, domains *storedomain.Cache) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.GetHeader("X-Store-Key")
		if key == "" {
			response.Error(c, http.StatusUnauthorized, "X-Store-Key header is missing", errors.New("missing store key"))
			c.Abort()
			return
		}

		var store model.Stores
		err := db.Select("id", "store_id").Where("api_key = ?", utility.HashAPIKey(key)).First(&store).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.Error(c, http.StatusUnauthorized, "Invalid store key", errors.New("unknown store key"))
			c.Abort()
			return
		}
		if err != nil {
			response.Error(c, http.StatusInternalServerError, "Internal Server Error", err)
			c.Abort()
			return
		}

		// A page on one store's domain must not use another store's key with
		// shoppers' cookies. Origins in CORS_ALLOWED_ORIGINS and same-origin
		// pages (such as the API's Swagger UI) may use any key; requests
		// without an Origin (curl, servers) are not browsers and are unaffected.
		origin := c.GetHeader("Origin")
		if origin != "" && !sameOrigin(origin, c.Request.Host) && !storedomain.OriginAllowed(origin, os.Getenv("CORS_ALLOWED_ORIGINS")) {
			owner := uuid.Nil
			if host, ok := storedomain.HTTPSHost(origin); ok {
				if owner, err = domains.StoreFor(host); err != nil {
					response.Error(c, http.StatusInternalServerError, "Internal Server Error", err)
					c.Abort()
					return
				}
			}
			if owner != store.ID {
				response.Error(c, http.StatusForbidden, "This store key can't be used from this website. Add the website's domain to the store first.", errors.New("origin is not one of the store's domains"))
				c.Abort()
				return
			}
		}

		c.Set("X-Store-Id", store.StoreId)
		c.Set("storeInternalId", store.ID)
		c.Next()
	}
}

// sameOrigin reports whether an http(s) origin points at the API itself.
func sameOrigin(origin, requestHost string) bool {
	return origin == "https://"+requestHost || origin == "http://"+requestHost
}
