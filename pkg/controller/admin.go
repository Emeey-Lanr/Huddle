package controller

import (
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

type AdminBase struct {
	Db  *gorm.DB
	Rdb *redis.Client
}

func (b *AdminBase) Signup(c *gin.Context) {
	var req model.Admins

	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid request payload", err)
		return
	}

	if err := utility.Validate.Struct(&req); err != nil {
		errMsg := utility.ValidationErrors(err)
		response.Error(c, http.StatusBadRequest, "Validation failed", errMsg)
		return
	}

	token, servicesErr := services.AdminSignup(b.Db, req)
	if servicesErr != nil {
		audit.LogEvent(c.Request.Context(), "ADMIN_SIGNUP", req.Username, req.Email, "FAILED", map[string]interface{}{"error": servicesErr.Message})
		response.Error(c, servicesErr.StatusCode, servicesErr.Message, servicesErr.Err)
		return
	}

	audit.LogEvent(c.Request.Context(), "ADMIN_SIGNUP", req.Username, req.Email, "SUCCESS", nil)
	response.Created(c, "Registration Successful", gin.H{"token": token})
}

func (b *AdminBase) Signin(c *gin.Context) {
	var req model.AdminSignin

	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid request payload", err)
		return
	}

	if err := utility.Validate.Struct(&req); err != nil {
		errMsg := utility.ValidationErrors(err)
		response.Error(c, http.StatusBadRequest, "Validation failed", errMsg)
		return
	}

	token, servicesErr := services.AdminSignin(b.Db, req)
	if servicesErr != nil {
		audit.LogEvent(c.Request.Context(), "ADMIN_SIGNIN", req.Username, "", "FAILED", map[string]interface{}{"error": servicesErr.Message})
		response.Error(c, servicesErr.StatusCode, servicesErr.Message, servicesErr.Err)
		return
	}

	audit.LogEvent(c.Request.Context(), "ADMIN_SIGNIN", req.Username, "", "SUCCESS", nil)
	response.Success(c, "Login Successful", gin.H{"token": token})
}
