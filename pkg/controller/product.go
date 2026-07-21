package controller

import (
	"errors"
	"huddle/pkg/audit"
	"huddle/pkg/model"
	"huddle/pkg/response"
	"huddle/pkg/services"
	"huddle/pkg/utility"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type ProductBase struct {
	Db  *gorm.DB
	Rdb *redis.Client
}

func (b *ProductBase) Add(c *gin.Context) {
	adminId, adminExists := c.Get("adminId")
	storePublicId, storeIdExists := c.Get("X-Store-Id")

	if !adminExists || !storeIdExists {
		response.Error(c, http.StatusUnauthorized, "Missing admin or store context", errors.New("unauthorized context"))
		return
	}

	var req model.Products
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid request payload", err)
		return
	}

	if err := utility.Validate.Struct(&req); err != nil {
		if validationErr := utility.ValidationErrors(err); validationErr != nil {
			response.Error(c, http.StatusBadRequest, "Validation failed", validationErr)
			return
		}
	}

	product, serviceErr := services.AddProduct(req, b.Db, b.Rdb, adminId.(string), storePublicId.(string))
	if serviceErr != nil {
		audit.LogEvent(c.Request.Context(), "PRODUCT_CREATED", adminId.(string), req.Name, "FAILED", map[string]interface{}{"error": serviceErr.Message})
		response.Error(c, serviceErr.StatusCode, serviceErr.Message, serviceErr.Err)
		return
	}

	audit.LogEvent(c.Request.Context(), "PRODUCT_CREATED", adminId.(string), product.PublicId, "SUCCESS", map[string]interface{}{"name": product.Name})
	response.Created(c, "Product Added Successfully", gin.H{"id": product.PublicId})
}

func (b *ProductBase) AddImages(c *gin.Context) {
	publicProductId := c.Param("id")
	adminId, _ := c.Get("adminId")
	actorStr := "anonymous"
	if adminId != nil {
		actorStr = adminId.(string)
	}

	form, err := c.MultipartForm()
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid multipart form data", err)
		return
	}

	files := form.File["image"]
	imageUploadResp, serviceErr := services.AddImages(b.Db, b.Rdb, files, publicProductId)
	if serviceErr != nil {
		audit.LogEvent(c.Request.Context(), "PRODUCT_IMAGES_UPLOADED", actorStr, publicProductId, "FAILED", map[string]interface{}{"error": serviceErr.Message})
		response.Error(c, serviceErr.StatusCode, serviceErr.Message, serviceErr.Err)
		return
	}

	audit.LogEvent(c.Request.Context(), "PRODUCT_IMAGES_UPLOADED", actorStr, publicProductId, "SUCCESS", map[string]interface{}{"count": len(files)})
	response.Created(c, "Images uploaded successfully", imageUploadResp)
}

func (b *ProductBase) GetAllProducts(c *gin.Context) {
	var query model.ProductQuery

	if err := c.ShouldBindQuery(&query); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid query parameters", err)
		return
	}

	products, serviceErr := services.GetAllProducts(b.Db, b.Rdb, query)
	if serviceErr != nil {
		response.Error(c, serviceErr.StatusCode, serviceErr.Message, serviceErr.Err)
		return
	}

	response.Success(c, "Products retrieved successfully", products)
}

func (b *ProductBase) GetAProduct(c *gin.Context) {
	publicId := c.Param("id")
	if publicId == "" {
		response.Error(c, http.StatusBadRequest, "Invalid Product ID", errors.New("product ID is required"))
		return
	}

	product, images, serviceErr := services.GetAProduct(b.Db, b.Rdb, publicId)
	if serviceErr != nil {
		response.Error(c, serviceErr.StatusCode, serviceErr.Message, serviceErr.Err)
		return
	}

	data := gin.H{"product": product, "images": images}
	response.Success(c, "Product Found", data)
}

func (b *ProductBase) DeleteProduct(c *gin.Context) {
	publicId := c.Param("id")
	adminId, _ := c.Get("adminId")
	actorStr := "anonymous"
	if adminId != nil {
		actorStr = adminId.(string)
	}

	if publicId == "" {
		response.Error(c, http.StatusBadRequest, "Product ID is required", errors.New("product ID is required"))
		return
	}

	if serviceErr := services.DeleteProduct(b.Db, b.Rdb, publicId); serviceErr != nil {
		audit.LogEvent(c.Request.Context(), "PRODUCT_DELETED", actorStr, publicId, "FAILED", map[string]interface{}{"error": serviceErr.Message})
		response.Error(c, serviceErr.StatusCode, serviceErr.Message, serviceErr.Err)
		return
	}

	audit.LogEvent(c.Request.Context(), "PRODUCT_DELETED", actorStr, publicId, "SUCCESS", nil)
	response.Success(c, "Product deleted successfully", nil)
}

func (b *ProductBase) UpdateProduct(c *gin.Context) {
	publicId := c.Param("id")
	adminId, _ := c.Get("adminId")
	actorStr := "anonymous"
	if adminId != nil {
		actorStr = adminId.(string)
	}

	if publicId == "" {
		response.Error(c, http.StatusBadRequest, "Product ID is required", errors.New("product ID is required"))
		return
	}

	var req model.ProductUpdate
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid request payload", err)
		return
	}

	product, serviceErr := services.UpdateProduct(b.Db, b.Rdb, publicId, req)
	if serviceErr != nil {
		audit.LogEvent(c.Request.Context(), "PRODUCT_UPDATED", actorStr, publicId, "FAILED", map[string]interface{}{"error": serviceErr.Message})
		response.Error(c, serviceErr.StatusCode, serviceErr.Message, serviceErr.Err)
		return
	}

	audit.LogEvent(c.Request.Context(), "PRODUCT_UPDATED", actorStr, publicId, "SUCCESS", nil)
	response.Success(c, "Product updated successfully", product)
}