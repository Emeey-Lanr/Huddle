package middleware

import (
	"errors"
	"huddle/pkg/model"
	"huddle/pkg/response"
	"huddle/pkg/utility"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// StoreKeyMiddleware identifies the store from the X-Store-Key header (the
// store's API key) for public storefront routes. The key is publishable: it
// is visible in storefront pages, so it must only ever unlock public actions.
// It sets "X-Store-Id" (public store id) and "storeInternalId".
func StoreKeyMiddleware(db *gorm.DB) gin.HandlerFunc {
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

		c.Set("X-Store-Id", store.StoreId)
		c.Set("storeInternalId", store.ID)
		c.Next()
	}
}
