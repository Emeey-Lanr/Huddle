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

type StoreBase struct {
	Db  *gorm.DB
	Rdb *redis.Client
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
