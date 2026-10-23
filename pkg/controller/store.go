package controller

import (
	"errors"
	"huddle/pkg/audit"
	"huddle/pkg/model"
	"huddle/pkg/response"
	"huddle/pkg/services"
	"huddle/pkg/storedomain"
	"huddle/pkg/utility"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type StoreBase struct {
	Db      *gorm.DB
	Rdb     *redis.Client
	Domains *storedomain.Cache
}

func (b *StoreBase) Create(c *gin.Context) {
	adminId, adminExists := c.Get("adminId")
	email, emailExists := c.Get("email")

	if !adminExists || !emailExists {
		response.Error(c, http.StatusUnauthorized, "Unauthorized access", errors.New("admin context missing"))
		return
	}

	uuidAdminId, err := utility.StringToUUID(adminId.(string))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid admin ID format", err)
		return
	}

	var req model.Stores
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

	data, apiKey, serviceErr := services.CreateStore(b.Db, req, uuidAdminId, email.(string))
	if serviceErr != nil {
		audit.LogEvent(c.Request.Context(), "STORE_CREATED", adminId.(string), req.StoreName, "FAILED", map[string]interface{}{"error": serviceErr.Message})
		response.Error(c, serviceErr.StatusCode, serviceErr.Message, serviceErr.Err)
		return
	}

	audit.LogEvent(c.Request.Context(), "STORE_CREATED", adminId.(string), data.StoreId, "SUCCESS", map[string]interface{}{"store_name": data.StoreName})

	response.Created(c, "Store Created Successfully", gin.H{
		"store_name": data.StoreName,
		"store_id":   data.StoreId,
		"api_key":    apiKey,
		"created_at": data.CreatedAt,
	})
}

// adminUUID reads the admin id set by AuthMiddleware.
func adminUUID(c *gin.Context) (uuid.UUID, bool) {
	id, err := utility.StringToUUID(c.GetString("adminId"))
	if err != nil {
		response.Error(c, http.StatusUnauthorized, "Unauthorized access", err)
		return uuid.Nil, false
	}
	return id, true
}

func (b *StoreBase) List(c *gin.Context) {
	adminId, ok := adminUUID(c)
	if !ok {
		return
	}

	stores, serviceErr := services.ListStores(b.Db, adminId)
	if serviceErr != nil {
		response.Error(c, serviceErr.StatusCode, serviceErr.Message, serviceErr.Err)
		return
	}

	response.Success(c, "Stores retrieved successfully", stores)
}

func (b *StoreBase) Get(c *gin.Context) {
	adminId, ok := adminUUID(c)
	if !ok {
		return
	}

	store, serviceErr := services.GetStore(b.Db, c.Param("id"), adminId)
	if serviceErr != nil {
		response.Error(c, serviceErr.StatusCode, serviceErr.Message, serviceErr.Err)
		return
	}

	response.Success(c, "Store retrieved successfully", store)
}

func (b *StoreBase) Update(c *gin.Context) {
	adminId, ok := adminUUID(c)
	if !ok {
		return
	}
	storeId := c.Param("id")

	var req model.StoreUpdate
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid request payload", err)
		return
	}

	if err := utility.Validate.Struct(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "Validation failed", utility.ValidationErrors(err))
		return
	}

	store, serviceErr := services.UpdateStore(b.Db, storeId, adminId, req)
	if serviceErr != nil {
		audit.LogEvent(c.Request.Context(), "STORE_UPDATED", adminId.String(), storeId, "FAILED", map[string]interface{}{"error": serviceErr.Message})
		response.Error(c, serviceErr.StatusCode, serviceErr.Message, serviceErr.Err)
		return
	}

	audit.LogEvent(c.Request.Context(), "STORE_UPDATED", adminId.String(), storeId, "SUCCESS", nil)
	response.Success(c, "Store updated successfully", store)
}

func (b *StoreBase) RegenerateKey(c *gin.Context) {
	adminId, ok := adminUUID(c)
	if !ok {
		return
	}
	storeId := c.Param("id")

	apiKey, serviceErr := services.RegenerateStoreKey(b.Db, storeId, adminId)
	if serviceErr != nil {
		audit.LogEvent(c.Request.Context(), "STORE_KEY_REGENERATED", adminId.String(), storeId, "FAILED", map[string]interface{}{"error": serviceErr.Message})
		response.Error(c, serviceErr.StatusCode, serviceErr.Message, serviceErr.Err)
		return
	}

	// The key itself is never logged.
	audit.LogEvent(c.Request.Context(), "STORE_KEY_REGENERATED", adminId.String(), storeId, "SUCCESS", nil)
	response.Success(c, "Store key regenerated. The old key no longer works.", gin.H{
		"store_id": storeId,
		"api_key":  apiKey,
	})
}

func (b *StoreBase) ListDomains(c *gin.Context) {
	adminId, ok := adminUUID(c)
	if !ok {
		return
	}

	domains, serviceErr := services.ListStoreDomains(b.Db, c.Param("id"), adminId)
	if serviceErr != nil {
		response.Error(c, serviceErr.StatusCode, serviceErr.Message, serviceErr.Err)
		return
	}

	response.Success(c, "Store domains retrieved successfully", domains)
}

func (b *StoreBase) AddDomain(c *gin.Context) {
	adminId, ok := adminUUID(c)
	if !ok {
		return
	}
	storeId := c.Param("id")

	var req model.StoreDomainRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid request payload", err)
		return
	}
	if err := utility.Validate.Struct(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "Validation failed", utility.ValidationErrors(err))
		return
	}

	domain, created, serviceErr := services.AddStoreDomain(b.Db, b.Domains, storeId, adminId, req.Domain)
	if serviceErr != nil {
		audit.LogEvent(c.Request.Context(), "STORE_DOMAIN_ADDED", adminId.String(), storeId, "FAILED", map[string]interface{}{"domain": req.Domain, "error": serviceErr.Message})
		response.Error(c, serviceErr.StatusCode, serviceErr.Message, serviceErr.Err)
		return
	}

	if !created {
		response.Success(c, "Domain already added", domain)
		return
	}
	audit.LogEvent(c.Request.Context(), "STORE_DOMAIN_ADDED", adminId.String(), storeId, "SUCCESS", map[string]interface{}{"domain": domain.Domain})
	response.Created(c, "Domain added", domain)
}

func (b *StoreBase) RemoveDomain(c *gin.Context) {
	adminId, ok := adminUUID(c)
	if !ok {
		return
	}
	storeId := c.Param("id")

	if serviceErr := services.RemoveStoreDomain(b.Db, b.Domains, storeId, adminId, c.Param("domain")); serviceErr != nil {
		audit.LogEvent(c.Request.Context(), "STORE_DOMAIN_REMOVED", adminId.String(), storeId, "FAILED", map[string]interface{}{"domain": c.Param("domain"), "error": serviceErr.Message})
		response.Error(c, serviceErr.StatusCode, serviceErr.Message, serviceErr.Err)
		return
	}

	audit.LogEvent(c.Request.Context(), "STORE_DOMAIN_REMOVED", adminId.String(), storeId, "SUCCESS", map[string]interface{}{"domain": c.Param("domain")})
	response.Success(c, "Domain removed", nil)
}
