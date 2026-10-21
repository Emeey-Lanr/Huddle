package controller

import (
	"huddle/pkg/model"
	"huddle/pkg/response"
	"huddle/pkg/services"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// StorefrontBase serves public storefront routes behind StoreKeyMiddleware.
type StorefrontBase struct {
	Db *gorm.DB
}

func storeFromKey(c *gin.Context) uuid.UUID {
	id, _ := c.Get("storeInternalId")
	return id.(uuid.UUID)
}

func (b *StorefrontBase) Store(c *gin.Context) {
	store, serviceErr := services.GetStorefrontStore(b.Db, storeFromKey(c))
	if serviceErr != nil {
		response.Error(c, serviceErr.StatusCode, serviceErr.Message, serviceErr.Err)
		return
	}
	response.Success(c, "Store retrieved successfully", store)
}

func (b *StorefrontBase) Products(c *gin.Context) {
	var query model.StorefrontProductQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid query parameters", err)
		return
	}

	products, serviceErr := services.ListStorefrontProducts(b.Db, storeFromKey(c), query)
	if serviceErr != nil {
		response.Error(c, serviceErr.StatusCode, serviceErr.Message, serviceErr.Err)
		return
	}
	response.Success(c, "Products retrieved successfully", products)
}

func (b *StorefrontBase) Product(c *gin.Context) {
	product, images, serviceErr := services.GetStorefrontProduct(b.Db, storeFromKey(c), c.Param("id"))
	if serviceErr != nil {
		response.Error(c, serviceErr.StatusCode, serviceErr.Message, serviceErr.Err)
		return
	}
	response.Success(c, "Product Found", gin.H{"product": product, "images": images})
}
