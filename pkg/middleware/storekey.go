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

// StoreKeyMiddleware identifies the store for public storefront routes, from
// either header:
//   - X-Store-Key, the store's publishable key, for sites developers build.
//   - X-Store-Host, the website's hostname (<slug>.<STOREFRONT_BASE_DOMAIN> or
//     a custom domain), for the shared template, which can't read keys back
//     because only their hashes are saved.
//
// Both are public values, so they must only ever unlock public actions.
// It sets "X-Store-Id" (public store id) and "storeInternalId".
func StoreKeyMiddleware(db *gorm.DB, domains *storedomain.Cache) gin.HandlerFunc {
	return func(c *gin.Context) {
		store, ok := identifyStore(c, db, domains)
		if !ok {
			c.Abort()
			return
		}

		// A page on one store's domain must not act for another store with
		// shoppers' cookies. Origins in CORS_ALLOWED_ORIGINS and same-origin
		// pages (such as the API's Swagger UI) may use any store; requests
		// without an Origin (curl, servers) are not browsers and are unaffected.
		origin := c.GetHeader("Origin")
		if origin != "" && !sameOrigin(origin, c.Request.Host) && !storedomain.OriginAllowed(origin, os.Getenv("CORS_ALLOWED_ORIGINS")) {
			owner := uuid.Nil
			if host, ok := storedomain.HTTPSHost(origin); ok {
				var err error
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

// identifyStore writes the error response itself when it returns false.
func identifyStore(c *gin.Context, db *gorm.DB, domains *storedomain.Cache) (*model.Stores, bool) {
	query := db.Select("id", "store_id")
	notFoundStatus, notFoundMessage := http.StatusNotFound, "Store not found"

	if key := c.GetHeader("X-Store-Key"); key != "" {
		query = query.Where("api_key = ?", utility.HashAPIKey(key))
		notFoundStatus, notFoundMessage = http.StatusUnauthorized, "Invalid store key"
	} else if header := c.GetHeader("X-Store-Host"); header != "" {
		host := storedomain.CleanHost(header)
		if slug, ok := storedomain.SlugFromHost(host, os.Getenv("STOREFRONT_BASE_DOMAIN")); ok {
			query = query.Where("slug = ?", slug)
		} else {
			storeId, err := domains.StoreFor(host)
			if err != nil {
				response.Error(c, http.StatusInternalServerError, "Internal Server Error", err)
				return nil, false
			}
			if storeId == uuid.Nil {
				response.Error(c, notFoundStatus, notFoundMessage, errors.New("no store at this host"))
				return nil, false
			}
			query = query.Where("id = ?", storeId)
		}
	} else {
		response.Error(c, http.StatusUnauthorized, "X-Store-Key header is missing", errors.New("missing store key"))
		return nil, false
	}

	var store model.Stores
	err := query.First(&store).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		response.Error(c, notFoundStatus, notFoundMessage, errors.New("no matching store"))
		return nil, false
	}
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Internal Server Error", err)
		return nil, false
	}
	return &store, true
}

// sameOrigin reports whether an http(s) origin points at the API itself.
func sameOrigin(origin, requestHost string) bool {
	return origin == "https://"+requestHost || origin == "http://"+requestHost
}
