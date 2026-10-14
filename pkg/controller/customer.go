package controller

import (
	"huddle/pkg/model"
	"huddle/pkg/response"
	customerservice "huddle/pkg/services/customer"
	"huddle/pkg/utility"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type CustomerBase struct {
	DB    *gorm.DB
	Redis *redis.Client
}

func (b *CustomerBase) Signup(c *gin.Context) {
	var req model.CustomerSignupReq

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

	token, customerResp, serviceErr := customerservice.Signup(b.DB, b.Redis, req)
	if serviceErr != nil {
		response.Error(c, serviceErr.StatusCode, serviceErr.Message, serviceErr.Err)
		return
	}

	response.Created(c, "Customer registered successfully", gin.H{
		"token":    token,
		"customer": customerResp,
	})
}

func (b *CustomerBase) Signin(c *gin.Context) {
	var req model.CustomerSigninReq

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

	token, customerResp, serviceErr := customerservice.Signin(b.DB, b.Redis, req)
	if serviceErr != nil {
		response.Error(c, serviceErr.StatusCode, serviceErr.Message, serviceErr.Err)
		return
	}

	response.Success(c, "Login successful", gin.H{
		"token":    token,
		"customer": customerResp,
	})
}

func (b *CustomerBase) GetProfile(c *gin.Context) {
	customerId, exists := c.Get("customerId")
	if !exists {
		response.Error(c, http.StatusUnauthorized, "Unauthorized access", nil)
		return
	}

	customerResp, serviceErr := customerservice.GetProfile(b.DB, customerId.(string))
	if serviceErr != nil {
		response.Error(c, serviceErr.StatusCode, serviceErr.Message, serviceErr.Err)
		return
	}

	response.Success(c, "Profile fetched successfully", customerResp)
}